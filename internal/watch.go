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
	// Snapshot the file's signature before the initial render so that a save
	// racing the initial Convert is still caught on the next tick (at worst one
	// redundant render, never a missed/stale one).
	lastMod, lastSize := statSig(in)

	// Initial render so the output exists right away. A failure here is
	// reported but not fatal — the user may be starting on a broken file.
	if err := c.Convert(in, out); err != nil {
		fmt.Fprintf(os.Stderr, "✗ %s: %v\n", in, err)
	} else {
		fmt.Printf("✓ Generated: %s\n", out)
	}
	fmt.Printf("watching %s → %s (Ctrl+C to stop)\n", in, out)

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
