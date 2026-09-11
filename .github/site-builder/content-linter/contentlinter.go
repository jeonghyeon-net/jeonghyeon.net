package contentlinter

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/text"

	"github.com/jeonghyeon-net/jeonghyeon.net/.github/site-builder/internal/sitepath"
	markdownutil "github.com/jeonghyeon-net/jeonghyeon.net/.github/site-builder/markdown-util"
)

// LintError represents a single linting error.
type LintError struct {
	File    string
	Code    string
	Message string
}

func (e LintError) Error() string {
	return fmt.Sprintf("%s: [%s] %s", e.File, e.Code, e.Message)
}

func isLayoutPath(relPath string) bool {
	slashRel := filepath.ToSlash(relPath)
	return slashRel == "header" || strings.HasPrefix(slashRel, "header/") ||
		slashRel == "footer" || strings.HasPrefix(slashRel, "footer/")
}

// LintFile checks a single markdown file for policy violations.
// path is the absolute (or working-directory-relative) path to read the file.
// relPath is the path relative to the content root, used for exemption logic.
func LintFile(path string, relPath string) []LintError {
	content, err := os.ReadFile(path)
	if err != nil {
		return []LintError{{File: path, Code: "read-error", Message: err.Error()}}
	}

	var errs []LintError

	// 1. frontmatter-forbidden: content must NOT start with "---" at byte 0 (no TrimSpace)
	if strings.HasPrefix(string(content), "---") {
		errs = append(errs, LintError{
			File:    relPath,
			Code:    "frontmatter-forbidden",
			Message: "file must not contain frontmatter (starts with ---)",
		})
	}

	// 2. html-forbidden: parse with goldmark, walk AST for HTML nodes
	reader := text.NewReader(content)
	parser := goldmark.DefaultParser()
	doc := parser.Parse(reader)

	hasHTML := false
	ast.Walk(doc, func(n ast.Node, entering bool) (ast.WalkStatus, error) {
		if !entering {
			return ast.WalkContinue, nil
		}
		if n.Kind() == ast.KindHTMLBlock || n.Kind() == ast.KindRawHTML {
			hasHTML = true
			return ast.WalkStop, nil
		}
		return ast.WalkContinue, nil
	})
	if hasHTML {
		errs = append(errs, LintError{
			File:    relPath,
			Code:    "html-forbidden",
			Message: "file must not contain raw HTML",
		})
	}

	// 3. h1-required: skip header and footer layout files
	if !isLayoutPath(relPath) {
		h1 := markdownutil.ExtractH1(content)
		if h1 == "" {
			errs = append(errs, LintError{
				File:    relPath,
				Code:    "h1-required",
				Message: "file must contain an H1 heading",
			})
		}
	}

	// 4. syntax-error: unclosed code fences (count lines starting with ```)
	lines := strings.Split(string(content), "\n")
	fenceCount := 0
	for _, line := range lines {
		if strings.HasPrefix(line, "```") {
			fenceCount++
		}
	}
	if fenceCount%2 != 0 {
		errs = append(errs, LintError{
			File:    relPath,
			Code:    "syntax-error",
			Message: "file has an unclosed code fence",
		})
	}

	return errs
}

// seriesPrefixRe matches folder names starting with two digits and a dash.
var seriesPrefixRe = regexp.MustCompile(`^\d{2}-`)

