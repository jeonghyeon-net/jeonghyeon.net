package contentlinter_test

import (
	"path/filepath"
	"runtime"
	"testing"

	contentlinter "github.com/jeonghyeon-net/jeonghyeon.net/transformer/content-linter"
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
	// _layout/ files are exempt from h1-required; use no_h1.md but pass _layout/ relPath
	path := testdataPath("no_h1.md")
	errs := contentlinter.LintFile(path, "_layout/header.md")
	if hasCode(errs, "h1-required") {
		t.Errorf("expected no h1-required error for _layout/ files, got: %v", errs)
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
	if !hasCode(errs, "blog-loose-md") {
		t.Errorf("expected blog-loose-md error, got: %v", errs)
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
