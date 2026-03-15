package autoindex

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	markdownutil "github.com/jeonghyeon-net/jeonghyeon.net/transformer/markdown-util"
)

var seriesPrefixRe = regexp.MustCompile(`^\d{2}-`)

// titleFromDirName converts a directory name to a title:
// strips a 2-digit prefix (e.g. "01-") and converts kebab-case to Title Case.
func titleFromDirName(name string) string {
	// Strip series prefix like "01-"
	if seriesPrefixRe.MatchString(name) {
		name = name[3:]
	}
	// Convert kebab-case to Title Case
	parts := strings.Split(name, "-")
	for i, p := range parts {
		if len(p) > 0 {
			parts[i] = strings.ToUpper(p[:1]) + p[1:]
		}
	}
	return strings.Join(parts, " ")
}

// extractTitle tries to read a title from a subfolder's index.md using ExtractH1.
// Falls back to titleFromDirName.
func extractTitle(folderPath, dirName string) string {
	indexPath := filepath.Join(folderPath, "index.md")
	data, err := os.ReadFile(indexPath)
	if err != nil {
		return titleFromDirName(dirName)
	}
	h1 := markdownutil.ExtractH1(data)
	if h1 == "" {
		return titleFromDirName(dirName)
	}
	return h1
}

// isSeries returns true if any subdirectory name matches the series prefix pattern.
func isSeries(subDirs []string) bool {
	for _, d := range subDirs {
		if seriesPrefixRe.MatchString(d) {
			return true
		}
	}
	return false
}

// generateMarkdown produces the markdown content for an auto-generated index.
func generateMarkdown(folderPath, folderName string, subDirs []string, cfg markdownutil.SiteConfig) string {
	title := cfg.Titles[folderName]
	if title == "" {
		title = titleFromDirName(folderName)
	}
	series := isSeries(subDirs)

	// Sort: alphabetically for regular, numerically (alphabetically on prefixed names) for series.
	sorted := make([]string, len(subDirs))
	copy(sorted, subDirs)
	sort.Strings(sorted)

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("# %s\n\n", title))

	for i, dir := range sorted {
		itemTitle := extractTitle(filepath.Join(folderPath, dir), dir)
		if series {
			sb.WriteString(fmt.Sprintf("%d. [%s](%s/)\n", i+1, itemTitle, dir))
		} else {
			sb.WriteString(fmt.Sprintf("- [%s](%s/)\n", itemTitle, dir))
		}
	}

	return sb.String()
}

// Generate scans contentDir and returns a map of (slash-separated relative path) -> generated markdown
// for every folder that is missing an index.md and has at least one subdirectory.
// Skips the root dir (".") and "_layout/".
func Generate(contentDir string) (map[string]string, error) {
	cfg := markdownutil.LoadConfig(contentDir)
	result := make(map[string]string)

	err := filepath.WalkDir(contentDir, func(absPath string, d os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if !d.IsDir() {
			return nil
		}

		relPath, err := filepath.Rel(contentDir, absPath)
		if err != nil {
			return err
		}

		// Skip root
		if relPath == "." {
			return nil
		}

		// Skip _layout/ and everything inside it
		slashRel := filepath.ToSlash(relPath)
		if slashRel == "_layout" || strings.HasPrefix(slashRel, "_layout/") {
			return filepath.SkipDir
		}

		// Skip if index.md already exists
		indexPath := filepath.Join(absPath, "index.md")
		if _, err := os.Stat(indexPath); err == nil {
			return nil
		}

		// Read subdirectories
		entries, err := os.ReadDir(absPath)
		if err != nil {
			return err
		}

		var subDirs []string
		for _, e := range entries {
			if e.IsDir() {
				subDirs = append(subDirs, e.Name())
			}
		}

		// Only generate if there are subdirectories
		if len(subDirs) == 0 {
			return nil
		}

		folderName := filepath.Base(absPath)
		content := generateMarkdown(absPath, folderName, subDirs, cfg)

		key := filepath.ToSlash(filepath.Join(relPath, "index.md"))
		result[key] = content

		return nil
	})

	if err != nil {
		return nil, err
	}

	return result, nil
}

// WriteGenerated writes the generated markdown files to disk.
// Returns the list of created absolute file paths.
func WriteGenerated(contentDir string, generated map[string]string) ([]string, error) {
	var created []string

	for slashRel, content := range generated {
		absPath := filepath.Join(contentDir, filepath.FromSlash(slashRel))

		// Ensure parent directory exists
		if err := os.MkdirAll(filepath.Dir(absPath), 0755); err != nil {
			return created, fmt.Errorf("mkdir %s: %w", filepath.Dir(absPath), err)
		}

		if err := os.WriteFile(absPath, []byte(content), 0644); err != nil {
			return created, fmt.Errorf("write %s: %w", absPath, err)
		}

		created = append(created, absPath)
	}

	return created, nil
}