// LintDir validates the entire content directory for structural and image policy violations.
// It also calls LintFile for each .md file found.
func LintDir(contentDir string) []LintError {
	var errs []LintError

	// 1. layout-required: header and footer must exist
	for _, name := range []string{"header/index.md", "footer/index.md"} {
		p := filepath.Join(contentDir, name)
		if _, err := os.Stat(p); os.IsNotExist(err) {
			errs = append(errs, LintError{
				File:    name,
				Code:    "layout-required",
				Message: fmt.Sprintf("required layout file %s is missing", name),
			})
		}
	}

	// Walk the entire content directory for per-file checks.
	err := filepath.WalkDir(contentDir, func(absPath string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if sitepath.IsExcluded(contentDir, absPath) {
			if d.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		if d.IsDir() {
			return nil
		}

		relPath, _ := filepath.Rel(contentDir, absPath)
		ext := strings.ToLower(filepath.Ext(relPath))

		// 6. image-not-webp: non-WebP images are forbidden
		// Exception: favicon.ico at root
		forbiddenExts := map[string]bool{
			".png": true, ".jpg": true, ".jpeg": true,
			".gif": true, ".bmp": true, ".tiff": true,
			".svg": true, ".ico": true,
		}
		isFavicon := relPath == "favicon.ico"
		if forbiddenExts[ext] && !isFavicon {
			errs = append(errs, LintError{
				File:    relPath,
				Code:    "image-not-webp",
				Message: fmt.Sprintf("image %s must be converted to WebP", relPath),
			})
		}

		// 7. css-js-forbidden: .css and .js files are not allowed
		if ext == ".css" || ext == ".js" {
			errs = append(errs, LintError{
				File:    relPath,
				Code:    "css-js-forbidden",
				Message: fmt.Sprintf("%s files are not allowed", ext),
			})
		}

		// Only process .md files for the remaining checks
		if ext != ".md" {
			return nil
		}

		// 2. posts-loose-md: .md files directly under posts/ are forbidden
		parts := strings.SplitN(relPath, string(filepath.Separator), 3)
		if len(parts) == 2 && parts[0] == "posts" && filepath.Base(relPath) != "index.md" {
			errs = append(errs, LintError{
				File:    relPath,
				Code:    "posts-loose-md",
				Message: fmt.Sprintf("markdown file %s must be inside a subfolder of posts/", relPath),
			})
		}

		// 3. page-not-index: all .md files must be named index.md inside their folder
		// Exceptions: root-level index.md, 404.md, and layout files
		base := filepath.Base(relPath)
		dir := filepath.Dir(relPath)
		isRootLevel := dir == "."
		isLayoutFile := isLayoutPath(relPath)
		isExempt := isLayoutFile || (isRootLevel && (base == "index.md" || base == "404.md"))
		if !isExempt && base != "index.md" {
			errs = append(errs, LintError{
				File:    relPath,
				Code:    "page-not-index",
				Message: fmt.Sprintf("markdown file %s must be named index.md", relPath),
			})
		}

		// Run per-file lint
		fileErrs := LintFile(absPath, relPath)
		errs = append(errs, fileErrs...)

		return nil
	})
	if err != nil {
		errs = append(errs, LintError{File: contentDir, Code: "walk-error", Message: err.Error()})
		return errs
	}

	// 4 & 5. series-mixed and series-duplicate: check subfolders of any directory
	err = filepath.WalkDir(contentDir, func(absPath string, d fs.DirEntry, err error) error {
		if err != nil || !d.IsDir() {
			return err
		}
		if sitepath.IsExcluded(contentDir, absPath) {
			return filepath.SkipDir
		}
		relDir, _ := filepath.Rel(contentDir, absPath)
		if isLayoutPath(relDir) {
			return filepath.SkipDir
		}

		entries, readErr := os.ReadDir(absPath)
		if readErr != nil {
			return nil
		}

		var subDirs []string
		for _, e := range entries {
			if e.IsDir() && !sitepath.IsExcluded(contentDir, filepath.Join(absPath, e.Name())) {
				subDirs = append(subDirs, e.Name())
			}
		}

		if len(subDirs) == 0 {
			return nil
		}

		// Count how many subdirs match the series pattern
		var seriesDirs []string
		var nonSeriesDirs []string
		for _, name := range subDirs {
			if seriesPrefixRe.MatchString(name) {
				seriesDirs = append(seriesDirs, name)
			} else {
				nonSeriesDirs = append(nonSeriesDirs, name)
			}
		}

		// 4. series-mixed: if any subfolder has series prefix, ALL must have it
		if len(seriesDirs) > 0 && len(nonSeriesDirs) > 0 {
			for _, name := range nonSeriesDirs {
				entryRel := filepath.Join(relDir, name)
				errs = append(errs, LintError{
					File:    entryRel,
					Code:    "series-mixed",
					Message: fmt.Sprintf("directory %s mixes series-prefixed and non-prefixed subfolders", entryRel),
				})
			}
		}

		// 5. series-duplicate: no two subfolders can share the same 2-digit prefix
		prefixCount := make(map[string][]string)
		for _, name := range seriesDirs {
			prefix := name[:2]
			prefixCount[prefix] = append(prefixCount[prefix], name)
		}
		for _, names := range prefixCount {
			if len(names) > 1 {
				for _, name := range names {
					entryRel := filepath.Join(relDir, name)
					errs = append(errs, LintError{
						File:    entryRel,
						Code:    "series-duplicate",
						Message: fmt.Sprintf("directory %s shares series prefix with another subfolder", entryRel),
					})
				}
			}
		}

		return nil
	})
	if err != nil {
		errs = append(errs, LintError{File: contentDir, Code: "walk-error", Message: err.Error()})
	}

	return errs
}

// FixDir repairs lint violations that have deterministic fixes under the site policy.
func FixDir(contentDir string) error {
	for _, name := range []string{"header/index.md", "footer/index.md"} {
		path := filepath.Join(contentDir, name)
		if _, err := os.Stat(path); os.IsNotExist(err) {
			if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
				return fmt.Errorf("create layout directory for %s: %w", name, err)
			}
			if err := os.WriteFile(path, nil, 0644); err != nil {
				return fmt.Errorf("create layout file %s: %w", name, err)
			}
		} else if err != nil {
			return fmt.Errorf("inspect layout file %s: %w", name, err)
		}
	}

	var markdownPaths []string
	err := filepath.WalkDir(contentDir, func(absPath string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if sitepath.IsExcluded(contentDir, absPath) {
			if d.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		if !d.IsDir() && strings.EqualFold(filepath.Ext(absPath), ".md") {
			markdownPaths = append(markdownPaths, absPath)
		}
		return nil
	})
	if err != nil {
		return fmt.Errorf("scan markdown files: %w", err)
	}

	frontmatterRe := regexp.MustCompile(`(?s)^---\r?\n.*?\r?\n---(?:\r?\n|$)`)
	linkedImageRe := regexp.MustCompile(`<a[ \t]+href="([^"()<>\r\n]+)"[ \t]*>[ \t]*<img[ \t]+src="([^"()<>\r\n]+)"[ \t]+alt="([^"\[\]\r\n]*)"(?:[ \t]+(?:width|height)="[^"]*")*[ \t]*/?>[ \t]*</a>`)
	for _, originalPath := range markdownPaths {
		absPath := originalPath
		relPath, err := filepath.Rel(contentDir, absPath)
		if err != nil {
			return fmt.Errorf("resolve relative path for %s: %w", absPath, err)
		}

		base := filepath.Base(relPath)
		dir := filepath.Dir(relPath)
		isRootLevel := dir == "."
		isLayoutFile := isLayoutPath(relPath)
		isExempt := isLayoutFile || (isRootLevel && (base == "index.md" || base == "404.md"))
		if !isExempt && base != "index.md" {
			pageDir := strings.TrimSuffix(base, filepath.Ext(base))
			destination := filepath.Join(filepath.Dir(absPath), pageDir, "index.md")
			if _, err := os.Stat(destination); err == nil {
				return fmt.Errorf("cannot move %s: %s already exists", relPath, destination)
			} else if !os.IsNotExist(err) {
				return fmt.Errorf("inspect destination %s: %w", destination, err)
			}
			if err := os.MkdirAll(filepath.Dir(destination), 0755); err != nil {
				return fmt.Errorf("create page directory for %s: %w", relPath, err)
			}
			if err := os.Rename(absPath, destination); err != nil {
				return fmt.Errorf("move %s to index file: %w", relPath, err)
			}
			absPath = destination
			relPath, err = filepath.Rel(contentDir, absPath)
			if err != nil {
				return fmt.Errorf("resolve moved path for %s: %w", absPath, err)
			}
		}

		info, err := os.Stat(absPath)
		if err != nil {
			return fmt.Errorf("inspect markdown file %s: %w", relPath, err)
		}
		data, err := os.ReadFile(absPath)
		if err != nil {
			return fmt.Errorf("read markdown file %s: %w", relPath, err)
		}
		fixed := frontmatterRe.ReplaceAllString(string(data), "")
		fixed = linkedImageRe.ReplaceAllString(fixed, `[![$3]($2)]($1)`)

		if !strings.HasPrefix(fixed, "---") && !isLayoutPath(relPath) && markdownutil.ExtractH1([]byte(fixed)) == "" {
			slashRel := filepath.ToSlash(relPath)
			title := ""
			switch slashRel {
			case "index.md":
				title = markdownutil.LoadConfig(contentDir).Name
				if title == "" {
					title = "Home"
				}
			case "404.md":
				title = "404"
			default:
				title = filepath.Base(filepath.Dir(relPath))
				if seriesPrefixRe.MatchString(title) {
					title = title[3:]
				}
				title = strings.TrimSpace(strings.NewReplacer("-", " ", "_", " ").Replace(title))
				if title == "" {
					title = "Untitled"
				}
				runes := []rune(title)
				upper := []rune(strings.ToUpper(string(runes[0])))
				runes[0] = upper[0]
				title = string(runes)
			}
			fixed = "# " + title + "\n\n" + strings.TrimLeft(fixed, "\r\n")
		}

		fenceCount := 0
		for _, line := range strings.Split(fixed, "\n") {
			if strings.HasPrefix(line, "```") {
				fenceCount++
			}
		}
		if fenceCount%2 != 0 {
			if !strings.HasSuffix(fixed, "\n") {
				fixed += "\n"
			}
			fixed += "```\n"
		}

		if fixed != string(data) {
			if err := os.WriteFile(absPath, []byte(fixed), info.Mode().Perm()); err != nil {
				return fmt.Errorf("write fixed markdown file %s: %w", relPath, err)
			}
		}
	}

	return nil
}
