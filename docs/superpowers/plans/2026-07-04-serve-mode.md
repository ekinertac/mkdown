# `mkdown serve` (Live Preview) Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.
>
> **Drift check (run first)**: `git diff --stat 0c9cab6..HEAD -- internal/converter.go cmd/mkdown/main.go` — if either file changed since this plan was written, compare the "Current state" excerpts against the live code before proceeding; on a mismatch, STOP.

**Goal:** Add a `mkdown serve readme.md` subcommand that serves a single markdown file's rendered HTML on a localhost port, opens the browser, and auto-refreshes it whenever the file changes.

**Architecture:** A new `internal.Serve` starts an `http.Server` on `127.0.0.1:0` (OS-assigned free port), serves the rendered HTML (with a small injected polling script) at `/` and a version counter at `/__mtime`, and runs a poll goroutine — reusing the existing `statSig` change-detection from `internal/watch.go` — that re-renders in memory and bumps the version on each file change. The browser's injected script polls `/__mtime` every 300ms and calls `location.reload()` when it changes. A new `Converter.Render(in) ([]byte, error)` provides the in-memory HTML; `Convert` keeps streaming to disk. `main` gets a `serve` subcommand and a cross-platform `openBrowser` helper.

**Tech Stack:** Go standard library only (`net/http`, `net`, `context`, `os/signal`, `os/exec`, `sync`, `html`). No new dependencies. Reuses `internal.Converter` and `statSig`.

**Spec:** `docs/superpowers/specs/2026-07-04-serve-mode-design.md`

**Planned at:** commit `0c9cab6`, 2026-07-04

---

## Commands you will need

| Purpose | Command | Expected on success |
|---|---|---|
| Build | `go build ./...` | exit 0, no output |
| Vet | `go vet ./...` | exit 0, no output |
| Tests | `go test ./...` | all packages `ok` |
| Race | `go test ./internal -race` | `ok`, no `DATA RACE` |
| Single test | `go test ./internal -run TestName -v` | as noted per step |

Conventions to follow: unexported helpers for internal-only functions; file-level comment block at the top of every new file explaining its role (see `internal/watch.go` for the house style); errors to `os.Stderr` + `os.Exit(1)` in `main`; `t.TempDir()` + real files in tests (see `internal/watch_test.go`).

---

## Task 1: Add `Converter.Render`, keep `Convert` streaming

**Files:**
- Modify: `internal/converter.go` (the `Convert` method, lines 118–187)
- Test: `internal/converter_test.go` (append one test)

### Current state

`internal/converter.go:118-187` — `Convert` reads the file, builds a `*Document`, then **streams** the templated output to the file via a buffered writer:

```go
func (c *Converter) Convert(inputPath, outputPath string) error {
	source, err := os.ReadFile(inputPath)
	if err != nil {
		return err
	}
	doc, markdownContent := c.parseFrontmatter(source)
	var mathBlocks map[int]string
	if c.enableMath {
		markdownContent, mathBlocks = c.protectMathBlocks(markdownContent)
	}
	buf := bufPool.Get().(*bytes.Buffer)
	buf.Reset()
	pc := parser.NewContext(parser.WithIDs(newFastIDs()))
	if err := c.markdown.Convert(markdownContent, buf, parser.WithContext(pc)); err != nil {
		bufPool.Put(buf)
		return err
	}
	htmlContent := buf.String()
	bufPool.Put(buf)
	if c.enableMath {
		htmlContent = c.restoreMathBlocks(htmlContent, mathBlocks)
	}
	doc.Content = template.HTML(htmlContent)
	c.injectScripts(doc, markdownContent)
	outputDir := filepath.Dir(outputPath)
	if outputDir != "" && outputDir != "." {
		if err := os.MkdirAll(outputDir, 0755); err != nil {
			return fmt.Errorf("failed to create output directory: %w", err)
		}
	}
	f, err := os.Create(outputPath)
	if err != nil {
		return err
	}
	bw := bufio.NewWriterSize(f, 32*1024)
	if err := c.template.Execute(bw, doc); err != nil {
		f.Close()
		return err
	}
	if err := bw.Flush(); err != nil {
		f.Close()
		return err
	}
	return f.Close()
}
```

