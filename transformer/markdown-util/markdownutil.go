package markdownutil

import (
	"bytes"

	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/text"
)

// collectText recursively walks all children of a node and collects plain text.
// It handles ast.Text, ast.CodeSpan, and any other inline node by walking children.
func collectText(n ast.Node, source []byte) string {
	var buf bytes.Buffer
	collectTextInto(n, source, &buf)
	return buf.String()
}

func collectTextInto(n ast.Node, source []byte, buf *bytes.Buffer) {
	switch node := n.(type) {
	case *ast.Text:
		buf.Write(node.Segment.Value(source))
		if node.SoftLineBreak() || node.HardLineBreak() {
			buf.WriteByte('\n')
		}
	case *ast.CodeSpan:
		// CodeSpan children are Text nodes containing the raw code content
		for child := node.FirstChild(); child != nil; child = child.NextSibling() {
			collectTextInto(child, source, buf)
		}
	default:
		// For any other node (Emphasis, Strong, Link, Image, etc.), recurse into children
		for child := n.FirstChild(); child != nil; child = child.NextSibling() {
			collectTextInto(child, source, buf)
		}
	}
}

// ExtractH1 extracts the plain text content of the first H1 heading in the markdown source.
// Inline formatting (bold, italic, code spans, links) is reduced to plain text.
// Returns empty string if no H1 is found.
func ExtractH1(source []byte) string {
	reader := text.NewReader(source)
	parser := goldmark.DefaultParser()
	doc := parser.Parse(reader)

	var result string
	ast.Walk(doc, func(n ast.Node, entering bool) (ast.WalkStatus, error) {
		if !entering {
			return ast.WalkContinue, nil
		}
		heading, ok := n.(*ast.Heading)
		if !ok {
			return ast.WalkContinue, nil
		}
		if heading.Level == 1 {
			result = collectText(heading, source)
			return ast.WalkStop, nil
		}
		return ast.WalkContinue, nil
	})

	return result
}

// ExtractFirstParagraph extracts the plain text of the first paragraph in the markdown source.
// Inline formatting is reduced to plain text by recursively collecting text from all inline children.
// Returns empty string if no paragraph is found.
func ExtractFirstParagraph(source []byte) string {
	reader := text.NewReader(source)
	parser := goldmark.DefaultParser()
	doc := parser.Parse(reader)

	var result string
	ast.Walk(doc, func(n ast.Node, entering bool) (ast.WalkStatus, error) {
		if !entering {
			return ast.WalkContinue, nil
		}
		if _, ok := n.(*ast.Paragraph); ok {
			result = collectText(n, source)
			return ast.WalkStop, nil
		}
		return ast.WalkContinue, nil
	})

	return result
}

// MarkdownToHTML converts a markdown source to an HTML string.
func MarkdownToHTML(source []byte) (string, error) {
	var buf bytes.Buffer
	md := goldmark.New()
	if err := md.Convert(source, &buf); err != nil {
		return "", err
	}
	return buf.String(), nil
}
