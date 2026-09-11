package contentlinter_test

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"

	contentlinter "github.com/jeonghyeon-net/jeonghyeon.net/.github/site-builder/content-linter"
)

func testdataPath(name string) string {
	_, filename, _, _ := runtime.Caller(0)
	dir := filepath.Dir(filename)
	return filepath.Join(dir, "testdata", name)
}

func hasCode(errs []contentlinter.LintError, code string) bool {
	for _, e := range errs {
		if e.Code == code {
			return true
		}
	}
	return false
}

// ── Task 3: LintFile tests ────────────────────────────────────────────────────

func TestLintFile_ValidMarkdown(t *testing.T) {
	path := testdataPath("valid.md")
	errs := contentlinter.LintFile(path, "valid.md")
	if len(errs) != 0 {
		t.Errorf("expected no errors, got: %v", errs)
	}
}

func TestLintFile_FrontmatterForbidden(t *testing.T) {
	path := testdataPath("has_frontmatter.md")
	errs := contentlinter.LintFile(path, "has_frontmatter.md")
	if !hasCode(errs, "frontmatter-forbidden") {
		t.Errorf("expected frontmatter-forbidden error, got: %v", errs)
	}
}

func TestLintFile_LeadingWhitespaceNotFrontmatter(t *testing.T) {
	path := testdataPath("leading_whitespace.md")
	errs := contentlinter.LintFile(path, "leading_whitespace.md")
	if hasCode(errs, "frontmatter-forbidden") {
		t.Errorf("expected no frontmatter-forbidden error for file with leading whitespace, got: %v", errs)
	}
}

func TestLintFile_HTMLForbidden(t *testing.T) {
	path := testdataPath("has_html.md")
	errs := contentlinter.LintFile(path, "has_html.md")
	if !hasCode(errs, "html-forbidden") {
		t.Errorf("expected html-forbidden error, got: %v", errs)
	}
}

func TestLintFile_H1Required(t *testing.T) {
	path := testdataPath("no_h1.md")
	errs := contentlinter.LintFile(path, "no_h1.md")
	if !hasCode(errs, "h1-required") {
		t.Errorf("expected h1-required error, got: %v", errs)
	}
}

func TestLintFile_LayoutExemptFromH1(t *testing.T) {
	path := testdataPath("no_h1.md")
	for _, relPath := range []string{"header/index.md", "footer/index.md"} {
		errs := contentlinter.LintFile(path, relPath)
		if hasCode(errs, "h1-required") {
			t.Errorf("expected no h1-required error for %s, got: %v", relPath, errs)
		}
	}
}

func TestLintFile_UnclosedCodeFence(t *testing.T) {
	path := testdataPath("unclosed_fence.md")
	errs := contentlinter.LintFile(path, "unclosed_fence.md")
	if !hasCode(errs, "syntax-error") {
		t.Errorf("expected syntax-error error, got: %v", errs)
	}
}

// ── Task 4: LintDir tests ─────────────────────────────────────────────────────

func TestLintDir_BlogLooseMdForbidden(t *testing.T) {
	dir := testdataPath("bad-structure")
	errs := contentlinter.LintDir(dir)
	if !hasCode(errs, "posts-loose-md") {
		t.Errorf("expected posts-loose-md error, got: %v", errs)
	}
}

func TestLintDir_PageStructureForbidden(t *testing.T) {
	dir := testdataPath("bad-page-structure")
	errs := contentlinter.LintDir(dir)
	if !hasCode(errs, "page-not-index") {
		t.Errorf("expected page-not-index error, got: %v", errs)
	}
}

func TestLintDir_LayoutRequired(t *testing.T) {
	dir := testdataPath("no-layout")
	errs := contentlinter.LintDir(dir)
	if !hasCode(errs, "layout-required") {
		t.Errorf("expected layout-required error, got: %v", errs)
	}
}

