package internal

import (
	"html/template"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestNewConverter(t *testing.T) {
	tests := []struct {
		name  string
		theme string
	}{
		{"dark theme", "dark"},
		{"light theme", "light"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := NewConverter(tt.theme)
			if c == nil {
				t.Fatal("NewConverter returned nil")
			}
			if c.theme != tt.theme {
				t.Errorf("expected theme %s, got %s", tt.theme, c.theme)
			}
			if c.markdown == nil {
				t.Error("markdown parser is nil")
			}
			if c.template == nil {
				t.Error("template is nil")
			}
		})
	}
}

func TestConvertBasicMarkdown(t *testing.T) {
	tmpDir := t.TempDir()
	inputPath := filepath.Join(tmpDir, "test.md")
	outputPath := filepath.Join(tmpDir, "test.html")

	// Write test markdown
	content := `# Test Heading

This is a paragraph with **bold** and *italic* text.

- List item 1
- List item 2
`
	if err := os.WriteFile(inputPath, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	// Convert
	c := NewConverter("dark")
	if err := c.Convert(inputPath, outputPath); err != nil {
		t.Fatalf("Convert failed: %v", err)
	}

	// Read output
	output, err := os.ReadFile(outputPath)
	if err != nil {
		t.Fatal(err)
	}

	outputStr := string(output)

	// Verify HTML structure
	checks := []string{
		"<!DOCTYPE html>",
		"<html",
		"<head>",
		"<body>",
		"<h1",
		"Test Heading",
		"<strong>bold</strong>",
		"<em>italic</em>",
		"<ul>",
		"<li>List item 1</li>",
	}

	for _, check := range checks {
		if !strings.Contains(outputStr, check) {
			t.Errorf("output missing expected content: %s", check)
		}
	}
}

func TestConvertWithFrontmatter(t *testing.T) {
	tmpDir := t.TempDir()
	inputPath := filepath.Join(tmpDir, "test.md")
	outputPath := filepath.Join(tmpDir, "test.html")

	content := `---
title: Test Document
author: Test Author
---

# Content`

	if err := os.WriteFile(inputPath, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	c := NewConverter("dark")
	if err := c.Convert(inputPath, outputPath); err != nil {
		t.Fatalf("Convert failed: %v", err)
	}

	output, err := os.ReadFile(outputPath)
	if err != nil {
		t.Fatal(err)
	}

	outputStr := string(output)

	// Check title is in HTML
	if !strings.Contains(outputStr, "<title>Test Document</title>") {
		t.Error("title from frontmatter not found in output")
	}
}

func TestExtensionsFootnotes(t *testing.T) {
	tmpDir := t.TempDir()
	inputPath := filepath.Join(tmpDir, "test.md")
	outputPath := filepath.Join(tmpDir, "test.html")

	content := `Text with footnote[^1].

[^1]: Footnote content.`

	if err := os.WriteFile(inputPath, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	c := NewConverter("dark")
	if err := c.Convert(inputPath, outputPath); err != nil {
		t.Fatalf("Convert failed: %v", err)
	}

	output, err := os.ReadFile(outputPath)
	if err != nil {
		t.Fatal(err)
	}

	outputStr := string(output)

	// Check footnote elements
	checks := []string{
		`class="footnote-ref"`,
		`class="footnotes"`,
		"Footnote content",
	}

	for _, check := range checks {
		if !strings.Contains(outputStr, check) {
			t.Errorf("footnote output missing: %s", check)
		}
	}
}

func TestExtensionsDefinitionList(t *testing.T) {
	tmpDir := t.TempDir()
	inputPath := filepath.Join(tmpDir, "test.md")
	outputPath := filepath.Join(tmpDir, "test.html")

	content := `Term
: Definition`

	if err := os.WriteFile(inputPath, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	c := NewConverter("dark")
	if err := c.Convert(inputPath, outputPath); err != nil {
		t.Fatalf("Convert failed: %v", err)
	}

	output, err := os.ReadFile(outputPath)
	if err != nil {
		t.Fatal(err)
	}

	outputStr := string(output)

	// Check definition list elements
	checks := []string{
		"<dl>",
		"<dt>Term</dt>",
		"<dd>Definition</dd>",
	}

	for _, check := range checks {
		if !strings.Contains(outputStr, check) {
			t.Errorf("definition list output missing: %s", check)
		}
	}
}

func TestExtensionsTypographer(t *testing.T) {
	tmpDir := t.TempDir()
	inputPath := filepath.Join(tmpDir, "test.md")
	outputPath := filepath.Join(tmpDir, "test.html")

	content := `"Quotes" and --- dashes and...`

	if err := os.WriteFile(inputPath, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	c := NewConverter("dark")
	if err := c.Convert(inputPath, outputPath); err != nil {
		t.Fatalf("Convert failed: %v", err)
	}

	output, err := os.ReadFile(outputPath)
	if err != nil {
		t.Fatal(err)
	}

	outputStr := string(output)

	// Check for smart typography
	checks := []string{
		"&ldquo;", // left quote
		"&rdquo;", // right quote
		"&mdash;", // em dash
		"&hellip;", // ellipsis
	}

	for _, check := range checks {
		if !strings.Contains(outputStr, check) {
			t.Errorf("typographer output missing: %s", check)
		}
	}
}

func TestExtensionsLinkify(t *testing.T) {
	tmpDir := t.TempDir()
	inputPath := filepath.Join(tmpDir, "test.md")
	outputPath := filepath.Join(tmpDir, "test.html")

	content := `Visit https://example.com for more.`

	if err := os.WriteFile(inputPath, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	c := NewConverter("dark")
	if err := c.Convert(inputPath, outputPath); err != nil {
		t.Fatalf("Convert failed: %v", err)
	}

	output, err := os.ReadFile(outputPath)
	if err != nil {
		t.Fatal(err)
	}

	outputStr := string(output)

	// Check URL was linkified
	if !strings.Contains(outputStr, `<a href="https://example.com"`) {
		t.Error("URL was not linkified")
	}
}

func TestThemeSelection(t *testing.T) {
	tmpDir := t.TempDir()
	inputPath := filepath.Join(tmpDir, "test.md")
	
	content := `# Test`
	if err := os.WriteFile(inputPath, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		theme         string
		expectedColor string
	}{
		{"dark", "#0d1117"},  // dark background
		{"light", "#ffffff"}, // light background
	}

	for _, tt := range tests {
		t.Run(tt.theme, func(t *testing.T) {
			outputPath := filepath.Join(tmpDir, tt.theme+".html")
			
			c := NewConverter(tt.theme)
			if err := c.Convert(inputPath, outputPath); err != nil {
				t.Fatalf("Convert failed: %v", err)
			}

			output, err := os.ReadFile(outputPath)
			if err != nil {
				t.Fatal(err)
			}

			if !strings.Contains(string(output), tt.expectedColor) {
				t.Errorf("expected %s theme color %s not found", tt.theme, tt.expectedColor)
			}
		})
	}
}

func TestParseFrontmatter(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		wantTitle string
	}{
		{
			name: "with frontmatter",
			input: `---
title: Custom Title
---
Content`,
			wantTitle: "Custom Title",
		},
		{
			name:     "without frontmatter",
			input:    "# Just Content",
			wantTitle: "Document",
		},
		{
			name: "invalid frontmatter",
			input: `---
broken yaml: [
---
Content`,
			wantTitle: "Document",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := NewConverter("dark")
			doc, _ := c.parseFrontmatter([]byte(tt.input))
			
			if doc.Title != tt.wantTitle {
				t.Errorf("expected title %q, got %q", tt.wantTitle, doc.Title)
			}

			if doc.Styles == template.CSS("") {
				t.Error("styles should not be empty")
			}
		})
	}
}

func TestSyntaxHighlighting(t *testing.T) {
	tmpDir := t.TempDir()
	inputPath := filepath.Join(tmpDir, "test.md")
	outputPath := filepath.Join(tmpDir, "test.html")

	content := "```go\nfmt.Println(\"hello\")\n```"

	if err := os.WriteFile(inputPath, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	c := NewConverter("dark")
	if err := c.Convert(inputPath, outputPath); err != nil {
		t.Fatalf("Convert failed: %v", err)
	}

	output, err := os.ReadFile(outputPath)
	if err != nil {
		t.Fatal(err)
	}

	outputStr := string(output)

	// Check for chroma syntax highlighting
	if !strings.Contains(outputStr, `class="chroma"`) {
		t.Error("syntax highlighting not applied")
	}
}

func TestTableExtension(t *testing.T) {
	tmpDir := t.TempDir()
	inputPath := filepath.Join(tmpDir, "test.md")
	outputPath := filepath.Join(tmpDir, "test.html")

	content := `| Header 1 | Header 2 |
|----------|----------|
| Cell 1   | Cell 2   |`

	if err := os.WriteFile(inputPath, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	c := NewConverter("dark")
	if err := c.Convert(inputPath, outputPath); err != nil {
		t.Fatalf("Convert failed: %v", err)
	}

	output, err := os.ReadFile(outputPath)
	if err != nil {
		t.Fatal(err)
	}

	outputStr := string(output)

	checks := []string{
		"<table>",
		"<thead>",
		"<th>Header 1</th>",
		"<td>Cell 1</td>",
	}

	for _, check := range checks {
		if !strings.Contains(outputStr, check) {
			t.Errorf("table output missing: %s", check)
		}
	}
}

