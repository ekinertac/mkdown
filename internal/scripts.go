package internal

import _ "embed"

//go:embed scripts/mermaid.js
var mermaidScript string

//go:embed scripts/katex.js
var katexScript string

// GetMermaidScript returns the Mermaid initialization script
func GetMermaidScript() string {
	return mermaidScript
}

// GetKatexScript returns the KaTeX initialization script
func GetKatexScript() string {
	return katexScript
}

