package autoindex

import (
	"strings"
	"testing"
)

func TestGenerate_BasicIndex(t *testing.T) {
	generated, err := Generate("testdata/basic")
	if err != nil {
		t.Fatalf("Generate error: %v", err)
	}

	content, ok := generated["blog/index.md"]
	if !ok {
		t.Fatalf("expected blog/index.md to be generated, got keys: %v", keys(generated))
	}

	if !strings.Contains(content, "[Post A](post-a/)") {
		t.Errorf("expected link to post-a, got:\n%s", content)
	}
	if !strings.Contains(content, "[Post B](post-b/)") {
		t.Errorf("expected link to post-b, got:\n%s", content)
	}
	// unordered list
	if !strings.Contains(content, "- [") {
		t.Errorf("expected unordered list, got:\n%s", content)
	}
}

func TestGenerate_SeriesIndex(t *testing.T) {
	generated, err := Generate("testdata/series")
	if err != nil {
		t.Fatalf("Generate error: %v", err)
	}

	content, ok := generated["blog/my-series/index.md"]
	if !ok {
		t.Fatalf("expected blog/my-series/index.md to be generated, got keys: %v", keys(generated))
	}

	// ordered list
	if !strings.Contains(content, "1. [") {
		t.Errorf("expected ordered list, got:\n%s", content)
	}
	if !strings.Contains(content, "[Introduction](01-intro/)") {
		t.Errorf("expected link to 01-intro with title Introduction, got:\n%s", content)
	}
	if !strings.Contains(content, "[Details](02-details/)") {
		t.Errorf("expected link to 02-details with title Details, got:\n%s", content)
	}
}

func TestGenerate_SkipsExistingIndex(t *testing.T) {
	generated, err := Generate("testdata/has-index")
	if err != nil {
		t.Fatalf("Generate error: %v", err)
	}

	if _, ok := generated["blog/index.md"]; ok {
		t.Error("expected blog/index.md to be skipped (already exists)")
	}
}

func TestGenerate_EmptyDir(t *testing.T) {
	generated, err := Generate("testdata/empty-dir")
	if err != nil {
		t.Fatalf("Generate error: %v", err)
	}

	if len(generated) != 0 {
		t.Errorf("expected no generation for empty dir, got: %v", generated)
	}
}

func keys(m map[string]string) []string {
	result := make([]string, 0, len(m))
	for k := range m {
		result = append(result, k)
	}
	return result
}
