package main

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync/atomic"
	"time"

	"github.com/fsnotify/fsnotify"

	autoindex "github.com/jeonghyeon-net/jeonghyeon.net/.github/site-builder/auto-index"
	contentlinter "github.com/jeonghyeon-net/jeonghyeon.net/.github/site-builder/content-linter"
	htmlminifier "github.com/jeonghyeon-net/jeonghyeon.net/.github/site-builder/html-minifier"
	"github.com/jeonghyeon-net/jeonghyeon.net/.github/site-builder/internal/sitepath"
	mdtohtml "github.com/jeonghyeon-net/jeonghyeon.net/.github/site-builder/md-to-html"
)

func main() {
	if len(os.Args) < 2 {
		fmt.Fprintln(os.Stderr, "Usage: site-builder <command> [args]")
		fmt.Fprintln(os.Stderr, "Commands: lint, index, render, render-single, minify, build, check, watch")
		os.Exit(1)
	}

	cmd := os.Args[1]
	args := os.Args[2:]

	var err error

	switch cmd {
	case "lint":
		fix := false
		sourceDir := ""
		if len(args) == 1 {
			sourceDir = args[0]
		} else if len(args) == 2 && args[0] == "--fix" {
			fix = true
			sourceDir = args[1]
		} else {
			fmt.Fprintln(os.Stderr, "Usage: site-builder lint [--fix] <source-dir>")
			os.Exit(1)
		}
		if fix {
			if err = contentlinter.FixDir(sourceDir); err != nil {
				break
			}
		}
		errs := contentlinter.LintDir(sourceDir)
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
			fmt.Fprintln(os.Stderr, "Usage: site-builder index <source-dir>")
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
			fmt.Fprintln(os.Stderr, "Usage: site-builder render <source-dir> <dist-dir>")
			os.Exit(1)
		}
		err = mdtohtml.Render(args[0], args[1])
		if err == nil {
			fmt.Println("render complete:", args[1])
		}

	case "minify":
		if len(args) != 1 {
			fmt.Fprintln(os.Stderr, "Usage: site-builder minify <dist-dir>")
			os.Exit(1)
		}
		err = htmlminifier.MinifyDir(args[0])
		if err == nil {
			fmt.Println("minify complete:", args[0])
		}

	case "build":
		if len(args) != 2 {
			fmt.Fprintln(os.Stderr, "Usage: site-builder build <source-dir> <dist-dir>")
			os.Exit(1)
		}
		sourceDir, distDir := args[0], args[1]
		// lint
		lintErrs := contentlinter.LintDir(sourceDir)
		if len(lintErrs) > 0 {
			for _, e := range lintErrs {
				fmt.Fprintln(os.Stderr, e)
			}
			fmt.Fprintf(os.Stderr, "\n%d error(s) found\n", len(lintErrs))
			os.Exit(1)
		}
		// index
		generated, genErr := autoindex.Generate(sourceDir)
		if genErr != nil {
			err = genErr
			break
		}
		if _, writeErr := autoindex.WriteGenerated(sourceDir, generated); writeErr != nil {
			err = writeErr
			break
		}
		// render
		if err = mdtohtml.Render(sourceDir, distDir); err != nil {
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
			fmt.Fprintln(os.Stderr, "Usage: site-builder check <dist-dir>")
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

	case "render-single":
		if len(args) != 2 {
			fmt.Fprintln(os.Stderr, "Usage: site-builder render-single <source-dir> <md-path>")
			os.Exit(1)
		}
		err = mdtohtml.RenderSingle(args[0], args[1], os.Stdout)

	case "watch":
		if len(args) != 2 {
			fmt.Fprintln(os.Stderr, "Usage: site-builder watch <source-dir> <dist-dir>")
			os.Exit(1)
		}
		err = watch(args[0], args[1])

	default:
		fmt.Fprintf(os.Stderr, "Unknown command: %s\n", cmd)
		fmt.Fprintln(os.Stderr, "Commands: lint, index, render, render-single, minify, build, check, watch")
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

func rebuild(sourceDir, distDir string) {
	fmt.Println("rebuilding...")
	// index
	generated, err := autoindex.Generate(sourceDir)
	if err != nil {
		fmt.Fprintln(os.Stderr, "index error:", err)
		return
	}
	if _, err := autoindex.WriteGenerated(sourceDir, generated); err != nil {
		fmt.Fprintln(os.Stderr, "index write error:", err)
		return
	}
	// render
	if err := mdtohtml.Render(sourceDir, distDir); err != nil {
		fmt.Fprintln(os.Stderr, "render error:", err)
		return
	}
	// minify
	if err := htmlminifier.MinifyDir(distDir); err != nil {
		fmt.Fprintln(os.Stderr, "minify error:", err)
		return
	}
	// check
	if violations := checkOutput(distDir); len(violations) > 0 {
		for _, v := range violations {
			fmt.Fprintln(os.Stderr, v)
		}
		fmt.Fprintf(os.Stderr, "%d violation(s) found\n", len(violations))
		return
	}
	fmt.Println("done.")
}

func watch(sourceDir, distDir string) error {
	// Initial build
	rebuild(sourceDir, distDir)

	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		return err
	}
	defer watcher.Close()

	// Watch all directories under sourceDir recursively
	filepath.Walk(sourceDir, func(path string, info os.FileInfo, err error) error {
		if err != nil || !info.IsDir() {
			return nil
		}
		if sitepath.IsExcluded(sourceDir, path) {
			return filepath.SkipDir
		}
		return watcher.Add(path)
	})

	fmt.Println("watching", sourceDir, "for changes...")

	// Debounce: wait for quiet period before rebuilding
	var timer *time.Timer
	var rebuilding atomic.Bool

	for {
		select {
		case event, ok := <-watcher.Events:
			if !ok {
				return nil
			}
			if sitepath.IsExcluded(sourceDir, event.Name) {
				continue
			}
			// Watch new directories
			if event.Has(fsnotify.Create) {
				if info, statErr := os.Stat(event.Name); statErr == nil && info.IsDir() {
					watcher.Add(event.Name)
				}
			}

			// Ignore events during rebuild (auto-index writes index.md back into the source tree)
			if rebuilding.Load() {
				continue
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
				rebuilding.Store(true)
				rebuild(sourceDir, distDir)
				rebuilding.Store(false)
			})

		case watchErr, ok := <-watcher.Errors:
			if !ok {
				return nil
			}
			fmt.Fprintln(os.Stderr, "watch error:", watchErr)
		}
	}
}