The document-building work (read → frontmatter → math → markdown convert → math restore → inject scripts) is duplicated exactly by what `Render` needs. Extract it into a private helper so both share it and `Convert` keeps streaming.

- [ ] **Step 1: Write the failing test**

Append to `internal/converter_test.go`:

```go
func TestRender(t *testing.T) {
	dir := t.TempDir()
	in := filepath.Join(dir, "doc.md")
	if err := os.WriteFile(in, []byte("# Hello\n\nWorld."), 0644); err != nil {
		t.Fatal(err)
	}
	c := NewConverter("dark")
	out, err := c.Render(in)
	if err != nil {
		t.Fatalf("Render returned error: %v", err)
	}
	s := string(out)
	if !strings.Contains(s, "<!DOCTYPE html>") {
		t.Errorf("Render output is not a full document: %s", s[:min(200, len(s))])
	}
	if !strings.Contains(s, "Hello") || !strings.Contains(s, "World") {
		t.Errorf("Render output missing content: %s", s)
	}
	// Render must not write any file.
	if _, err := os.Stat(filepath.Join(dir, "doc.html")); !os.IsNotExist(err) {
		t.Error("Render should not write an output file")
	}
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
```

Check the top of `internal/converter_test.go` — if `"strings"`, `"os"`, or `"path/filepath"` are not already imported, add them. (They are used elsewhere in that file; confirm before assuming.) If a `min` helper already exists in the package/test file, do NOT redeclare it — drop the local `min` and inline `len(s)`.

- [ ] **Step 2: Run the test to verify it fails**

Run: `go test ./internal -run TestRender -v`
Expected: FAIL — `c.Render undefined`.

- [ ] **Step 3: Refactor `Convert` and add `Render`**

Replace the entire `Convert` method (lines 118–187) with the following three functions. `buildDocument` holds the shared work; `Convert` streams to the file as before; `Render` executes the template into a buffer.

```go
// buildDocument reads the input markdown file and returns the fully populated
// template Document (frontmatter parsed, math protected/restored, scripts
// injected). Shared by Convert (streams to file) and Render (returns bytes).
func (c *Converter) buildDocument(inputPath string) (*Document, error) {
	source, err := os.ReadFile(inputPath)
	if err != nil {
		return nil, err
	}

	doc, markdownContent := c.parseFrontmatter(source)

	var mathBlocks map[int]string
	if c.enableMath {
		markdownContent, mathBlocks = c.protectMathBlocks(markdownContent)
	}

	buf := bufPool.Get().(*bytes.Buffer)
	buf.Reset()
	pc := parser.NewContext(parser.WithIDs(newFastIDs()))
	if err := c.markdown.Convert(markdownContent, buf, parser.WithContext(pc)); err != nil {
		bufPool.Put(buf)
		return nil, err
	}
	htmlContent := buf.String()
	bufPool.Put(buf)

	if c.enableMath {
		htmlContent = c.restoreMathBlocks(htmlContent, mathBlocks)
	}

	doc.Content = template.HTML(htmlContent)
	c.injectScripts(doc, markdownContent)
	return doc, nil
}

// Render reads the input markdown file and returns the full standalone HTML
// document as bytes, without writing anything to disk.
func (c *Converter) Render(inputPath string) ([]byte, error) {
	doc, err := c.buildDocument(inputPath)
	if err != nil {
		return nil, err
	}
	var out bytes.Buffer
	if err := c.template.Execute(&out, doc); err != nil {
		return nil, err
	}
	return out.Bytes(), nil
}

func (c *Converter) Convert(inputPath, outputPath string) error {
	doc, err := c.buildDocument(inputPath)
	if err != nil {
		return err
	}

	// Create output directory if it doesn't exist
	outputDir := filepath.Dir(outputPath)
	if outputDir != "" && outputDir != "." {
		if err := os.MkdirAll(outputDir, 0755); err != nil {
			return fmt.Errorf("failed to create output directory: %w", err)
		}
	}

	// Stream the templated HTML straight to the file through a buffered writer,
	// avoiding a second in-memory copy of the whole rendered document.
	f, err := os.Create(outputPath)
	if err != nil {
		return err
	}
	bw := bufio.NewWriterSize(f, 32*1024)
	if err := c.template.Execute(bw, doc); err != nil {
		f.Close()
		return err
	}
	if err := bw.Flush(); err != nil {
		f.Close()
		return err
	}
	return f.Close()
}
```

