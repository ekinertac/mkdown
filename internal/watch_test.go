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
