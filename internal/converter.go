package internal

import (
	"bytes"
	_ "embed"
	"fmt"
	"html/template"
	"os"
	"path/filepath"
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
	markdown     goldmark.Markdown
	template     *template.Template
	theme        string
	enableMermaid bool
	enableMath    bool
}

type Document struct {
	Title    string
	Content  template.HTML
	Styles   template.CSS
	Scripts  template.HTML
	Metadata map[string]interface{}
}

type ConverterOptions struct {
	Theme         string
	EnableMermaid bool
	EnableMath    bool
}

func NewConverter(theme string) *Converter {
	return NewConverterWithOptions(ConverterOptions{
		Theme:         theme,
		EnableMermaid: false,
		EnableMath:    false,
	})
}

func NewConverterWithOptions(opts ConverterOptions) *Converter {
	md := goldmark.New(
		goldmark.WithExtensions(
			extension.GFM, // GitHub Flavored Markdown (includes tables, strikethrough, task lists)
			extension.Footnote,
			extension.DefinitionList,
			extension.Typographer,
			extension.Linkify,
			highlighting.NewHighlighting(
				highlighting.WithStyle("monokai"),
				highlighting.WithFormatOptions(
					html.WithClasses(true),
					html.WithLineNumbers(false),
				),
			),
			// Bypass chroma for fences it has no lexer for (e.g. mermaid).
			// See codeblock.go — avoids chroma's per-block filename-glob
			// fallback that otherwise dominates runtime on such documents.
			newPassthroughExtension(),
		),
		goldmark.WithParserOptions(
			parser.WithAutoHeadingID(),
		),
		goldmark.WithRendererOptions(
			goldmarkhtml.WithXHTML(),
			goldmarkhtml.WithUnsafe(), // Allow raw HTML to prevent math breaking
		),
	)

	tmpl := template.Must(template.New("default").Parse(defaultTemplate))

	return &Converter{
		markdown:      md,
		template:      tmpl,
		theme:         opts.Theme,
		enableMermaid: opts.EnableMermaid,
		enableMath:    opts.EnableMath,
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

	// Protect math blocks if math is enabled
	if c.enableMath {
		markdownContent = c.protectMathBlocks(markdownContent)
	}

	// Convert markdown to HTML
	var buf bytes.Buffer
	if err := c.markdown.Convert(markdownContent, &buf); err != nil {
		return err
	}

	htmlContent := buf.String()

	// Restore math blocks
	if c.enableMath {
		htmlContent = c.restoreMathBlocks(htmlContent)
	}

	doc.Content = template.HTML(htmlContent)

	// Inject scripts if needed
	c.injectScripts(doc, markdownContent)

	// Render template
	var output bytes.Buffer
	if err := c.template.Execute(&output, doc); err != nil {
		return err
	}

	// Create output directory if it doesn't exist
	outputDir := filepath.Dir(outputPath)
	if outputDir != "" && outputDir != "." {
		if err := os.MkdirAll(outputDir, 0755); err != nil {
			return fmt.Errorf("failed to create output directory: %w", err)
		}
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

	// Check for YAML frontmatter (--- at start). Only stringify the source
	// when a frontmatter marker is actually present — otherwise this copies
	// the entire (potentially multi-MB) document for nothing.
	if bytes.HasPrefix(source, []byte("---\n")) || bytes.HasPrefix(source, []byte("---\r\n")) {
		str := string(source)
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

var mathBlockPlaceholder = "<!--MATH_BLOCK_%d-->"
var mathBlocks = make(map[int]string)
var mathBlockCounter = 0

func (c *Converter) protectMathBlocks(markdown []byte) []byte {
	content := string(markdown)
	
	// Reset for each conversion
	mathBlocks = make(map[int]string)
	mathBlockCounter = 0
	
	// Find and replace $$ blocks
	parts := strings.Split(content, "$$")
	if len(parts) < 3 {
		return markdown // No $$ blocks found
	}
	
	var result []string
	for i := 0; i < len(parts); i++ {
		if i%2 == 0 {
			// Outside math block
			result = append(result, parts[i])
		} else {
			// Inside math block
			placeholder := fmt.Sprintf(mathBlockPlaceholder, mathBlockCounter)
			mathBlocks[mathBlockCounter] = parts[i]
			mathBlockCounter++
			result = append(result, placeholder)
		}
	}
	
	return []byte(strings.Join(result, ""))
}

func (c *Converter) restoreMathBlocks(html string) string {
	for id, content := range mathBlocks {
		placeholder := fmt.Sprintf(mathBlockPlaceholder, id)
		// Wrap in proper math delimiters
		mathHTML := fmt.Sprintf("<div class=\"math-block\">$$\n%s\n$$</div>", content)
		html = strings.Replace(html, "&lt;!--MATH_BLOCK_"+fmt.Sprintf("%d", id)+"--&gt;", mathHTML, 1)
		html = strings.Replace(html, placeholder, mathHTML, 1)
	}
	return html
}

func (c *Converter) injectScripts(doc *Document, markdown []byte) {
	var scripts []string

	// Scan the raw bytes directly. The feature guards short-circuit, so in the
	// default path (no mermaid/math) we touch the document not at all, instead
	// of copying it to a string up front.
	if c.enableMermaid && bytes.Contains(markdown, []byte("```mermaid")) {
		mermaidTheme := "dark"
		if c.theme == "light" {
			mermaidTheme = "default"
		}
		script := strings.Replace(GetMermaidScript(), "{{THEME}}", mermaidTheme, 1)
		scripts = append(scripts, script)
	}

	// Check for Math expressions
	if c.enableMath && (bytes.Contains(markdown, []byte("$$")) || bytes.Contains(markdown, []byte("$"))) {
		scripts = append(scripts, GetKatexScript())
	}

	if len(scripts) > 0 {
		doc.Scripts = template.HTML(strings.Join(scripts, "\n"))
	}
}

