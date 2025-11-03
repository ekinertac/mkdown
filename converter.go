package main

import (
	"bytes"
	_ "embed"
	"html/template"
	"os"
	"strings"

	"github.com/alecthomas/chroma/v2/formatters/html"
	"github.com/yuin/goldmark"
	highlighting "github.com/yuin/goldmark-highlighting/v2"
	"github.com/yuin/goldmark/extension"
	"github.com/yuin/goldmark/parser"
	goldmarkhtml "github.com/yuin/goldmark/renderer/html"
	"gopkg.in/yaml.v3"
)

//go:embed templates/default.html
var defaultTemplate string

//go:embed templates/dark.css
var darkThemeCSS string

//go:embed templates/light.css
var lightThemeCSS string

type Converter struct {
	markdown goldmark.Markdown
	template *template.Template
	theme    string
}

type Document struct {
	Title    string
	Content  template.HTML
	Styles   template.CSS
	Metadata map[string]interface{}
}

func NewConverter(theme string) *Converter {
	md := goldmark.New(
		goldmark.WithExtensions(
			extension.GFM, // GitHub Flavored Markdown (includes tables, strikethrough, task lists)
			highlighting.NewHighlighting(
				highlighting.WithStyle("monokai"),
				highlighting.WithFormatOptions(
					html.WithClasses(true),
					html.WithLineNumbers(false),
				),
			),
		),
		goldmark.WithParserOptions(
			parser.WithAutoHeadingID(),
		),
		goldmark.WithRendererOptions(
			goldmarkhtml.WithHardWraps(),
			goldmarkhtml.WithXHTML(),
		),
	)

	tmpl := template.Must(template.New("default").Parse(defaultTemplate))

	return &Converter{
		markdown: md,
		template: tmpl,
		theme:    theme,
	}
}

func (c *Converter) Convert(inputPath, outputPath string) error {
	// Read input file
	source, err := os.ReadFile(inputPath)
	if err != nil {
		return err
	}

	// Parse frontmatter
	doc, markdownContent := c.parseFrontmatter(source)

	// Convert markdown to HTML
	var buf bytes.Buffer
	if err := c.markdown.Convert(markdownContent, &buf); err != nil {
		return err
	}

	doc.Content = template.HTML(buf.String())

	// Render template
	var output bytes.Buffer
	if err := c.template.Execute(&output, doc); err != nil {
		return err
	}

	// Write output file
	return os.WriteFile(outputPath, output.Bytes(), 0644)
}

func (c *Converter) parseFrontmatter(source []byte) (*Document, []byte) {
	// Select theme CSS
	themeCSS := darkThemeCSS
	if c.theme == "light" {
		themeCSS = lightThemeCSS
	}

	doc := &Document{
		Title:    "Document",
		Styles:   template.CSS(themeCSS),
		Metadata: make(map[string]interface{}),
	}

	content := source
	str := string(source)

	// Check for YAML frontmatter (--- at start)
	if strings.HasPrefix(str, "---\n") || strings.HasPrefix(str, "---\r\n") {
		parts := strings.SplitN(str, "\n", 2)
		if len(parts) == 2 {
			rest := parts[1]
			endIdx := strings.Index(rest, "\n---\n")
			if endIdx == -1 {
				endIdx = strings.Index(rest, "\n---\r\n")
			}
			if endIdx == -1 {
				endIdx = strings.Index(rest, "\r\n---\r\n")
			}

			if endIdx != -1 {
				frontmatter := rest[:endIdx]
				content = []byte(rest[endIdx+5:]) // Skip past "---\n"

				// Parse YAML
				var metadata map[string]interface{}
				if err := yaml.Unmarshal([]byte(frontmatter), &metadata); err == nil {
					doc.Metadata = metadata
					if title, ok := metadata["title"].(string); ok {
						doc.Title = title
					}
				}
			}
		}
	}

	return doc, content
}

