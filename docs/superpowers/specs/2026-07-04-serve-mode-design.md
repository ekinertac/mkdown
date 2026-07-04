# Design: `mkdown serve` (live preview)

**Date:** 2026-07-04
**Status:** Approved (design), pending implementation plan
**Component:** `cmd/mkdown`, `internal`

## Summary

Add a `serve` subcommand that starts a local HTTP server rendering a single
markdown file to HTML, opens it in the browser, and **auto-refreshes the browser
whenever the file changes** — a live markdown preview.

```
mkdown serve readme.md
→ serving http://127.0.0.1:52341  (opens browser)
[edit + save readme.md] → browser tab refreshes to the new HTML automatically
```

## Decisions (settled in brainstorming, fixed for v1)

1. **Live reload**, not a static snapshot. The server watches the file,
   re-renders in memory on save, and the browser refreshes itself.
2. **Reload via polling**, not SSE. An injected script polls a version endpoint
   every ~300ms and reloads when the version changes. No long-lived
   connections; stdlib only.
3. **Single file only.** `mkdown serve readme.md` serves exactly one file.
   Globs / directories are out of scope for v1.
4. **`serve` is a subcommand** (`os.Args[1] == "serve"`) — mkdown's first. It's
   additive: any invocation without `serve` flows through the existing flag
   parser unchanged.
5. **Localhost only.** Bind `127.0.0.1:0` — an OS-assigned free port, never
   exposed to the network.
6. **Auto-assigned port.** No `--port` flag in v1 ("available port").

## Architecture

### CLI integration — `cmd/mkdown/main.go`

- Detect `serve` as the first argument. When present, treat the next positional
  argument as the input file and continue parsing the existing render flags
  (`-t`/`--theme`, `--mermaid`, `--math`, `--no-highlight`) — they all apply to
  every render. `-o` is meaningless for serve and is ignored.
- Reuse the existing single-file validation: exactly one input, it must exist,
  and must be `.md`/`.markdown`. Error and exit non-zero otherwise (e.g.
  `mkdown serve` with no file, or with multiple files).
- Build the `Converter` from the parsed flags exactly as the single-file path
  does, then call `internal.Serve`.
- Wire a `SIGINT`-cancelled `context.Context` (`signal.NotifyContext`) so Ctrl+C
  shuts the server down cleanly.

### New `Converter.Render` — `internal/converter.go`

Today `Convert(in, out string) error` renders **and writes a file**; there is no
way to obtain the HTML in memory. Add:

```go
// Render reads the input markdown file and returns the full standalone HTML
// document as bytes, without writing anything to disk.
func (c *Converter) Render(in string) ([]byte, error)
```

**Preserve `Convert`'s streaming.** `Convert` currently streams the templated
output straight to the file through a buffered writer (a deliberate optimization
— it avoids buffering a second full-size copy of the document). Do NOT implement
`Convert` as `Render` + `WriteFile`; that would reintroduce the full-document
buffer.

Instead, factor out the shared work — everything that produces the populated
`*Document` (read source, parse frontmatter, protect math, `markdown.Convert`,
restore math, `injectScripts`) — into one private helper, then:
- `Render(in)` builds the document and executes the template into a
  `bytes.Buffer`, returning its bytes.
- `Convert(in, out)` builds the document and executes the template into the
  buffered **file** writer exactly as today (streaming preserved), keeping the
  output-dir creation.

No behavior change to `Convert`; existing `Convert` tests still pass.

### New `internal/serve.go`

```go
func Serve(ctx context.Context, c *Converter, in string, interval time.Duration, ready func(url string)) error
```

Behavior:

1. **Initial render** into a mutex-guarded holder `{ html []byte; version int }`
   (version starts at 1). On initial render error, store an error page instead
   (see Error handling) — serving still starts so the user can fix the file
   live.
2. **Bind** an `http.Server` to `127.0.0.1:0`, read the actual assigned port from
   the listener, and call `ready("http://127.0.0.1:<port>")` if `ready != nil`.
   (The callback lets `main` open the browser and lets tests capture the URL
   without a fixed port.)
