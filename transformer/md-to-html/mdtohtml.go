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
// posts/my-post/index.md -> siteURL + "/posts/my-post/"
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
func pageTemplate(cfg markdownutil.SiteConfig, title, description, pageURL, siteURL, imageURL, headerHTML, bodyHTML, footerHTML string) string {
	var b strings.Builder
	b.WriteString("<!DOCTYPE html>\n<html lang=\"" + escapeAttr(cfg.Lang) + "\">\n<head>\n")
	b.WriteString("  <meta charset=\"utf-8\">\n")
	b.WriteString("  <meta name=\"viewport\" content=\"width=device-width, initial-scale=1\">\n")
	b.WriteString("  <meta name=\"color-scheme\" content=\"light dark\">\n")
	b.WriteString("  <meta name=\"description\" content=\"" + escapeAttr(description) + "\">\n")
	b.WriteString("  <meta name=\"author\" content=\"" + escapeAttr(cfg.Author) + "\">\n")
	b.WriteString("  <meta property=\"og:title\" content=\"" + escapeAttr(title) + "\">\n")
	b.WriteString("  <meta property=\"og:description\" content=\"" + escapeAttr(description) + "\">\n")
	b.WriteString("  <meta property=\"og:url\" content=\"" + escapeAttr(pageURL) + "\">\n")
	b.WriteString("  <meta property=\"og:type\" content=\"website\">\n")
	b.WriteString("  <meta property=\"og:site_name\" content=\"" + escapeAttr(cfg.Name) + "\">\n")
	if imageURL != "" {
		b.WriteString("  <meta property=\"og:image\" content=\"" + escapeAttr(imageURL) + "\">\n")
		b.WriteString("  <meta name=\"twitter:card\" content=\"summary_large_image\">\n")
		b.WriteString("  <meta name=\"twitter:image\" content=\"" + escapeAttr(imageURL) + "\">\n")
	} else {
		b.WriteString("  <meta name=\"twitter:card\" content=\"summary\">\n")
	}
	b.WriteString("  <meta name=\"twitter:title\" content=\"" + escapeAttr(title) + "\">\n")
	b.WriteString("  <meta name=\"twitter:description\" content=\"" + escapeAttr(description) + "\">\n")
	b.WriteString("  <title>" + escapeHTML(title) + "</title>\n")
	b.WriteString("  <link rel=\"canonical\" href=\"" + escapeAttr(pageURL) + "\">\n")
	b.WriteString("  <link rel=\"alternate\" type=\"application/rss+xml\" title=\"" + escapeAttr(cfg.Name) + "\" href=\"" + escapeAttr(siteURL) + "/feed.xml\">\n")
	b.WriteString("  <link rel=\"icon\" href=\"/favicon.ico\">\n")
	b.WriteString("</head>\n<body>\n")
	b.WriteString("<font face=\"" + escapeAttr(cfg.Font) + "\">\n")
	b.WriteString("<table align=\"center\" width=\"" + cfg.Width + "\"><tr><td>\n")
	b.WriteString("<header>" + headerHTML + "</header>\n")
	b.WriteString("<main>" + bodyHTML + "</main>\n")
	b.WriteString("<footer>" + footerHTML + "</footer>\n")
	b.WriteString("</td></tr></table>\n</font>\n</body>\n</html>\n")
	return b.String()
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
	isPost      bool
	is404       bool
}

