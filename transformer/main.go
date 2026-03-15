package main

import (
	"fmt"
	"os"
)

func main() {
	if len(os.Args) < 2 {
		fmt.Fprintln(os.Stderr, "Usage: transformer <command> [args]")
		fmt.Fprintln(os.Stderr, "Commands: lint, index, render, minify, build")
		os.Exit(1)
	}

	cmd := os.Args[1]
	args := os.Args[2:]

	switch cmd {
	case "lint":
		if len(args) != 1 {
			fmt.Fprintln(os.Stderr, "Usage: transformer lint <content-dir>")
			os.Exit(1)
		}
		fmt.Printf("lint: content-dir=%s\n", args[0])

	case "index":
		if len(args) != 1 {
			fmt.Fprintln(os.Stderr, "Usage: transformer index <content-dir>")
			os.Exit(1)
		}
		fmt.Printf("index: content-dir=%s\n", args[0])

	case "render":
		if len(args) != 3 {
			fmt.Fprintln(os.Stderr, "Usage: transformer render <content-dir> <dist-dir> <site-url>")
			os.Exit(1)
		}
		fmt.Printf("render: content-dir=%s dist-dir=%s site-url=%s\n", args[0], args[1], args[2])

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
}