3. **Poll goroutine** — the same modtime+size change detection used by `Watch`
   (`internal/watch.go`), on the given `interval`. On a detected change,
   re-render: on success store the new HTML; on error store an error page.
   Either way, **increment `version`** so the browser reloads.
4. **Handlers:**
   - `GET /` → the current HTML with a reload `<script>` injected before
     `</body>` (the template always emits `</body>`; if absent, append). The
     script polls `/__mtime` every 300ms and calls `location.reload()` when the
     returned value differs from the last seen.
   - `GET /__mtime` → the current `version` as plain text.
5. **Shutdown** — when `ctx` is cancelled, call `server.Shutdown(context)` and
   return. Ctrl+C exits cleanly with no stack trace.

### Browser open — `main` (small helper)

`openBrowser(url string) error`, cross-platform: darwin `open`, linux
`xdg-open`, windows `rundll32 url.dll,FileProtocolHandler`. **Best effort** — if
it fails, the URL has already been printed, so the user can open it manually.
`main` passes `ready = func(url){ print "serving <url> (Ctrl+C to stop)";
openBrowser(url) }`.

## Concurrency

Unlike `Watch` (single goroutine), the HTTP handlers run concurrently with the
poll goroutine. The holder (`html`, `version`) is shared mutable state and is
guarded by a `sync.RWMutex` (handlers take the read lock; the poll goroutine
takes the write lock). Verified race-free under `go test -race`.

## Error handling

- **Render error on change:** store a minimal HTML error page (`<pre>` with the
  error text, escaped) as the served content and bump `version`, so the browser
  shows the error live. A subsequent successful render replaces it and bumps
  again. The server never dies on a render error.
- **Initial render error:** same — serve the error page, keep serving.
- **Browser open failure:** ignored (URL already printed).
- **Transient stat miss** during the poll (file briefly gone mid-rename):
  skip that tick, same as `Watch`.

## Testing — `internal/serve_test.go` and `internal/converter_test.go`

- **`Render` unit test:** returns HTML bytes containing expected converted
  content (e.g. an `<h1>` for `# Title`), without writing a file.
- **`Serve` integration test:** start `Serve` in a goroutine on a temp `.md`
  with a short interval, capture the URL via the `ready` callback; `GET /`
  asserts the initial content and that the reload script is present; `GET
  /__mtime` returns `1`; modify the file; poll `/__mtime` until it bumps; `GET /`
  again asserts the new content; cancel `ctx` and assert `Serve` returns. Run
  under `-race`.

## Files touched

- `cmd/mkdown/main.go` — `serve` subcommand dispatch, `openBrowser` helper,
  SIGINT wiring, help text + example.
- `internal/converter.go` — extract `Render`; `Convert` calls it.
- `internal/serve.go` — new; `Serve` + the holder + handlers + reload-script
  injection.
- `internal/serve_test.go` — new; the tests above.
- `internal/converter_test.go` — add the `Render` test.
- `README.md` — document the `serve` subcommand.

## Out of scope (YAGNI, addable later without rework)

- `--port` override (auto-assign only for v1).
- Multiple files, globs, or directory serving.
- SSE / WebSocket reload transport (polling chosen).
- HTTPS, LAN exposure, auth.
- Serving referenced local assets (images/CSS) beyond the self-contained HTML.

## Future enhancement (deferred — its own spec later)

**Highlight-on-change.** On reload, flash the parts of the page that changed and
clear the highlight on click. This is *not* a small add-on to v1: `/__mtime` is
a version counter, not a content diff, and v1 reloads with `location.reload()`
(a full page swap that would destroy any highlight and reset scroll). Doing it
right means computing an actual diff and replacing the reload transport with a
fetch-and-morph client (fetch new HTML, diff against the current DOM, patch only
changed nodes, flash them, clear on click) — effectively a second feature.
v1's poll+reload architecture does not block it: when built, it swaps the
injected `location.reload()` for the fetch-and-morph path. Deferred to its own
brainstorm → spec → plan.
