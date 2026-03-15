package main

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/fsnotify/fsnotify"

	autoindex "github.com/jeonghyeon-net/jeonghyeon.net/transformer/auto-index"
	contentlinter "github.com/jeonghyeon-net/jeonghyeon.net/transformer/content-linter"
	htmlminifier "github.com/jeonghyeon-net/jeonghyeon.net/transformer/html-minifier"
	mdtohtml "github.com/jeonghyeon-net/jeonghyeon.net/transformer/md-to-html"
)

func main() {
	if len(os.Args) < 2 {
		fmt.Fprintln(os.Stderr, "Usage: transformer <command> [args]")
		fmt.Fprintln(os.Stderr, "Commands: lint, index, render, minify, build, check, watch")
		os.Exit(1)
	}

	cmd := os.Args[1]
	args := os.Args[2:]

	var err error

	switch cmd {
	case "lint":
		if len(args) != 1 {
			fmt.Fprintln(os.Stderr, "Usage: transformer lint <content-dir>")
			os.Exit(1)
		}
		errs := contentlinter.LintDir(args[0])
		if len(errs) > 0 {
			for _, e := range errs {
				fmt.Fprintln(os.Stderr, e)
			}
			fmt.Fprintf(os.Stderr, "\n%d error(s) found\n", len(errs))
			os.Exit(1)
		}
		fmt.Println("lint passed")

	case "index":
		if len(args) != 1 {
			fmt.Fprintln(os.Stderr, "Usage: transformer index <content-dir>")
			os.Exit(1)
		}
		generated, genErr := autoindex.Generate(args[0])
		if genErr != nil {
			err = genErr
			break
		}
		created, writeErr := autoindex.WriteGenerated(args[0], generated)
		if writeErr != nil {
			err = writeErr
			break
		}
		for _, p := range created {
			fmt.Println(p)
		}

	case "render":
		if len(args) != 2 {
			fmt.Fprintln(os.Stderr, "Usage: transformer render <content-dir> <dist-dir>")
			os.Exit(1)
		}
		err = mdtohtml.Render(args[0], args[1])
		if err == nil {
			fmt.Println("render complete:", args[1])
		}

	case "minify":
		if len(args) != 1 {
			fmt.Fprintln(os.Stderr, "Usage: transformer minify <dist-dir>")
			os.Exit(1)
		}
		err = htmlminifier.MinifyDir(args[0])
		if err == nil {
			fmt.Println("minify complete:", args[0])
		}

	case "build":
		if len(args) != 2 {
			fmt.Fprintln(os.Stderr, "Usage: transformer build <content-dir> <dist-dir>")
			os.Exit(1)
		}
		contentDir, distDir := args[0], args[1]
		// lint
		lintErrs := contentlinter.LintDir(contentDir)
		if len(lintErrs) > 0 {
			for _, e := range lintErrs {
				fmt.Fprintln(os.Stderr, e)
			}
			fmt.Fprintf(os.Stderr, "\n%d error(s) found\n", len(lintErrs))
			os.Exit(1)
		}
		// index
		generated, genErr := autoindex.Generate(contentDir)
		if genErr != nil {
			err = genErr
			break
		}
		if _, writeErr := autoindex.WriteGenerated(contentDir, generated); writeErr != nil {
			err = writeErr
			break
		}
		// render
		if err = mdtohtml.Render(contentDir, distDir); err != nil {
			break
		}
		// minify
		if err = htmlminifier.MinifyDir(distDir); err != nil {
			break
		}
		// check output
		checkErrs := checkOutput(distDir)
		if len(checkErrs) > 0 {
			for _, e := range checkErrs {
				fmt.Fprintln(os.Stderr, e)
			}
			fmt.Fprintf(os.Stderr, "\n%d violation(s) found in output\n", len(checkErrs))
			os.Exit(1)
		}
		fmt.Println("build complete:", distDir)

	case "check":
		if len(args) != 1 {
			fmt.Fprintln(os.Stderr, "Usage: transformer check <dist-dir>")
			os.Exit(1)
		}
		checkErrs := checkOutput(args[0])
		if len(checkErrs) > 0 {
			for _, e := range checkErrs {
				fmt.Fprintln(os.Stderr, e)
			}
			fmt.Fprintf(os.Stderr, "\n%d violation(s) found\n", len(checkErrs))
			os.Exit(1)
		}
		fmt.Println("check passed")

	case "watch":
		if len(args) != 2 {
			fmt.Fprintln(os.Stderr, "Usage: transformer watch <content-dir> <dist-dir>")
			os.Exit(1)
		}
		err = watch(args[0], args[1])

	default:
		fmt.Fprintf(os.Stderr, "Unknown command: %s\n", cmd)
		fmt.Fprintln(os.Stderr, "Commands: lint, index, render, minify, build, check, watch")
		os.Exit(1)
	}

	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

// checkOutput scans dist HTML files for any CSS or JS contamination.
func checkOutput(distDir string) []string {
	var violations []string

	forbidden := []string{
		"<style", "</style>",
		"<script", "</script>",
		"style=",
		"javascript:",
	}

	// Matches on* event handler attributes (onclick=, onload=, etc.)
	onEventRe := regexp.MustCompile(`\bon[a-z]+=`)
	// Matches <link rel="stylesheet" or <link rel='stylesheet'
	linkStyleRe := regexp.MustCompile(`<link[^>]*rel\s*=\s*["']stylesheet["']`)

	filepath.Walk(distDir, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() || !strings.HasSuffix(path, ".html") {
			return nil
		}

		data, readErr := os.ReadFile(path)
		if readErr != nil {
			return nil
		}
		content := strings.ToLower(string(data))
		relPath, _ := filepath.Rel(distDir, path)

		for _, pattern := range forbidden {
			if strings.Contains(content, pattern) {
				violations = append(violations, fmt.Sprintf("%s: contains '%s'", relPath, pattern))
			}
		}

		if matches := onEventRe.FindAllString(content, -1); len(matches) > 0 {
			for _, m := range matches {
				violations = append(violations, fmt.Sprintf("%s: contains event handler '%s'", relPath, m))
			}
		}

		if linkStyleRe.MatchString(content) {
			violations = append(violations, fmt.Sprintf("%s: contains <link rel=\"stylesheet\">", relPath))
		}

		return nil
	})

	return violations
}

