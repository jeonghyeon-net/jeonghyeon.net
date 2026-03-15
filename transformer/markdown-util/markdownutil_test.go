package markdownutil_test

import (
	"strings"
	"testing"

	markdownutil "github.com/jeonghyeon-net/jeonghyeon.net/transformer/markdown-util"
)

// ExtractH1 tests

func TestExtractH1_Basic(t *testing.T) {
	src := []byte("# Hello World\n\nSome paragraph.")
	got := markdownutil.ExtractH1(src)
	if got != "Hello World" {
		t.Errorf("got %q, want %q", got, "Hello World")
	}
}

func TestExtractH1_NoH1(t *testing.T) {
	src := []byte("## Section\n\nSome paragraph.")
	got := markdownutil.ExtractH1(src)
	if got != "" {
		t.Errorf("got %q, want empty string", got)
	}
}

func TestExtractH1_WithInlineBold(t *testing.T) {
	src := []byte("# Hello **World**\n\nSome paragraph.")
	got := markdownutil.ExtractH1(src)
	if got != "Hello World" {
		t.Errorf("got %q, want %q", got, "Hello World")
	}
}

// ExtractFirstParagraph tests

func TestExtractFirstParagraph_Basic(t *testing.T) {
	src := []byte("# Title\n\nThis is a paragraph.")
	got := markdownutil.ExtractFirstParagraph(src)
	if got != "This is a paragraph." {
		t.Errorf("got %q, want %q", got, "This is a paragraph.")
	}
}

func TestExtractFirstParagraph_WithBold(t *testing.T) {
	src := []byte("A paragraph with **bold** text.")
	got := markdownutil.ExtractFirstParagraph(src)
	if got != "A paragraph with bold text." {
		t.Errorf("got %q, want %q", got, "A paragraph with bold text.")
	}
}

func TestExtractFirstParagraph_WithLink(t *testing.T) {
	src := []byte("See [the docs](https://example.com) for more.")
	got := markdownutil.ExtractFirstParagraph(src)
	if got != "See the docs for more." {
		t.Errorf("got %q, want %q", got, "See the docs for more.")
	}
}

func TestExtractFirstParagraph_WithCodeSpan(t *testing.T) {
	src := []byte("Use `go build` to compile.")
	got := markdownutil.ExtractFirstParagraph(src)
	if got != "Use go build to compile." {
		t.Errorf("got %q, want %q", got, "Use go build to compile.")
	}
}

func TestExtractFirstParagraph_Empty(t *testing.T) {
	src := []byte("")
	got := markdownutil.ExtractFirstParagraph(src)
	if got != "" {
		t.Errorf("got %q, want empty string", got)
	}
}

func TestExtractFirstParagraph_HeadingOnly(t *testing.T) {
	src := []byte("# Just a heading")
	got := markdownutil.ExtractFirstParagraph(src)
	if got != "" {
		t.Errorf("got %q, want empty string", got)
	}
}

// MarkdownToHTML tests

func TestMarkdownToHTML_Basic(t *testing.T) {
	src := []byte("# Title\n\nA paragraph.")
	html, err := markdownutil.MarkdownToHTML(src)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(html, "<h1>") {
		t.Errorf("expected <h1> in output, got: %s", html)
	}
	if !strings.Contains(html, "<p>") {
		t.Errorf("expected <p> in output, got: %s", html)
	}
}