func TestLintDir_SeriesMixedForbidden(t *testing.T) {
	dir := testdataPath("bad-series")
	errs := contentlinter.LintDir(dir)
	if !hasCode(errs, "series-mixed") {
		t.Errorf("expected series-mixed error, got: %v", errs)
	}
}

func TestLintDir_SeriesDuplicateForbidden(t *testing.T) {
	dir := testdataPath("dup-series")
	errs := contentlinter.LintDir(dir)
	if !hasCode(errs, "series-duplicate") {
		t.Errorf("expected series-duplicate error, got: %v", errs)
	}
}

func TestLintDir_ImageWebpOnly(t *testing.T) {
	dir := testdataPath("bad-image")
	errs := contentlinter.LintDir(dir)
	if !hasCode(errs, "image-not-webp") {
		t.Errorf("expected image-not-webp error, got: %v", errs)
	}
}

func TestLintDir_ValidStructure(t *testing.T) {
	dir := testdataPath("good-structure")
	errs := contentlinter.LintDir(dir)
	if len(errs) != 0 {
		t.Errorf("expected no errors for valid structure, got: %v", errs)
	}
}

func TestLintDir_SkipsRepositoryFiles(t *testing.T) {
	root := t.TempDir()
	files := map[string]string{
		"index.md":                    "# Home\n",
		"header/index.md":             "Header\n",
		"footer/index.md":             "Footer\n",
		".github/site-builder/bad.md": "<script>alert(1)</script>\n",
		".gitignore":                  "dist/\n",
		"README.md":                   "<script>alert(1)</script>\n",
		"dist/stale.md":               "<script>alert(1)</script>\n",
	}
	for name, content := range files {
		path := filepath.Join(root, name)
		if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(content), 0644); err != nil {
			t.Fatal(err)
		}
	}

	errs := contentlinter.LintDir(root)
	if len(errs) != 0 {
		t.Errorf("expected repository files to be skipped, got: %v", errs)
	}
}

func TestFixDir_RepairsMarkdownViolations(t *testing.T) {
	root := t.TempDir()
	files := map[string]string{
		"index.md":                    "# Home\n",
		"header/index.md":             "Header\n",
		"footer/index.md":             "Footer\n",
		"posts/my-post/index.md":      "---\ntitle: ignored\n---\nIntro.\n\n```go\nfmt.Println(\"hello\")\n",
		".github/site-builder/bad.md": "---\nprivate: true\n---\n",
	}
	for name, content := range files {
		path := filepath.Join(root, name)
		if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(content), 0644); err != nil {
			t.Fatal(err)
		}
	}

	if err := contentlinter.FixDir(root); err != nil {
		t.Fatalf("FixDir error: %v", err)
	}

	fixed, err := os.ReadFile(filepath.Join(root, "posts/my-post/index.md"))
	if err != nil {
		t.Fatal(err)
	}
	want := "# My post\n\nIntro.\n\n```go\nfmt.Println(\"hello\")\n```\n"
	if string(fixed) != want {
		t.Errorf("unexpected fixed markdown:\nwant: %q\ngot:  %q", want, string(fixed))
	}
	private, err := os.ReadFile(filepath.Join(root, ".github/site-builder/bad.md"))
	if err != nil {
		t.Fatal(err)
	}
	if string(private) != files[".github/site-builder/bad.md"] {
		t.Error("expected repository metadata to remain untouched")
	}
	if errs := contentlinter.LintDir(root); len(errs) != 0 {
		t.Errorf("expected fixed content to pass lint, got: %v", errs)
	}
}

func TestFixDir_MovesPageToIndexFile(t *testing.T) {
	root := t.TempDir()
	files := map[string]string{
		"index.md":         "# Home\n",
		"header/index.md":  "Header\n",
		"footer/index.md":  "Footer\n",
		"posts/my-post.md": "# My Post\n",
	}
	for name, content := range files {
		path := filepath.Join(root, name)
		if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(content), 0644); err != nil {
			t.Fatal(err)
		}
	}

	if err := contentlinter.FixDir(root); err != nil {
		t.Fatalf("FixDir error: %v", err)
	}
	if _, err := os.Stat(filepath.Join(root, "posts/my-post.md")); !os.IsNotExist(err) {
		t.Error("expected loose page to be moved")
	}
	if _, err := os.Stat(filepath.Join(root, "posts/my-post/index.md")); err != nil {
		t.Errorf("expected page at posts/my-post/index.md: %v", err)
	}
	if errs := contentlinter.LintDir(root); len(errs) != 0 {
		t.Errorf("expected moved page to pass lint, got: %v", errs)
	}
}