Do NOT change any other method, the imports, or `bufPool`. The behavior of `Convert` is unchanged.

- [ ] **Step 4: Run the tests**

Run: `go test ./internal -run 'TestRender|TestConvert' -v`
Expected: PASS, including all pre-existing `Convert` tests.

Run: `go test ./...`
Expected: all packages `ok`.

- [ ] **Step 5: Commit**

```bash
git add internal/converter.go internal/converter_test.go
git commit -m "Add Converter.Render; share document build with Convert"
```

---

## Task 2: Implement `internal.Serve`

**Files:**
- Create: `internal/serve.go`
- Test: `internal/serve_test.go`

Reuses `statSig(path string) (time.Time, int64)` from `internal/watch.go:61-69` (same package — call it directly, do not redefine it).

- [ ] **Step 1: Write the failing test**

Create `internal/serve_test.go`:

```go
package internal

import (
	"context"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func httpGetBody(t *testing.T, url string) string {
	t.Helper()
	resp, err := http.Get(url)
	if err != nil {
		t.Fatalf("GET %s: %v", url, err)
	}
	defer resp.Body.Close()
	b, _ := io.ReadAll(resp.Body)
	return strings.TrimSpace(string(b))
}

func waitForMtimeChange(t *testing.T, base, old string) {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if httpGetBody(t, base+"/__mtime") != old {
			return
		}
		time.Sleep(5 * time.Millisecond)
	}
	t.Fatalf("/__mtime never changed from %q", old)
}

func TestServeRerendersOnChange(t *testing.T) {
	dir := t.TempDir()
	in := filepath.Join(dir, "doc.md")
	if err := os.WriteFile(in, []byte("# First"), 0644); err != nil {
		t.Fatal(err)
	}

	c := NewConverter("dark")
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	urlCh := make(chan string, 1)
	done := make(chan error, 1)
	go func() {
		done <- Serve(ctx, c, in, 10*time.Millisecond, func(u string) { urlCh <- u })
	}()

	var base string
	select {
	case base = <-urlCh:
	case <-time.After(2 * time.Second):
		t.Fatal("server never became ready")
	}

	body := httpGetBody(t, base+"/")
	if !strings.Contains(body, "First") {
		t.Fatalf("initial page missing content: %s", body)
	}
	if !strings.Contains(body, "/__mtime") {
		t.Fatal("reload script not injected into served page")
	}
	if v := httpGetBody(t, base+"/__mtime"); v != "1" {
		t.Fatalf("expected initial version 1, got %q", v)
	}

	if err := os.WriteFile(in, []byte("# Second"), 0644); err != nil {
		t.Fatal(err)
	}
	waitForMtimeChange(t, base, "1")

	body = httpGetBody(t, base+"/")
	if !strings.Contains(body, "Second") {
		t.Fatalf("page not re-rendered after change: %s", body)
	}

	cancel()
	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("Serve returned error: %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("Serve did not return after context cancel")
	}
}
```

- [ ] **Step 2: Run the test to verify it fails**

Run: `go test ./internal -run TestServeRerendersOnChange -v`
Expected: FAIL — `undefined: Serve`.

- [ ] **Step 3: Implement `internal/serve.go`**

Create `internal/serve.go`:

