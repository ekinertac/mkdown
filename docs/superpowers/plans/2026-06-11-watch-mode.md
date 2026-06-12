# `--watch` Mode Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add a `--watch` flag so `mkdown --watch readme.md` re-renders the file to HTML every time it changes on disk, until Ctrl+C.

**Architecture:** A new `internal.Watch` function polls the input file's modification time and size every 250ms and re-runs the existing `Converter.Convert` whenever either changes. `main.go` gains a `--watch` flag that, for a single input, runs this loop under a `SIGINT`-cancelled `context.Context` instead of converting once and exiting. Polling (not `fsnotify`) keeps the binary dependency-free and is immune to editor temp-file-rename saves.

**Tech Stack:** Go standard library only — `context`, `os/signal`, `time`. No new dependencies. Reuses the existing `internal.Converter` (goldmark + chroma).

**Spec:** `docs/superpowers/specs/2026-06-11-watch-mode-design.md`

---

## File Structure

- `internal/watch.go` — **new.** The `Watch` function and its `statSig` helper. One responsibility: poll a file and re-render on change.
- `internal/watch_test.go` — **new.** Tests for `Watch` (re-renders on change; does not re-render without change).
- `cmd/mkdown/main.go` — **modify.** Add the `--watch` flag, its help text, and a branch that runs the watch loop under a signal-cancelled context for a single input.
- `README.md` — **modify.** Document `--watch` in the flags list and examples.

All rendering stays in `internal.Converter`; `Watch` only orchestrates polling and re-rendering, so it stays small and testable.

---

## Task 1: `internal.Watch` — poll and re-render on change

**Files:**
- Create: `internal/watch.go`
- Test: `internal/watch_test.go`

- [ ] **Step 1: Write the failing test**

Create `internal/watch_test.go`:

```go
package internal

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// waitForContains polls a file until it contains substr or the timeout elapses.
func waitForContains(t *testing.T, path, substr string) {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if b, err := os.ReadFile(path); err == nil && strings.Contains(string(b), substr) {
			return
		}
		time.Sleep(5 * time.Millisecond)
	}
	t.Fatalf("file %s did not contain %q within timeout", path, substr)
}

func TestWatchRerendersOnChange(t *testing.T) {
	dir := t.TempDir()
	in := filepath.Join(dir, "doc.md")
	out := filepath.Join(dir, "doc.html")
	if err := os.WriteFile(in, []byte("# First"), 0644); err != nil {
		t.Fatal(err)
	}

	c := NewConverter("dark")
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	done := make(chan error, 1)
	go func() { done <- Watch(ctx, c, in, out, 10*time.Millisecond) }()

	// Initial render should produce the output with the original content.
	waitForContains(t, out, "First")

	// Change the file; expect a re-render with the new content. "# First" (7
	// bytes) and "# Second" (8 bytes) differ in size, so the size check catches
	// the change even if the two writes land in the same mtime tick.
	if err := os.WriteFile(in, []byte("# Second"), 0644); err != nil {
		t.Fatal(err)
	}
	waitForContains(t, out, "Second")

	cancel()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("Watch did not return after context cancel")
	}
}
```

- [ ] **Step 2: Run the test to verify it fails**

Run: `go test ./internal -run TestWatchRerendersOnChange -v`
Expected: FAIL — compile error, `undefined: Watch`.

- [ ] **Step 3: Write the minimal implementation**

Create `internal/watch.go`:

```go
// watch.go — `--watch` mode: re-render a single markdown file to HTML every
// time it changes on disk, until the context is cancelled.
//
// Change detection is by polling the file's modtime + size (see statSig), not
// an event-based watcher. Polling adds no dependency and is immune to the
// editor atomic-save pattern (write temp + rename over target) that stops
// event watchers from firing after the first save. Render errors are reported
// but never stop the loop. Wired up from cmd/mkdown/main.go; see
// docs/superpowers/specs/2026-06-11-watch-mode-design.md.
package internal

import (
	"context"
	"fmt"
	"os"
	"time"
)

// Watch renders in -> out once immediately, then re-renders every time the
// input file's modtime or size changes, polling every `interval`, until ctx is
// cancelled. A render failure is reported to stderr but does not stop watching.
func Watch(ctx context.Context, c *Converter, in, out string, interval time.Duration) error {
	// Initial render so the output exists right away. A failure here is
	// reported but not fatal — the user may be starting on a broken file.
	if err := c.Convert(in, out); err != nil {
		fmt.Fprintf(os.Stderr, "✗ %s: %v\n", in, err)
	} else {
		fmt.Printf("✓ Generated: %s\n", out)
	}
	fmt.Printf("watching %s → %s (Ctrl+C to stop)\n", in, out)

	lastMod, lastSize := statSig(in)

	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return nil
		case <-ticker.C:
			mod, size := statSig(in)
			if mod.IsZero() {
				continue // transient stat error (e.g. mid-rename); retry next tick
			}
			if mod.Equal(lastMod) && size == lastSize {
				continue // unchanged
			}
			lastMod, lastSize = mod, size
			if err := c.Convert(in, out); err != nil {
				fmt.Fprintf(os.Stderr, "✗ %s: %v\n", in, err)
				continue
			}
			fmt.Printf("✓ %s (%s)\n", out, time.Now().Format("15:04:05"))
		}
	}
}

// statSig returns the file's modtime and size, or a zero time on any stat
// error (so callers can treat zero as "couldn't read, skip this tick").
func statSig(path string) (time.Time, int64) {
	info, err := os.Stat(path)
	if err != nil {
		return time.Time{}, 0
	}
	return info.ModTime(), info.Size()
}
```

