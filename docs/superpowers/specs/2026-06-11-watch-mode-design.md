# Design: `--watch` mode

**Date:** 2026-06-11
**Status:** Approved (design), pending implementation plan
**Component:** `cmd/mkdown`, `internal`

## Summary

Add a `--watch` flag that turns mkdown from a convert-and-exit CLI into a
long-running process which re-renders a single markdown file to HTML every time
the file changes on disk.

```
mkdown --watch readme.md
→ watching readme.md → readme.html (Ctrl+C to stop)
[edit + save readme.md] → ✓ readme.html (12:34:56)
```

The user keeps `readme.html` open in a browser and refreshes (or uses a browser
auto-reload extension). mkdown's only job is to keep the `.html` current.

## Decisions

These were settled during brainstorming and are fixed for v1:

1. **Regenerate-only, no server.** On change, re-run the existing conversion and
   overwrite the output file. No HTTP server, no live-reload script injection,
   no WebSocket/SSE. The browser refresh is the user's responsibility.
2. **Single file only.** `mkdown --watch readme.md` watches exactly one file.
   Globs / multiple inputs / directory watching are out of scope for v1.
3. **Detect changes by polling mtime.** A ~250ms poll loop comparing the file's
   modification time, not an event-based watcher.

### Why polling over fsnotify

- **Zero new dependencies** — keeps the single-binary, minimal-deps identity.
- **Immune to editor atomic saves.** vim/VSCode and others save by writing a
  temp file and renaming it over the target, which pulls the inode out from
  under an event-based watch (events stop firing after the first save). Polling
  `os.Stat(path)` just observes "mtime changed" and is unaffected.
- **Self-debouncing** — no event-burst coalescing needed.
- The only cost is up to ~250ms latency, imperceptible for a save-and-refresh
  loop.

## Architecture

### CLI integration — `cmd/mkdown/main.go`

- Add a `--watch` boolean to the manual flag-parsing loop.
- Validation (before doing anything): `--watch` requires **exactly one** input
  file. Error and exit non-zero on zero or multiple inputs.
- All existing render flags (`-o`, `-t`, `--mermaid`, `--math`,
  `--no-highlight`) continue to apply and are passed into the converter as
  today; every re-render uses them.
- When `--watch` is set, construct the `Converter` exactly as the single-file
  path does, resolve the output path the same way (`-o` or `<input>.html`), then
  call the watch loop instead of converting once.
- Wire `SIGINT` (Ctrl+C) to cancel a `context.Context` so the loop exits
  cleanly without a stack trace.

### Watch loop — new `internal/watch.go`

```go
func Watch(ctx context.Context, c *Converter, in, out string, interval time.Duration) error
```

Behavior:

1. **Initial render** immediately, so the `.html` exists right away. Print the
   usual `✓ Generated: <out>` line, then `watching <in> → <out> (Ctrl+C to stop)`.
2. Record the input's current `mtime` and `size`.
3. On each `interval` tick (driven by a `time.Ticker`):
   - `os.Stat(in)`. On a transient error (file briefly missing during a
     rename), skip this tick — do not crash.
   - If `mtime` **or** `size` differs from the last seen values, re-render via
     `c.Convert(in, out)` and update the stored values.
   - On success print `✓ <out> (HH:MM:SS)`; on render error print
     `✗ <in>: <err>` and **keep watching**.
4. Return when `ctx` is cancelled.

Rationale for params:
- `context.Context` makes shutdown clean and testable (main cancels on SIGINT;
  tests cancel directly).
- `interval` is injected so main passes 250ms and tests pass ~10ms for
  determinism.
- `size` is a cheap secondary signal in case two saves land within the same
  coarse mtime tick.

## Error handling

- A render error (bad markdown, transient write failure) during watch is logged
  and the loop continues — a broken save must not end the session.
- The initial render error is also logged-and-continue, so the user can start
  watching a currently-broken file and fix it live.
- Input existence is validated before the loop begins (the file must exist to
  start watching).
- Transient `os.Stat` failures mid-loop are ignored for that tick.

## Testing — `internal/watch_test.go`

- **Re-render on change:** write a temp `.md`, start `Watch` in a goroutine with
  a short interval (~10ms), modify the `.md`, wait briefly, assert the `.html`
  exists and contains the new content, then cancel the context and assert
  `Watch` returns.
- **No spurious re-render:** with no file change, assert the output isn't
  rewritten (e.g., its mtime is unchanged after a few ticks).
- The `ctx` + `interval` parameters are what make these deterministic and fast.

## Files touched

- `cmd/mkdown/main.go` — `--watch` flag, validation, SIGINT→cancel wiring, branch
  to the watch loop.
- `internal/watch.go` — new; the `Watch` function and polling logic.
- `internal/watch_test.go` — new; tests above.
- `--help` text in `main.go` and the README flags table — document `--watch`.

## Out of scope (YAGNI, deferrable without rework)

- HTTP server / localhost serving.
- Browser auto-refresh (live-reload injection, meta-refresh, polling script).
- Auto-opening the browser.
- Multiple files, globs, or directory watching.