```go
// serve.go — `mkdown serve`: serve a single markdown file's rendered HTML on a
// localhost port and auto-refresh the browser whenever the file changes.
//
// The rendered page is held in memory (serveHolder) and re-rendered by a poll
// goroutine using the same modtime+size change detection as Watch (see
// watch.go's statSig). Each served page has a tiny script injected that polls
// /__mtime and reloads when the version counter changes. Localhost-only, on an
// OS-assigned free port. Wired up from cmd/mkdown/main.go; see
// docs/superpowers/specs/2026-07-04-serve-mode-design.md.
package internal

import (
	"context"
	"fmt"
	"html"
	"net"
	"net/http"
	"strings"
	"sync"
	"time"
)

// reloadScript is injected before </body> of every served page. It polls
// /__mtime every 300ms and reloads the page when the version counter changes.
const reloadScript = `<script>
(function(){var v=null;setInterval(function(){fetch('/__mtime').then(function(r){return r.text()}).then(function(t){if(v!==null&&t!==v){location.reload()}v=t}).catch(function(){})},300)})();
</script>`

// Serve renders in and serves it on a localhost port, re-rendering and
// signalling the browser to reload every time the file changes, until ctx is
// cancelled. If ready is non-nil it is called once with the base URL
// ("http://127.0.0.1:PORT") after the listener is bound — main uses it to open
// the browser, tests use it to learn the assigned port. Change detection polls
// modtime+size every interval, the same mechanism as Watch.
func Serve(ctx context.Context, c *Converter, in string, interval time.Duration, ready func(url string)) error {
	h := &serveHolder{}
	h.set(renderPage(c, in)) // initial render (or error page); version -> 1

	mux := http.NewServeMux()
	mux.HandleFunc("/__mtime", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain")
		fmt.Fprintf(w, "%d", h.getVersion())
	})
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Write(h.getHTML())
	})

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return err
	}
	if ready != nil {
		ready("http://" + ln.Addr().String())
	}

	server := &http.Server{Handler: mux}

	// Poll for file changes and re-render.
	go func() {
		mod, size := statSig(in)
		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				m, s := statSig(in)
				if m.IsZero() || (m.Equal(mod) && s == size) {
					continue
				}
				mod, size = m, s
				h.set(renderPage(c, in))
			}
		}
	}()

	// Shut the server down when the context is cancelled.
	go func() {
		<-ctx.Done()
		shutCtx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		_ = server.Shutdown(shutCtx)
	}()

	if err := server.Serve(ln); err != nil && err != http.ErrServerClosed {
		return err
	}
	return nil
}

// renderPage returns the bytes to serve for in: the rendered document with the
// reload script injected before </body>, or a minimal error page (also with
// the reload script, so a fixed file reloads out of the error state).
func renderPage(c *Converter, in string) []byte {
	var s string
	if out, err := c.Render(in); err != nil {
		s = "<!DOCTYPE html><html><head><meta charset=\"utf-8\"><title>mkdown error</title></head><body><pre>" +
			html.EscapeString(err.Error()) + "</pre></body></html>"
	} else {
		s = string(out)
	}
	if i := strings.LastIndex(s, "</body>"); i != -1 {
		return []byte(s[:i] + reloadScript + s[i:])
	}
	return []byte(s + reloadScript)
}

// serveHolder is the concurrency-safe current page + version, read by the HTTP
// handlers and written by the poll goroutine.
type serveHolder struct {
	mu      sync.RWMutex
	html    []byte
	version int
}

// set replaces the served page and bumps the version. The slice is never
// mutated in place, so readers holding the old slice are unaffected.
func (h *serveHolder) set(b []byte) {
	h.mu.Lock()
	h.html = b
	h.version++
	h.mu.Unlock()
}

func (h *serveHolder) getHTML() []byte {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return h.html
}

func (h *serveHolder) getVersion() int {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return h.version
}
```

- [ ] **Step 4: Run the test and the race detector**