// Render converts contentDir into distDir with HTML pages, sitemap, feed, and llms.txt.
func Render(contentDir, distDir string) error {
	cfg := markdownutil.LoadConfig(contentDir)
	siteURL := cfg.URL

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
			firstImage := markdownutil.ExtractFirstImage(src)

			bodyHTML, err := markdownutil.MarkdownToHTML(src)
			if err != nil {
				return fmt.Errorf("convert %s: %w", relPath, err)
			}

			pageURL := relPathToURL(slashRel, siteURL)

			// Resolve relative image path to absolute URL for og:image
			imageURL := ""
			if firstImage != "" && !strings.HasPrefix(firstImage, "http") {
				dir := filepath.Dir(slashRel)
				imageURL = siteURL + "/" + filepath.ToSlash(filepath.Join(dir, firstImage))
			} else {
				imageURL = firstImage
			}

			html := pageTemplate(cfg, title, description, pageURL, siteURL, imageURL, headerHTML, bodyHTML, footerHTML)

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
			is404 := slashRel == "404.md"
			// Blog post = under posts/ but not posts/index.md itself (which is a listing page)
			isPost := strings.HasPrefix(slashRel, "posts/") && slashRel != "posts/index.md"
			// Check if this is a leaf post (has no subdirectories with index.md = not a series listing)
			if isPost {
				dir := filepath.Dir(filepath.Join(contentDir, slashRel))
				entries, _ := os.ReadDir(dir)
				for _, e := range entries {
					if e.IsDir() {
						// This folder has subdirectories → it's a series/category listing, not a post
						isPost = false
						break
					}
				}
			}

			pages = append(pages, pageInfo{
				title:       title,
				description: description,
				url:         pageURL,
				isPost:      isPost,
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
	if err := writeFeed(distDir, siteURL, cfg, pages); err != nil {
		return err
	}

	// Generate llms.txt
	if err := writeLLMs(distDir, cfg, pages); err != nil {
		return err
	}

	return nil
}

// RenderSingle converts a single markdown file to a full HTML page and writes it to w.
// It loads config, header, and footer from contentDir, but does not generate
// sitemap, feed, or llms.txt.
func RenderSingle(contentDir, mdPath string, w io.Writer) error {
	cfg := markdownutil.LoadConfig(contentDir)
	siteURL := cfg.URL

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

	headerHTML = strings.TrimRight(headerHTML, "\n")
	footerHTML = strings.TrimRight(footerHTML, "\n")

	src, err := os.ReadFile(mdPath)
	if err != nil {
		return fmt.Errorf("read %s: %w", mdPath, err)
	}

	title := markdownutil.ExtractH1(src)
	description := markdownutil.ExtractFirstParagraph(src)
	firstImage := markdownutil.ExtractFirstImage(src)

	bodyHTML, err := markdownutil.MarkdownToHTML(src)
	if err != nil {
		return fmt.Errorf("convert %s: %w", mdPath, err)
	}

	relPath, err := filepath.Rel(contentDir, mdPath)
	if err != nil {
		return fmt.Errorf("rel path: %w", err)
	}
	slashRel := filepath.ToSlash(relPath)
	pageURL := relPathToURL(slashRel, siteURL)

	imageURL := ""
	if firstImage != "" && !strings.HasPrefix(firstImage, "http") {
		dir := filepath.Dir(slashRel)
		imageURL = siteURL + "/" + filepath.ToSlash(filepath.Join(dir, firstImage))
	} else {
		imageURL = firstImage
	}

	html := pageTemplate(cfg, title, description, pageURL, siteURL, imageURL, headerHTML, bodyHTML, footerHTML)
	_, err = io.WriteString(w, html)
	return err
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

func writeFeed(distDir, siteURL string, cfg markdownutil.SiteConfig, pages []pageInfo) error {
	var sb strings.Builder
	sb.WriteString(`<?xml version="1.0" encoding="UTF-8"?>` + "\n")
	sb.WriteString(`<rss version="2.0">` + "\n")
	sb.WriteString("<channel>\n")
	sb.WriteString(fmt.Sprintf("  <title>%s</title>\n", escapeHTML(cfg.Name)))
	sb.WriteString(fmt.Sprintf("  <link>%s</link>\n", escapeHTML(siteURL+"/")))
	sb.WriteString(fmt.Sprintf("  <description>%s</description>\n", escapeHTML(cfg.Description)))

	for _, p := range pages {
		if !p.isPost {
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

func writeLLMs(distDir string, cfg markdownutil.SiteConfig, pages []pageInfo) error {
	var sb strings.Builder
	sb.WriteString("# " + cfg.Name + "\n")
	sb.WriteString("> " + cfg.Description + "\n")
	sb.WriteString("## 페이지\n")

	for _, p := range pages {
		if p.is404 {
			continue
		}
		if p.description != "" {
			sb.WriteString(fmt.Sprintf("- [%s](%s): %s\n", p.title, p.url, p.description))
		} else {
			sb.WriteString(fmt.Sprintf("- [%s](%s)\n", p.title, p.url))
		}
	}

	return os.WriteFile(filepath.Join(distDir, "llms.txt"), []byte(sb.String()), 0644)
}
