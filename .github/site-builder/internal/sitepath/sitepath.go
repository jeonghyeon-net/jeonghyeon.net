package sitepath

import (
	"path/filepath"
	"strings"
)

// IsExcluded reports whether path belongs to repository metadata or build output
// at the site source root.
func IsExcluded(sourceRoot, path string) bool {
	relPath, err := filepath.Rel(sourceRoot, path)
	if err != nil {
		return false
	}

	slashRel := filepath.ToSlash(filepath.Clean(relPath))
	if slashRel == "." {
		return false
	}

	rootName := strings.SplitN(slashRel, "/", 2)[0]
	if strings.HasPrefix(rootName, ".") || rootName == "dist" {
		return true
	}

	switch strings.ToLower(rootName) {
	case "authors", "authors.md", "changelog.md", "code_of_conduct.md", "contributing.md",
		"go.mod", "go.sum", "license", "license.md", "license.txt", "makefile",
		"package-lock.json", "package.json", "pnpm-lock.yaml", "readme", "readme.md",
		"security.md", "yarn.lock":
		return true
	default:
		return false
	}
}