- [ ] **Step 4: Run the test to verify it passes**

Run: `go test ./internal -run TestWatchRerendersOnChange -v`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/watch.go internal/watch_test.go
git commit -m "Add internal.Watch: poll a file and re-render on change"
```

---

## Task 2: Guard against spurious re-renders

This test pins the "only re-render when the input actually changes" behavior so a future change to the polling logic can't silently start rewriting the output every tick.

**Files:**
- Test: `internal/watch_test.go` (append)

- [ ] **Step 1: Write the test**

Append to `internal/watch_test.go`:

```go
func TestWatchNoRerenderWithoutChange(t *testing.T) {
	dir := t.TempDir()
	in := filepath.Join(dir, "doc.md")
	out := filepath.Join(dir, "doc.html")
	if err := os.WriteFile(in, []byte("# Stable"), 0644); err != nil {
		t.Fatal(err)
	}

	c := NewConverter("dark")
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go func() { _ = Watch(ctx, c, in, out, 10*time.Millisecond) }()

	waitForContains(t, out, "Stable")

	info1, err := os.Stat(out)
	if err != nil {
		t.Fatal(err)
	}
	// Let many poll ticks pass with no change to the input file.
	time.Sleep(150 * time.Millisecond)
	info2, err := os.Stat(out)
	if err != nil {
		t.Fatal(err)
	}
	if !info1.ModTime().Equal(info2.ModTime()) {
		t.Fatal("output was rewritten despite no input change")
	}
}
```

- [ ] **Step 2: Run the test to verify it passes**

Run: `go test ./internal -run TestWatchNoRerenderWithoutChange -v`
Expected: PASS (the Task 1 implementation already only re-renders on change; this locks it in).

- [ ] **Step 3: Run the whole internal suite to confirm nothing regressed**

Run: `go test ./internal`
Expected: `ok  github.com/ekinertac/mkdown/internal`

- [ ] **Step 4: Commit**

```bash
git add internal/watch_test.go
git commit -m "Test that watch does not re-render an unchanged file"
```

---

## Task 3: Wire `--watch` into the CLI

**Files:**
- Modify: `cmd/mkdown/main.go`

- [ ] **Step 1: Add the new imports**

In `cmd/mkdown/main.go`, change the import block (currently starting at line 12) so it also imports `context` and `os/signal`. The full block should read:

```go
import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"runtime"
	"runtime/debug"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/ekinertac/mkdown/internal"
)
```

- [ ] **Step 2: Add the `watch` variable**

In the `var (...)` declaration inside `main()` (currently ending with `noHighlight bool` at line 49), add a `watch` field:

```go
	var (
		showVersion   bool
		outputPath    string
		inputPaths    []string
		theme         = "dark" // default theme
		enableMermaid bool
		enableMath    bool
		noHighlight   bool
		watch         bool
	)
```

- [ ] **Step 3: Parse the `--watch` flag**

In the flag `switch`, add a case next to the other boolean flags. Replace:

```go
		case "--no-highlight":
			noHighlight = true
```

with:

```go
		case "--no-highlight":
			noHighlight = true
		case "--watch":
			watch = true
```

- [ ] **Step 4: Add the help text**

In the `-h, --help` case, replace:

```go
			fmt.Println("  --no-highlight       Skip syntax highlighting (much faster for bulk/code-heavy docs)")
			fmt.Println("  -v, --version        Show version")
```

with:

```go
			fmt.Println("  --no-highlight       Skip syntax highlighting (much faster for bulk/code-heavy docs)")
			fmt.Println("  --watch              Re-render the file on every change (single file; Ctrl+C to stop)")
			fmt.Println("  -v, --version        Show version")
```

And in the examples list of the same case, replace:

```go
			fmt.Println("  mkdown math.md --math")
```

with:

```go
			fmt.Println("  mkdown math.md --math")
			fmt.Println("  mkdown --watch README.md     # re-render on save until Ctrl+C")
```

- [ ] **Step 5: Add the watch branch**

The converter is built at line 140-145. Insert the watch branch immediately after it, before the `// Single file: keep the original, chatty behavior.` comment. Replace:

```go
	converter := internal.NewConverterWithOptions(internal.ConverterOptions{
		Theme:            theme,
		EnableMermaid:    enableMermaid,
		EnableMath:       enableMath,
		DisableHighlight: noHighlight,
	})

	// Single file: keep the original, chatty behavior.
```

with:

```go
	converter := internal.NewConverterWithOptions(internal.ConverterOptions{
		Theme:            theme,
		EnableMermaid:    enableMermaid,
		EnableMath:       enableMath,
		DisableHighlight: noHighlight,
	})

	// Watch mode: re-render a single file on every change until interrupted.
	if watch {
		if len(inputPaths) != 1 {
			fmt.Fprintln(os.Stderr, "Error: --watch requires exactly one input file")
			os.Exit(1)
		}
		out := outputPath
		if out == "" {
			out = outputFor(inputPaths[0])
		}
		ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
		defer stop()
		if err := internal.Watch(ctx, converter, inputPaths[0], out, 250*time.Millisecond); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
		return
	}

	// Single file: keep the original, chatty behavior.
```

- [ ] **Step 6: Build and vet**

Run: `go build ./... && go vet ./...`
Expected: no output (success).

- [ ] **Step 7: Run the full test suite**

Run: `go test ./...`
Expected: all packages `ok`.

- [ ] **Step 8: Manual smoke test**

Run these by hand (the watch process is long-running, so it's a manual check, not an automated test):

```bash
go build -o /tmp/mkdown-watch ./cmd/mkdown
cd "$(mktemp -d)"
printf '# Hello\n' > note.md
/tmp/mkdown-watch --watch note.md
# In another terminal: edit note.md and save. The first terminal should print
#   ✓ note.html (HH:MM:SS)
# and note.html should contain the new content. Ctrl+C stops it cleanly with
# no stack trace.
```

Also verify the guard rejects multiple inputs:

```bash
/tmp/mkdown-watch --watch a.md b.md
# Expected: "Error: --watch requires exactly one input file" and exit code 1.
```

- [ ] **Step 9: Commit**

```bash
git add cmd/mkdown/main.go
git commit -m "Add --watch flag to re-render a single file on change"
```

---

## Task 4: Document `--watch` in the README

**Files:**
- Modify: `README.md`

- [ ] **Step 1: Add `--watch` to the CLI flags block**

In `README.md`, find the fenced flags block (inside the "CLI Flags" section) and add a `--watch` line after the `--no-highlight` line:

```
  --no-highlight       Skip syntax highlighting (faster; code renders as plain <pre><code>)
  --watch              Re-render the file on every change (single file; Ctrl+C to stop)
```

- [ ] **Step 2: Add a `--watch` example**

In the same flags block's `Examples:` list (or the examples just below it), add:

```
  mkdown --watch README.md                 # re-render on save until Ctrl+C
```

- [ ] **Step 3: Regenerate the rendered README and verify the build**

Run:

```bash
go run ./cmd/mkdown README.md >/dev/null && echo "README.html regenerated"
```

Expected: `README.html regenerated`.

- [ ] **Step 4: Commit**

```bash
git add README.md README.html
git commit -m "Document --watch flag in the README"
```

---

## Self-Review

**Spec coverage:**
- Regenerate-only, no server → `Watch` calls `Converter.Convert` and writes the file; nothing else. ✓ (Tasks 1)
- Single file only; error on multiple → guard in Task 3 Step 5. ✓
- Poll mtime (+ size) every 250ms → `statSig` + ticker; main passes `250ms`. ✓ (Tasks 1, 3)
- Initial render immediately → first `Convert` before the loop. ✓ (Task 1)
- Render errors non-fatal, keep watching → `continue` after logging in both the initial and loop paths. ✓ (Task 1)
- Transient stat misses skipped → `if mod.IsZero() { continue }`. ✓ (Task 1)
- Clean SIGINT shutdown, no stack trace → `signal.NotifyContext` + `<-ctx.Done()` returns nil. ✓ (Tasks 1, 3)
- All render flags still apply → the same `converter` (built from all flags) is passed to `Watch`. ✓ (Task 3)
- Output lines per spec (`✓ Generated:`, `watching … (Ctrl+C to stop)`, `✓ <out> (HH:MM:SS)`) → in `Watch`. ✓ (Task 1)
- Tests: re-render on change + no spurious re-render → Tasks 1 and 2. ✓
- Docs: help text + README → Tasks 3 and 4. ✓
- Out of scope (server, auto-refresh, auto-open, globs) → none added. ✓

**Placeholder scan:** No TBD/TODO; every code step shows full code; every command lists expected output. ✓

**Type consistency:** `Watch(ctx context.Context, c *Converter, in, out string, interval time.Duration) error` and `statSig(path string) (time.Time, int64)` are used identically in `internal/watch.go`, the tests, and the `main.go` call site (`internal.Watch(ctx, converter, inputPaths[0], out, 250*time.Millisecond)`). `NewConverter("dark")` matches the existing constructor. ✓