Run: `go test ./internal -run TestServeRerendersOnChange -v`
Expected: PASS.

Run: `go test ./internal -race -run 'TestServe|TestWatch|TestRender'`
Expected: `ok`, no `DATA RACE`.

- [ ] **Step 5: Commit**

```bash
git add internal/serve.go internal/serve_test.go
git commit -m "Add internal.Serve: live-preview HTTP server with poll reload"
```

---

## Task 3: Wire the `serve` subcommand into the CLI

**Files:**
- Modify: `cmd/mkdown/main.go`

### Current state

`cmd/mkdown/main.go` starts `main()` with `debug.SetGCPercent(400)` then a manual flag-parsing loop. It imports (among others) `context`, `fmt`, `os`, `os/signal`, `runtime`, `strings`, `time`, and `github.com/ekinertac/mkdown/internal`. Helper funcs `outputFor`, `featureSuffix` live at the bottom. There are no subcommands today.

- [ ] **Step 1: Add the `os/exec` import**

Add `"os/exec"` to the import block in `cmd/mkdown/main.go` (keep imports grouped/sorted; `os/exec` goes just after `"os"`... actually after `"os/signal"` is fine — `gofmt` will order it). `runtime`, `context`, `os/signal`, `fmt`, `os`, `strings`, `time` are already imported.

- [ ] **Step 2: Dispatch `serve` at the top of `main()`**

Immediately after `debug.SetGCPercent(400)` (the first statement in `main()`), insert:

```go
	// serve is a subcommand: `mkdown serve <file> [flags]`. Handle it before the
	// flag parser, which is built around a single positional input file.
	if len(os.Args) > 1 && os.Args[1] == "serve" {
		runServe(os.Args[2:])
		return
	}
```

- [ ] **Step 3: Add `runServe` and `openBrowser`**

Add these two functions to `cmd/mkdown/main.go` (near the other helpers at the bottom):

```go
// runServe parses `mkdown serve <file> [flags]` and runs the live-preview
// server until interrupted. It parses its own args (a single markdown file plus
// the render flags) rather than reusing the main flag loop, which is built for
// batch/convert output paths that don't apply here.
func runServe(args []string) {
	var (
		inputPath     string
		theme         = "dark"
		enableMermaid bool
		enableMath    bool
		noHighlight   bool
	)
	for i := 0; i < len(args); i++ {
		arg := args[i]
		switch arg {
		case "-t", "--theme":
			if i+1 < len(args) {
				theme = args[i+1]
				if theme != "dark" && theme != "light" {
					fmt.Fprintf(os.Stderr, "Error: Invalid theme '%s'. Available: dark, light\n", theme)
					os.Exit(1)
				}
				i++
			} else {
				fmt.Fprintln(os.Stderr, "Error: -t requires an argument")
				os.Exit(1)
			}
		case "--mermaid":
			enableMermaid = true
		case "--math":
			enableMath = true
		case "--no-highlight":
			noHighlight = true
		default:
			if !strings.HasPrefix(arg, "-") && inputPath == "" {
				inputPath = arg
			} else {
				fmt.Fprintf(os.Stderr, "Error: Unknown argument: %s\n", arg)
				os.Exit(1)
			}
		}
	}

	if inputPath == "" {
		fmt.Fprintln(os.Stderr, "Usage: mkdown serve <input.md> [--theme|--mermaid|--math|--no-highlight]")
		os.Exit(1)
	}
	if _, err := os.Stat(inputPath); os.IsNotExist(err) {
		fmt.Fprintf(os.Stderr, "Error: File '%s' not found\n", inputPath)
		os.Exit(1)
	}
	lower := strings.ToLower(inputPath)
	if !strings.HasSuffix(lower, ".md") && !strings.HasSuffix(lower, ".markdown") {
		fmt.Fprintf(os.Stderr, "Error: Input file must be a markdown file (.md or .markdown): %s\n", inputPath)
		os.Exit(1)
	}

	converter := internal.NewConverterWithOptions(internal.ConverterOptions{
		Theme:            theme,
		EnableMermaid:    enableMermaid,
		EnableMath:       enableMath,
		DisableHighlight: noHighlight,
	})

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()

	ready := func(url string) {
		fmt.Printf("serving %s (Ctrl+C to stop)\n", url)
		_ = openBrowser(url) // best effort; URL already printed
	}

	if err := internal.Serve(ctx, converter, inputPath, 250*time.Millisecond, ready); err != nil {
		stop()
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}

// openBrowser opens url in the default browser, best-effort, cross-platform.
func openBrowser(url string) error {
	switch runtime.GOOS {
	case "darwin":
		return exec.Command("open", url).Start()
	case "windows":
		return exec.Command("rundll32", "url.dll,FileProtocolHandler", url).Start()
	default:
		return exec.Command("xdg-open", url).Start()
	}
}
```

