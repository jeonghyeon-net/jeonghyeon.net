package mdtohtml

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func runRender(t *testing.T) string {
	t.Helper()
	distDir := t.TempDir()
	err := Render("testdata/basic", distDir)
	if err != nil {
		t.Fatalf("Render error: %v", err)
	}
	return distDir
}

func readFile(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	return string(data)
}

func TestRender_BasicPage(t *testing.T) {
	distDir := runRender(t)

	html := readFile(t, filepath.Join(distDir, "index.html"))

	checks := []struct {
		label   string
		snippet string
	}{
		{"title tag", "<title>Home</title>"},
		{"h1 in main", "<h1>Home</h1>"},
		{"header present", "<header>"},
		{"footer present", "<footer>"},
		{"meta description", `content="Welcome to my site."`},
		{"viewport meta", `name="viewport"`},
		{"html lang ko", `lang="ko"`},
	}

	for _, c := range checks {
		if !strings.Contains(html, c.snippet) {
			t.Errorf("%s: expected %q in output:\n%s", c.label, c.snippet, html)
		}
	}
}

func TestRender_MarkdownOriginalCopied(t *testing.T) {
	distDir := runRender(t)

	mdPath := filepath.Join(distDir, "index.md")
	if _, err := os.Stat(mdPath); os.IsNotExist(err) {
		t.Errorf("expected index.md to be copied to dist, but not found")
	}
}

func TestRender_404Page(t *testing.T) {
	distDir := runRender(t)

	path := filepath.Join(distDir, "404.html")
	if _, err := os.Stat(path); os.IsNotExist(err) {
		t.Errorf("expected 404.html to exist in dist")
	}
}

func TestRender_LayoutMarkdownNotInOutput(t *testing.T) {
	distDir := runRender(t)

	for _, path := range []string{"header/index.md", "footer/index.md"} {
		if _, err := os.Stat(filepath.Join(distDir, path)); !os.IsNotExist(err) {
			t.Errorf("expected layout markdown %s to NOT be in dist output", path)
		}
	}
}

func TestRenderSingle_UsesFlatLayout(t *testing.T) {
	var output strings.Builder
	err := RenderSingle("testdata/basic", "testdata/basic/index.md", &output)
	if err != nil {
		t.Fatalf("RenderSingle error: %v", err)
	}
	if !strings.Contains(output.String(), "Copyright 2026") {
		t.Errorf("expected flat footer layout in output:\n%s", output.String())
	}
}

func TestRender_RobotsTxtCopied(t *testing.T) {
	distDir := runRender(t)

	path := filepath.Join(distDir, "robots.txt")
	if _, err := os.Stat(path); os.IsNotExist(err) {
		t.Errorf("expected robots.txt to be copied to dist")
	}
}

func TestRender_SitemapExcludes404(t *testing.T) {
	distDir := runRender(t)

	path := filepath.Join(distDir, "sitemap.xml")
	if _, err := os.Stat(path); os.IsNotExist(err) {
		t.Fatalf("expected sitemap.xml to exist in dist")
	}

	content := readFile(t, path)
	if !strings.Contains(content, "<urlset") {
		t.Errorf("sitemap.xml missing <urlset>")
	}
	if strings.Contains(content, "404") {
		t.Errorf("sitemap.xml must not contain 404 pages, got:\n%s", content)
	}
}

func TestRender_FeedHasGuid(t *testing.T) {
	distDir := runRender(t)

	path := filepath.Join(distDir, "feed.xml")
	if _, err := os.Stat(path); os.IsNotExist(err) {
		t.Fatalf("expected feed.xml to exist in dist")
	}

	content := readFile(t, path)
	if !strings.Contains(content, "<guid>") {
		t.Errorf("feed.xml missing <guid> element, got:\n%s", content)
	}
}

func TestRender_LLMsTxt(t *testing.T) {
	distDir := runRender(t)

	path := filepath.Join(distDir, "llms.txt")
	if _, err := os.Stat(path); os.IsNotExist(err) {
		t.Fatalf("expected llms.txt to exist in dist")
	}

	content := readFile(t, path)
	if !strings.Contains(content, "test") {
		t.Errorf("llms.txt missing site name, got:\n%s", content)
	}
}

func TestRender_SkipsRepositoryFiles(t *testing.T) {
	root := t.TempDir()
	files := map[string]string{
		"index.md":             "# Home\n\nWelcome.\n",
		"index.yaml":           "name: test\nurl: https://example.com\n",
		"header/index.md":      "Header\n",
		"footer/index.md":      "Footer\n",
		".github/internal.txt": "private\n",
		".gitignore":           "dist/\n",
		"README.md":            "repository documentation\n",
		"dist/stale.txt":       "stale\n",
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

	distDir := filepath.Join(root, "dist")
	if err := Render(root, distDir); err != nil {
		t.Fatalf("Render error: %v", err)
	}
	for _, name := range []string{".github/internal.txt", ".gitignore", "README.md", "dist/stale.txt"} {
		if _, err := os.Stat(filepath.Join(distDir, name)); !os.IsNotExist(err) {
			t.Errorf("expected repository file %s to be skipped", name)
		}
	}
	if _, err := os.Stat(filepath.Join(distDir, "index.html")); err != nil {
		t.Errorf("expected site page to be rendered: %v", err)
	}
}
