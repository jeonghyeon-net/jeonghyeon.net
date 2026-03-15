package main

import (
	"fmt"
	"os"
	"path/filepath"
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
		fmt.Fprintln(os.Stderr, "Commands: lint, index, render, minify, build, watch")
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
		if len(args) != 3 {
			fmt.Fprintln(os.Stderr, "Usage: transformer render <content-dir> <dist-dir> <site-url>")
			os.Exit(1)
		}
		err = mdtohtml.Render(args[0], args[1], args[2])
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
		if len(args) != 3 {
			fmt.Fprintln(os.Stderr, "Usage: transformer build <content-dir> <dist-dir> <site-url>")
			os.Exit(1)
		}
		contentDir, distDir, siteURL := args[0], args[1], args[2]
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
		if err = mdtohtml.Render(contentDir, distDir, siteURL); err != nil {
			break
		}
		// minify
		if err = htmlminifier.MinifyDir(distDir); err != nil {
			break
		}
		fmt.Println("build complete:", distDir)

	case "watch":
		if len(args) != 3 {
			fmt.Fprintln(os.Stderr, "Usage: transformer watch <content-dir> <dist-dir> <site-url>")
			os.Exit(1)
		}
		err = watch(args[0], args[1], args[2])

	default:
		fmt.Fprintf(os.Stderr, "Unknown command: %s\n", cmd)
		fmt.Fprintln(os.Stderr, "Commands: lint, index, render, minify, build, watch")
		os.Exit(1)
	}

	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func rebuild(contentDir, distDir, siteURL string) {
	fmt.Println("rebuilding...")
	if err := mdtohtml.Render(contentDir, distDir, siteURL); err != nil {
		fmt.Fprintln(os.Stderr, "render error:", err)
		return
	}
	if err := htmlminifier.MinifyDir(distDir); err != nil {
		fmt.Fprintln(os.Stderr, "minify error:", err)
		return
	}
	fmt.Println("done.")
}

func watch(contentDir, distDir, siteURL string) error {
	// Initial build
	rebuild(contentDir, distDir, siteURL)

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
			ext := strings.ToLower(filepath.Ext(event.Name))
			if ext != ".md" && ext != ".txt" && ext != ".webp" {
				continue
			}
			// Debounce 300ms
			if timer != nil {
				timer.Stop()
			}
			timer = time.AfterFunc(300*time.Millisecond, func() {
				rebuild(contentDir, distDir, siteURL)
			})

			// Watch new directories
			if event.Has(fsnotify.Create) {
				if info, statErr := os.Stat(event.Name); statErr == nil && info.IsDir() {
					watcher.Add(event.Name)
				}
			}

		case watchErr, ok := <-watcher.Errors:
			if !ok {
				return nil
			}
			fmt.Fprintln(os.Stderr, "watch error:", watchErr)
		}
	}
}
