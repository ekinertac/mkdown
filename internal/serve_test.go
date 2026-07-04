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

func httpStatus(t *testing.T, url string) int {
	t.Helper()
	resp, err := http.Get(url)
	if err != nil {
		t.Fatalf("GET %s: %v", url, err)
	}
	defer resp.Body.Close()
	return resp.StatusCode
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

// --- versionStore unit tests ---

func TestVersionStoreAddAndDedup(t *testing.T) {
	s := newVersionStore()
	if !s.add([]byte("<h1>a</h1>"), "opened") {
		t.Fatal("first add should append")
	}
	if s.headID() != 1 {
		t.Fatalf("headID = %d, want 1", s.headID())
	}
	// Identical content must be deduped (no new version).
	if s.add([]byte("<h1>a</h1>"), "edit") {
		t.Fatal("identical content should be deduped")
	}
	if s.headID() != 1 {
		t.Fatalf("headID after dedup = %d, want 1", s.headID())
	}
	// Different content appends.
	if !s.add([]byte("<h1>b</h1>"), "edit") {
		t.Fatal("changed content should append")
	}
	if s.headID() != 2 {
		t.Fatalf("headID = %d, want 2", s.headID())
	}
	if got := string(s.head()); got != "<h1>b</h1>" {
		t.Fatalf("head = %q, want <h1>b</h1>", got)
	}
	// Lookup by id.
	if b, ok := s.get(1); !ok || string(b) != "<h1>a</h1>" {
		t.Fatalf("get(1) = %q, %v", b, ok)
	}
	if _, ok := s.get(99); ok {
		t.Fatal("get(99) should be not found")
	}
	// metaList is newest-first.
	m := s.metaList()
	if len(m) != 2 || m[0].ID != 2 || m[1].ID != 1 {
		t.Fatalf("metaList order wrong: %+v", m)
	}
}

// --- Serve integration test ---

func TestServeVersionHistory(t *testing.T) {
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

	// "/" is the shell (iframe + sidebar), NOT the document itself.
	shell := httpGetBody(t, base+"/")
	if !strings.Contains(shell, `id="content"`) || !strings.Contains(shell, "History") {
		t.Fatalf("/ is not the shell page: %s", shell[:min2(300, len(shell))])
	}

	// The document lives at /__content.
	if body := httpGetBody(t, base+"/__content"); !strings.Contains(body, "First") {
		t.Fatalf("/__content missing initial content: %s", body)
	}
	if v := httpGetBody(t, base+"/__mtime"); v != "1" {
		t.Fatalf("initial /__mtime = %q, want 1", v)
	}

	// Edit → a new version appears; the old snapshot stays viewable.
	if err := os.WriteFile(in, []byte("# Second"), 0644); err != nil {
		t.Fatal(err)
	}
	waitForMtimeChange(t, base, "1")

	if body := httpGetBody(t, base+"/__content"); !strings.Contains(body, "Second") {
		t.Fatalf("/__content not updated: %s", body)
	}
	if body := httpGetBody(t, base+"/__version/1"); !strings.Contains(body, "First") {
		t.Fatalf("/__version/1 should still show original: %s", body)
	}
	if s := httpStatus(t, base+"/__version/999"); s != http.StatusNotFound {
		t.Fatalf("/__version/999 status = %d, want 404", s)
	}
	if s := httpStatus(t, base+"/__version/abc"); s != http.StatusNotFound {
		t.Fatalf("/__version/abc (non-numeric) status = %d, want 404", s)
	}
	if s := httpStatus(t, base+"/nope"); s != http.StatusNotFound {
		t.Fatalf("/nope status = %d, want 404", s)
	}

	// /__versions lists both, newest-first, with kinds.
	versions := httpGetBody(t, base+"/__versions")
	if !strings.Contains(versions, `"id":2`) || !strings.Contains(versions, `"id":1`) {
		t.Fatalf("/__versions missing entries: %s", versions)
	}
	if !strings.Contains(versions, `"kind":"opened"`) || !strings.Contains(versions, `"kind":"edit"`) {
		t.Fatalf("/__versions missing kinds: %s", versions)
	}

	// A save that doesn't change the rendered output must NOT add a version
	// (dedup), so /__mtime must not bump. Rewrite the same content, let many
	// poll ticks pass, and confirm the head id is unchanged.
	if err := os.WriteFile(in, []byte("# Second"), 0644); err != nil {
		t.Fatal(err)
	}
	time.Sleep(120 * time.Millisecond) // ~12 poll ticks at the 10ms interval
	if v := httpGetBody(t, base+"/__mtime"); v != "2" {
		t.Fatalf("/__mtime after a no-op save = %q, want unchanged 2", v)
	}
	if v := httpGetBody(t, base+"/__versions"); strings.Contains(v, `"id":3`) {
		t.Fatalf("a no-op save added a version: %s", v)
	}

	cancel()
	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("Serve returned error: %v", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("Serve did not return after context cancel")
	}
}

func min2(a, b int) int {
	if a < b {
		return a
	}
	return b
}