func TestFixDir_CreatesMissingLayouts(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "index.md"), []byte("# Home\n"), 0644); err != nil {
		t.Fatal(err)
	}

	if err := contentlinter.FixDir(root); err != nil {
		t.Fatalf("FixDir error: %v", err)
	}
	for _, name := range []string{"header/index.md", "footer/index.md"} {
		if _, err := os.Stat(filepath.Join(root, name)); err != nil {
			t.Errorf("expected %s to be created: %v", name, err)
		}
	}
	if errs := contentlinter.LintDir(root); len(errs) != 0 {
		t.Errorf("expected generated layouts to pass lint, got: %v", errs)
	}
}

func TestFixDir_DoesNotHideUnclosedFrontmatter(t *testing.T) {
	root := t.TempDir()
	files := map[string]string{
		"index.md":               "# Home\n",
		"header/index.md":        "Header\n",
		"footer/index.md":        "Footer\n",
		"posts/my-post/index.md": "---\ntitle: unfinished\n",
	}
	for name, content := range files {
		path := filepath.Join(root, name)
		if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(content), 0644); err != nil {
			t.Fatal(err)
		}
	}

	if err := contentlinter.FixDir(root); err != nil {
		t.Fatalf("FixDir error: %v", err)
	}
	if errs := contentlinter.LintDir(root); !hasCode(errs, "frontmatter-forbidden") {
		t.Errorf("expected unsafe frontmatter to remain a lint error, got: %v", errs)
	}
}

func TestFixDir_LeavesRawHTMLAsLintError(t *testing.T) {
	root := t.TempDir()
	files := map[string]string{
		"index.md":               "# Home\n",
		"header/index.md":        "Header\n",
		"footer/index.md":        "Footer\n",
		"posts/my-post/index.md": "# My post\n\n<div>unsafe</div>\n",
	}
	for name, content := range files {
		path := filepath.Join(root, name)
		if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(content), 0644); err != nil {
			t.Fatal(err)
		}
	}

	if err := contentlinter.FixDir(root); err != nil {
		t.Fatalf("FixDir error: %v", err)
	}
	if errs := contentlinter.LintDir(root); !hasCode(errs, "html-forbidden") {
		t.Errorf("expected unsafe HTML to remain a lint error, got: %v", errs)
	}
}

func TestFixDir_ConvertsLinkedImageHTML(t *testing.T) {
	root := t.TempDir()
	files := map[string]string{
		"index.md":        "# Home\n",
		"header/index.md": "Header\n",
		"footer/index.md": "<a href=\"https://example.com\"><img src=\"https://example.com/badge.gif\" alt=\"Visitor counter\" width=\"88\" height=\"31\" /></a>\n",
	}
	for name, content := range files {
		path := filepath.Join(root, name)
		if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(content), 0644); err != nil {
			t.Fatal(err)
		}
	}

	if err := contentlinter.FixDir(root); err != nil {
		t.Fatalf("FixDir error: %v", err)
	}
	fixed, err := os.ReadFile(filepath.Join(root, "footer/index.md"))
	if err != nil {
		t.Fatal(err)
	}
	want := "[![Visitor counter](https://example.com/badge.gif)](https://example.com)\n"
	if string(fixed) != want {
		t.Errorf("unexpected linked image conversion:\nwant: %q\ngot:  %q", want, string(fixed))
	}
	if errs := contentlinter.LintDir(root); len(errs) != 0 {
		t.Errorf("expected converted linked image to pass lint, got: %v", errs)
	}
}
