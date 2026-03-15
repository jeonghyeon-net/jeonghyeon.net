package htmlminifier

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestMinifyDir(t *testing.T) {
	dir := t.TempDir()

	// Create an HTML file with extra whitespace
	htmlContent := `<!DOCTYPE html>
<html>
  <head>
    <title>Test</title>
  </head>
  <body>
    <p>Hello   World</p>
  </body>
</html>`
	htmlPath := filepath.Join(dir, "index.html")
	if err := os.WriteFile(htmlPath, []byte(htmlContent), 0644); err != nil {
		t.Fatal(err)
	}

	// Create a .md file that must NOT be touched
	mdContent := "# Hello\n\nThis is **markdown**.\n"
	mdPath := filepath.Join(dir, "README.md")
	if err := os.WriteFile(mdPath, []byte(mdContent), 0644); err != nil {
		t.Fatal(err)
	}

	if err := MinifyDir(dir); err != nil {
		t.Fatalf("MinifyDir returned error: %v", err)
	}

	// HTML should be minified (whitespace removed)
	gotHTML, err := os.ReadFile(htmlPath)
	if err != nil {
		t.Fatal(err)
	}
	minifiedHTML := string(gotHTML)
	if strings.Contains(minifiedHTML, "  ") {
		t.Errorf("expected HTML to be minified (no double spaces), got:\n%s", minifiedHTML)
	}
	if len(minifiedHTML) >= len(htmlContent) {
		t.Errorf("expected minified HTML to be shorter than original: got %d >= %d", len(minifiedHTML), len(htmlContent))
	}

	// .md file must be unchanged
	gotMD, err := os.ReadFile(mdPath)
	if err != nil {
		t.Fatal(err)
	}
	if string(gotMD) != mdContent {
		t.Errorf("expected .md file to be unchanged:\nwant: %q\ngot:  %q", mdContent, string(gotMD))
	}
}

func TestMinifyDir_SubDirectories(t *testing.T) {
	dir := t.TempDir()

	// Create nested directory with HTML
	subDir := filepath.Join(dir, "blog")
	if err := os.MkdirAll(subDir, 0755); err != nil {
		t.Fatal(err)
	}
	htmlContent := `<html>  <body>  <p>  test  </p>  </body>  </html>`
	htmlPath := filepath.Join(subDir, "index.html")
	if err := os.WriteFile(htmlPath, []byte(htmlContent), 0644); err != nil {
		t.Fatal(err)
	}

	if err := MinifyDir(dir); err != nil {
		t.Fatalf("MinifyDir returned error: %v", err)
	}

	gotHTML, err := os.ReadFile(htmlPath)
	if err != nil {
		t.Fatal(err)
	}
	if len(string(gotHTML)) >= len(htmlContent) {
		t.Errorf("expected nested HTML to be minified")
	}
}
