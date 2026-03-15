package mdtohtml

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"

	markdownutil "github.com/jeonghyeon-net/jeonghyeon.net/transformer/markdown-util"
)

// escapeHTML replaces &, <, > with HTML entities.
func escapeHTML(s string) string {
	s = strings.ReplaceAll(s, "&", "&amp;")
	s = strings.ReplaceAll(s, "<", "&lt;")
	s = strings.ReplaceAll(s, ">", "&gt;")
	return s
}

// escapeAttr additionally escapes double-quotes for use in HTML attributes.
func escapeAttr(s string) string {
	s = escapeHTML(s)
	s = strings.ReplaceAll(s, `"`, "&quot;")
	return s
}

// relPathToURL converts a relative markdown path to an absolute URL.
// index.md -> siteURL + "/"
// 404.md   -> siteURL + "/404.html"
// blog/my-post/index.md -> siteURL + "/blog/my-post/"
func relPathToURL(relPath, siteURL string) string {
	slash := filepath.ToSlash(relPath)

	if slash == "index.md" {
		return siteURL + "/"
	}
	if slash == "404.md" {
		return siteURL + "/404.html"
	}
	// Remove trailing /index.md
	slash = strings.TrimSuffix(slash, "/index.md")
	return siteURL + "/" + slash + "/"
}

// pageTemplate builds a full HTML page.
func pageTemplate(title, description, headerHTML, bodyHTML, footerHTML string) string {
	return fmt.Sprintf(`<!DOCTYPE html>
<html lang="ko">
<head>
  <meta charset="utf-8">
  <meta name="viewport" content="width=device-width, initial-scale=1">
  <meta name="description" content="%s">
  <title>%s</title>
</head>
<body>
<header>%s</header>
<main>%s</main>
<footer>%s</footer>
</body>
</html>
`, escapeAttr(description), escapeHTML(title), headerHTML, bodyHTML, footerHTML)
}

// copyFile copies src to dst, creating any necessary parent directories.
func copyFile(src, dst string) error {
	if err := os.MkdirAll(filepath.Dir(dst), 0755); err != nil {
		return err
	}
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()

	out, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer out.Close()

	_, err = io.Copy(out, in)
	return err
}

type pageInfo struct {
	title       string
	description string
	url         string
	isBlog      bool
	is404       bool
}

// Render converts contentDir into distDir with HTML pages, sitemap, feed, and llms.txt.
func Render(contentDir, distDir, siteURL string) error {
	// Read layout files
	headerSrc, err := os.ReadFile(filepath.Join(contentDir, "_layout", "header", "index.md"))
	if err != nil {
		return fmt.Errorf("read header/index.md: %w", err)
	}
	footerSrc, err := os.ReadFile(filepath.Join(contentDir, "_layout", "footer", "index.md"))
	if err != nil {
		return fmt.Errorf("read footer/index.md: %w", err)
	}

	headerHTML, err := markdownutil.MarkdownToHTML(headerSrc)
	if err != nil {
		return fmt.Errorf("convert header.md: %w", err)
	}
	footerHTML, err := markdownutil.MarkdownToHTML(footerSrc)
	if err != nil {
		return fmt.Errorf("convert footer.md: %w", err)
	}

	// Trim trailing newlines from layout HTML to keep template clean
	headerHTML = strings.TrimRight(headerHTML, "\n")
	footerHTML = strings.TrimRight(footerHTML, "\n")

	var pages []pageInfo

	// Walk contentDir
	err = filepath.WalkDir(contentDir, func(absPath string, d os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}

		relPath, err := filepath.Rel(contentDir, absPath)
		if err != nil {
			return err
		}

		slashRel := filepath.ToSlash(relPath)

		// _layout: skip .md files (already read above), but copy non-.md files (badges etc.)
		if strings.HasPrefix(slashRel, "_layout/") || slashRel == "_layout" {
			if d.IsDir() {
				return nil
			}
			if filepath.Ext(relPath) != ".md" {
				return copyFile(absPath, filepath.Join(distDir, relPath))
			}
			return nil
		}

		if d.IsDir() {
			return nil
		}

		destPath := filepath.Join(distDir, relPath)

		if filepath.Ext(relPath) == ".md" {
			src, err := os.ReadFile(absPath)
			if err != nil {
				return fmt.Errorf("read %s: %w", absPath, err)
			}

			title := markdownutil.ExtractH1(src)
			description := markdownutil.ExtractFirstParagraph(src)

			bodyHTML, err := markdownutil.MarkdownToHTML(src)
			if err != nil {
				return fmt.Errorf("convert %s: %w", relPath, err)
			}

			html := pageTemplate(title, description, headerHTML, bodyHTML, footerHTML)

			// Determine output .html path
			htmlDestPath := strings.TrimSuffix(destPath, ".md") + ".html"
			if err := os.MkdirAll(filepath.Dir(htmlDestPath), 0755); err != nil {
				return err
			}
			if err := os.WriteFile(htmlDestPath, []byte(html), 0644); err != nil {
				return fmt.Errorf("write %s: %w", htmlDestPath, err)
			}

			// Also copy the original .md file
			if err := copyFile(absPath, destPath); err != nil {
				return fmt.Errorf("copy md %s: %w", relPath, err)
			}

			// Collect page info for sitemap/feed/llms
			url := relPathToURL(slashRel, siteURL)
			is404 := slashRel == "404.md"
			// Blog post = under blog/ but not blog/index.md itself (which is a listing page)
			isBlogPost := strings.HasPrefix(slashRel, "blog/") && slashRel != "blog/index.md"
			// Check if this is a leaf post (has no subdirectories with index.md = not a series listing)
			if isBlogPost {
				dir := filepath.Dir(filepath.Join(contentDir, slashRel))
				entries, _ := os.ReadDir(dir)
				for _, e := range entries {
					if e.IsDir() {
						// This folder has subdirectories → it's a series/category listing, not a post
						isBlogPost = false
						break
					}
				}
			}

			pages = append(pages, pageInfo{
				title:       title,
				description: description,
				url:         url,
				isBlog:      isBlogPost,
				is404:       is404,
			})
		} else {
			// Copy non-md files as-is
			if err := copyFile(absPath, destPath); err != nil {
				return fmt.Errorf("copy %s: %w", relPath, err)
			}
		}

		return nil
	})
	if err != nil {
		return err
	}

	// Sort pages for deterministic output
	sort.Slice(pages, func(i, j int) bool {
		return pages[i].url < pages[j].url
	})

	// Generate sitemap.xml
	if err := writeSitemap(distDir, pages); err != nil {
		return err
	}

	// Generate feed.xml
	if err := writeFeed(distDir, siteURL, pages); err != nil {
		return err
	}

	// Generate llms.txt
	if err := writeLLMs(distDir, pages); err != nil {
		return err
	}

	return nil
}

