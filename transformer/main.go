package main

import (
	"fmt"
	"os"

	autoindex "github.com/jeonghyeon-net/jeonghyeon.net/transformer/auto-index"
	contentlinter "github.com/jeonghyeon-net/jeonghyeon.net/transformer/content-linter"
	mdtohtml "github.com/jeonghyeon-net/jeonghyeon.net/transformer/md-to-html"
)

func main() {
	if len(os.Args) < 2 {
		fmt.Fprintln(os.Stderr, "Usage: transformer <command> [args]")
		fmt.Fprintln(os.Stderr, "Commands: lint, index, render, minify, build")
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
		fmt.Printf("minify: dist-dir=%s\n", args[0])

	case "build":
		if len(args) != 3 {
			fmt.Fprintln(os.Stderr, "Usage: transformer build <content-dir> <dist-dir> <site-url>")
			os.Exit(1)
		}
		fmt.Printf("build: content-dir=%s dist-dir=%s site-url=%s\n", args[0], args[1], args[2])

	default:
		fmt.Fprintf(os.Stderr, "Unknown command: %s\n", cmd)
		fmt.Fprintln(os.Stderr, "Commands: lint, index, render, minify, build")
		os.Exit(1)
	}

	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
