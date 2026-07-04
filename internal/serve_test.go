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