- [ ] **Step 4: Add help text and an example**

In the `-h, --help` case of the main flag loop, add a `serve` line to the flags list and an example. Replace:

```go
			fmt.Println("  --watch              Re-render the file on every change (single file; Ctrl+C to stop)")
			fmt.Println("  -v, --version        Show version")
```
with:
```go
			fmt.Println("  --watch              Re-render the file on every change (single file; Ctrl+C to stop)")
			fmt.Println("  -v, --version        Show version")
			fmt.Println("\nSubcommands:")
			fmt.Println("  serve <input.md>     Live preview in the browser, auto-refresh on save (Ctrl+C to stop)")
```

And in the examples list, after the `mkdown --watch README.md` line (if present) or the last example, add:
```go
			fmt.Println("  mkdown serve README.md       # live preview in the browser")
```

(If the exact anchor lines differ, place the flags line among the flag `Println`s and the example among the example `Println`s — match the surrounding style.)

- [ ] **Step 5: Build, vet, test**

Run: `go build ./... && go vet ./...`
Expected: no output.

Run: `go test ./...`
Expected: all `ok`.

- [ ] **Step 6: Manual smoke test**

The server is long-running, so this is a manual check (use a background process + polling; if `sleep` is unavailable, poll in an `until` loop):

```bash
go build -o /tmp/mkdown-serve ./cmd/mkdown
D="$(mktemp -d)"; printf '# Hello\n' > "$D/note.md"
( /tmp/mkdown-serve serve "$D/note.md" > /tmp/serve.out 2>&1 & echo $! > /tmp/serve.pid )
# read the URL it printed:
#   serving http://127.0.0.1:PORT (Ctrl+C to stop)
URL=$(grep -o 'http://127.0.0.1:[0-9]*' /tmp/serve.out | head -1)
curl -s "$URL/" | grep -q "Hello" && echo "initial OK"
curl -s "$URL/__mtime"                                  # -> 1
printf '# Changed\n' > "$D/note.md"
# poll until /__mtime != 1, then:
curl -s "$URL/" | grep -q "Changed" && echo "reload OK"
kill "$(cat /tmp/serve.pid)"
```
Verify: `initial OK`, `/__mtime` starts at `1` then bumps, `reload OK`, and Ctrl+C/kill exits cleanly. Also confirm the served page contains `/__mtime` (the injected reload script). If you cannot run the long-running smoke test in your environment, rely on build+vet+test and the `TestServeRerendersOnChange` integration test, and say so in your report.

- [ ] **Step 7: Commit**

```bash
git add cmd/mkdown/main.go
git commit -m "Add serve subcommand for live browser preview"
```

---

## Task 4: Document `serve` in the README

**Files:**
- Modify: `README.md`

- [ ] **Step 1: Add a Live Preview subsection**

In `README.md`, in the "Usage" area near the "Batch Conversion" subsection, add a new subsection:

```markdown
### Live Preview

Serve a file and edit it with your browser auto-refreshing on every save:

    mkdown serve readme.md

This starts a local server (a free port on `127.0.0.1`), opens your browser, and
re-renders on each save — the tab reloads itself. Ctrl+C stops it. The render
flags (`--theme`, `--mermaid`, `--math`, `--no-highlight`) apply.
```

