// codeblock.go — fast handling of fenced code blocks whose language has no
// chroma lexer (e.g. ```mermaid, ```dot, custom DSLs).
//
// Why this exists: the goldmark-highlighting renderer calls the global
// chroma lexers.Get(lang) for EVERY fenced code block. On a cache-miss chroma
// falls back to globbing the language string as a filename (filepath.Match)
// against all ~250 registered lexers, then runs regex content analysis — and
// it does this on every block with no negative-lookup caching. For documents
// with many unknown-language fences (mermaid being the common one) this
// dominates runtime: profiling showed ~75% of total CPU spent in this path,
// even though for an unknown language the highlighter ultimately emits a plain
// <pre><code class="language-X"> block anyway (see highlighting.go:535-552).
//
// This file adds a goldmark extension that runs BEFORE rendering: an AST
// transformer rewrites every fenced block whose language chroma can't resolve
// into a passthroughCodeBlock node, which a dedicated renderer emits as the
// exact same plain markup the highlighter would have produced — so output is
// byte-identical, but chroma never sees these blocks. A process-lifetime cache
// (sync.Map) means each distinct unknown language pays the chroma lookup at
// most once, not once per block.
//
// Related: converter.go wires this extension in alongside the highlighting
// extension; scripts/mermaid.js consumes the `code.language-mermaid` output.
package internal

import (
	"sync"

	"github.com/alecthomas/chroma/v2/lexers"
	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/parser"
	"github.com/yuin/goldmark/renderer"
	goldmarkhtml "github.com/yuin/goldmark/renderer/html"
	"github.com/yuin/goldmark/text"
	"github.com/yuin/goldmark/util"
)

// kindPassthroughCodeBlock identifies our custom node. Registered once at
// package init — NewNodeKind must not be called per-node.
var kindPassthroughCodeBlock = ast.NewNodeKind("PassthroughCodeBlock")

// passthroughCodeBlock is a fenced code block we render verbatim (escaped),
// without routing it through chroma. It carries the info-string language so the
// renderer can emit the `language-<lang>` class that mermaid.js and CSS expect.
type passthroughCodeBlock struct {
	ast.BaseBlock
	language []byte
}

func (n *passthroughCodeBlock) Kind() ast.NodeKind { return kindPassthroughCodeBlock }

func (n *passthroughCodeBlock) Dump(source []byte, level int) {
	ast.DumpHelper(n, source, level, nil, nil)
}

// passthroughExtension bundles the transformer (rewrite) and renderer (emit).
// The lexerKnown cache lives here so it persists across every Convert call made
// on the goldmark instance — "mermaid" is resolved against chroma once for the
// whole process, then served from cache forever.
type passthroughExtension struct {
	lexerKnown sync.Map // language string -> bool (chroma has a real lexer)
}

func newPassthroughExtension() *passthroughExtension {
	return &passthroughExtension{}
}

// known reports whether chroma can resolve lang to a real lexer, caching the
// (expensive) first lookup. This is the only place the slow chroma path runs,
// and only once per distinct language.
func (e *passthroughExtension) known(lang string) bool {
	if v, ok := e.lexerKnown.Load(lang); ok {
		return v.(bool)
	}
	v := lexers.Get(lang) != nil
	e.lexerKnown.Store(lang, v)
	return v
}

func (e *passthroughExtension) Extend(m goldmark.Markdown) {
	m.Parser().AddOptions(parser.WithASTTransformers(
		util.Prioritized(e, 100),
	))
	m.Renderer().AddOptions(renderer.WithNodeRenderers(
		util.Prioritized(e, 100),
	))
}

// Transform rewrites unknown-language fenced blocks into passthrough nodes.
// Blocks are collected during the walk and replaced afterwards to avoid
// mutating the tree mid-traversal. Blocks with no language, or a language
// chroma knows, are left untouched so the highlighter handles them as before.
func (e *passthroughExtension) Transform(doc *ast.Document, reader text.Reader, _ parser.Context) {
	source := reader.Source()
	var targets []*ast.FencedCodeBlock
	_ = ast.Walk(doc, func(n ast.Node, entering bool) (ast.WalkStatus, error) {
		if !entering {
			return ast.WalkContinue, nil
		}
		fc, ok := n.(*ast.FencedCodeBlock)
		if !ok {
			return ast.WalkContinue, nil
		}
		lang := fc.Language(source)
		if len(lang) == 0 || e.known(string(lang)) {
			return ast.WalkContinue, nil
		}
		targets = append(targets, fc)
		return ast.WalkContinue, nil
	})

	for _, fc := range targets {
		pb := &passthroughCodeBlock{language: fc.Language(source)}
		pb.SetLines(fc.Lines())
		parent := fc.Parent()
		if parent != nil {
			parent.ReplaceChild(parent, fc, pb)
		}
	}
}

func (e *passthroughExtension) RegisterFuncs(reg renderer.NodeRendererFuncRegisterer) {
	reg.Register(kindPassthroughCodeBlock, e.render)
}

// render emits the exact markup the goldmark-highlighting fallback would
// (highlighting.go:535-552): <pre><code class="language-LANG">escaped</code></pre>.
// util.DefaultWriter.Write/RawWrite match the escaping the library uses, so
// switching a block onto this path leaves output byte-identical.
func (e *passthroughExtension) render(w util.BufWriter, source []byte, n ast.Node, entering bool) (ast.WalkStatus, error) {
	pb := n.(*passthroughCodeBlock)
	if entering {
		_, _ = w.WriteString("<pre><code")
		if len(pb.language) > 0 {
			_, _ = w.WriteString(` class="language-`)
			goldmarkhtml.DefaultWriter.Write(w, pb.language)
			_ = w.WriteByte('"')
		}
		_ = w.WriteByte('>')
		lines := pb.Lines()
		for i := 0; i < lines.Len(); i++ {
			seg := lines.At(i)
			goldmarkhtml.DefaultWriter.RawWrite(w, seg.Value(source))
		}
	} else {
		_, _ = w.WriteString("</code></pre>\n")
	}
	return ast.WalkContinue, nil
}