func writeSitemap(distDir string, pages []pageInfo) error {
	var sb strings.Builder
	sb.WriteString(`<?xml version="1.0" encoding="UTF-8"?>` + "\n")
	sb.WriteString(`<urlset xmlns="http://www.sitemaps.org/schemas/sitemap/0.9">` + "\n")

	for _, p := range pages {
		if p.is404 {
			continue
		}
		sb.WriteString("  <url>\n")
		sb.WriteString(fmt.Sprintf("    <loc>%s</loc>\n", escapeHTML(p.url)))
		sb.WriteString("  </url>\n")
	}

	sb.WriteString("</urlset>\n")

	return os.WriteFile(filepath.Join(distDir, "sitemap.xml"), []byte(sb.String()), 0644)
}

func writeFeed(distDir, siteURL string, pages []pageInfo) error {
	var sb strings.Builder
	sb.WriteString(`<?xml version="1.0" encoding="UTF-8"?>` + "\n")
	sb.WriteString(`<rss version="2.0">` + "\n")
	sb.WriteString("<channel>\n")
	sb.WriteString(fmt.Sprintf("  <title>%s</title>\n", escapeHTML("jeonghyeon.net")))
	sb.WriteString(fmt.Sprintf("  <link>%s</link>\n", escapeHTML(siteURL+"/")))
	sb.WriteString("  <description>Personal website — developer and musician</description>\n")

	for _, p := range pages {
		if !p.isBlog {
			continue
		}
		sb.WriteString("  <item>\n")
		sb.WriteString(fmt.Sprintf("    <title>%s</title>\n", escapeHTML(p.title)))
		sb.WriteString(fmt.Sprintf("    <link>%s</link>\n", escapeHTML(p.url)))
		sb.WriteString(fmt.Sprintf("    <guid>%s</guid>\n", escapeHTML(p.url)))
		sb.WriteString(fmt.Sprintf("    <description>%s</description>\n", escapeHTML(p.description)))
		sb.WriteString("  </item>\n")
	}

	sb.WriteString("</channel>\n")
	sb.WriteString("</rss>\n")

	return os.WriteFile(filepath.Join(distDir, "feed.xml"), []byte(sb.String()), 0644)
}

func writeLLMs(distDir string, pages []pageInfo) error {
	var sb strings.Builder
	sb.WriteString("# jeonghyeon.net\n")
	sb.WriteString("> Personal website — developer and musician\n")
	sb.WriteString("## Pages\n")

	for _, p := range pages {
		if p.is404 {
			continue
		}
		sb.WriteString(fmt.Sprintf("- [%s](%s): %s\n", p.title, p.url, p.description))
	}

	return os.WriteFile(filepath.Join(distDir, "llms.txt"), []byte(sb.String()), 0644)
}