- [ ] **Step 2: Regenerate the rendered README and confirm build**

Run: `go run ./cmd/mkdown README.md >/dev/null && echo "README.html regenerated"`
Expected: `README.html regenerated`.

- [ ] **Step 3: Commit**

```bash
git add README.md README.html
git commit -m "Document serve subcommand in the README"
```

---

## Done criteria

ALL must hold:

- [ ] `go build ./... && go vet ./...` exit 0
- [ ] `go test ./...` all `ok`; `TestRender` and `TestServeRerendersOnChange` exist and pass
- [ ] `go test ./internal -race -run 'TestServe|TestWatch|TestRender'` clean (no `DATA RACE`)
- [ ] `internal/serve.go` and `internal/serve_test.go` exist; `internal/converter.go` has `Render` and `buildDocument`; `Convert` still streams to file
- [ ] `mkdown serve <file>.md` starts a server, opens a browser, and reloads on save (manual smoke)
- [ ] `mkdown README.md` (no subcommand) still converts as before — subcommand dispatch didn't break the flag path
- [ ] No files outside the in-scope lists modified (`git status`)
- [ ] `plans/README.md` — N/A (this plan lives under `docs/superpowers/plans/`; no index to update)

## STOP conditions

Stop and report (do not improvise) if:

- The `Convert` excerpt in Task 1 doesn't match the live code (drift since `0c9cab6`).
- `statSig` is not present in `internal/watch.go` (a prior refactor moved/renamed it) — the poll loop depends on it.
- Adding the `serve` subcommand dispatch changes the behavior of any existing `main_test.go` test (run `go test ./cmd/mkdown` — it must stay green).
- A step's verification fails twice after a reasonable fix attempt.
- The fix appears to require touching a file outside the in-scope list.

## Maintenance notes

- **Highlight-on-change is deferred** (see the spec's "Future enhancement"). When built, it replaces the injected `location.reload()` with a fetch-and-morph client; the `/__mtime` version endpoint and poll architecture stay.
- The reload poll interval (300ms client, 250ms server) is a comfort/latency tradeoff; if it ever feels laggy, both are single constants.
- `Serve` binds `127.0.0.1` deliberately (never `0.0.0.0`) — a reviewer should ensure that stays, since it's the whole security posture of the feature.
- If a `--port` flag is added later, thread it into `Serve` (replace the `:0`) and `runServe`.
- `renderPage` injects the reload script into the error page too, so a broken save recovers on fix — keep that if the error page is restyled.

## Self-review (advisor)

**Spec coverage:** Render (Task 1) ✓; serve subcommand + localhost:0 + browser open (Task 3) ✓; poll goroutine reusing statSig + version bump (Task 2) ✓; `/` + `/__mtime` + injected 300ms poll script (Task 2) ✓; RWMutex holder (Task 2) ✓; error page + bump on render failure (Task 2, `renderPage`) ✓; SIGINT clean shutdown (Tasks 2, 3) ✓; render flags apply (Task 3 `runServe`) ✓; Convert streaming preserved (Task 1) ✓; tests incl. -race (Tasks 1, 2) ✓; README (Task 4) ✓; out-of-scope items none added ✓.

**Placeholder scan:** none — every code step is complete; every command has an expected result.

**Type consistency:** `Serve(ctx context.Context, c *Converter, in string, interval time.Duration, ready func(url string)) error`, `Render(inputPath string) ([]byte, error)`, `buildDocument(inputPath string) (*Document, error)`, `renderPage(c *Converter, in string) []byte`, `serveHolder.{set([]byte), getHTML() []byte, getVersion() int}`, `openBrowser(url string) error`, `runServe(args []string)` — used identically across `serve.go`, `converter.go`, `serve_test.go`, and `main.go`. `statSig` and `NewConverter`/`NewConverterWithOptions` match existing signatures.