func rebuild(contentDir, distDir string) {
	fmt.Println("rebuilding...")
	// index
	generated, err := autoindex.Generate(contentDir)
	if err != nil {
		fmt.Fprintln(os.Stderr, "index error:", err)
		return
	}
	if _, err := autoindex.WriteGenerated(contentDir, generated); err != nil {
		fmt.Fprintln(os.Stderr, "index write error:", err)
		return
	}
	// render
	if err := mdtohtml.Render(contentDir, distDir); err != nil {
		fmt.Fprintln(os.Stderr, "render error:", err)
		return
	}
	// minify
	if err := htmlminifier.MinifyDir(distDir); err != nil {
		fmt.Fprintln(os.Stderr, "minify error:", err)
		return
	}
	fmt.Println("done.")
}

func watch(contentDir, distDir string) error {
	// Initial build
	rebuild(contentDir, distDir)

	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		return err
	}
	defer watcher.Close()

	// Watch all directories under contentDir recursively
	filepath.Walk(contentDir, func(path string, info os.FileInfo, err error) error {
		if err != nil || !info.IsDir() {
			return nil
		}
		return watcher.Add(path)
	})

	fmt.Println("watching", contentDir, "for changes...")

	// Debounce: wait for quiet period before rebuilding
	var timer *time.Timer

	for {
		select {
		case event, ok := <-watcher.Events:
			if !ok {
				return nil
			}
			// Watch new directories
			if event.Has(fsnotify.Create) {
				if info, statErr := os.Stat(event.Name); statErr == nil && info.IsDir() {
					watcher.Add(event.Name)
				}
			}

			// Trigger rebuild on delete or relevant file changes
			shouldRebuild := event.Has(fsnotify.Remove) || event.Has(fsnotify.Rename)
			if !shouldRebuild {
				ext := strings.ToLower(filepath.Ext(event.Name))
				shouldRebuild = ext == ".md" || ext == ".txt" || ext == ".webp" || ext == ".yaml"
			}
			if !shouldRebuild {
				continue
			}

			// Debounce 300ms
			if timer != nil {
				timer.Stop()
			}
			timer = time.AfterFunc(300*time.Millisecond, func() {
				rebuild(contentDir, distDir)
			})

		case watchErr, ok := <-watcher.Errors:
			if !ok {
				return nil
			}
			fmt.Fprintln(os.Stderr, "watch error:", watchErr)
		}
	}
}
