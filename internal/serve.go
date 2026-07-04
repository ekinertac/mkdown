// serve.go — `mkdown serve`: live-preview HTTP server with a right-side version
// history sidebar (sub-project A1, live versions only).
//
// The server serves a shell page (iframe + sidebar) at "/". The live document
// and every past snapshot of this editing session live in an in-memory,
// mutex-guarded versionStore; a poll goroutine (reusing watch.go's statSig)
// re-renders on each file change and appends a version, deduping no-op saves.
// The shell's JS fetches /__versions, polls /__mtime, and reloads only the
// iframe when following the live head. Content served to the iframe is the
// pristine Converter.Render output — no injection — so it equals the standalone
// output. Localhost-only. Wired up from cmd/mkdown/main.go; see
// docs/superpowers/specs/2026-07-04-serve-live-version-history-design.md.
package internal

import (
	"bytes"
	"context"
	_ "embed"
	"encoding/json"
	"fmt"
	"html"
	"net"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"
)

//go:embed serve_assets/shell.html
var shellHTML []byte

// Serve renders in and serves it on a localhost port with a version-history
// sidebar, appending a version on every content change and signalling the shell
// to reload the preview, until ctx is cancelled. If ready is non-nil it is
// called once with the base URL after the listener is bound (main opens the
// browser; tests learn the port). Change detection polls modtime+size every
// interval, the same mechanism as Watch.
func Serve(ctx context.Context, c *Converter, in string, interval time.Duration, ready func(url string)) error {
	// Snapshot the file signature before the initial render so a save racing
	// that render is still caught on the next tick (at worst one redundant
	// render, never a missed one). Mirrors the same fix in Watch.
	mod, size := statSig(in)

	store := newVersionStore()
	store.add(renderContent(c, in), "opened") // version 1

	mux := http.NewServeMux()

	// The shell (sidebar + iframe). Root path only; everything unknown 404s.
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/" {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Write(shellHTML)
	})

	// The current live document (pristine), loaded by the iframe when following.
	mux.HandleFunc("/__content", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Write(store.head())
	})

	// A specific snapshot's document (pristine).
	mux.HandleFunc("/__version/", func(w http.ResponseWriter, r *http.Request) {
		id, err := strconv.Atoi(strings.TrimPrefix(r.URL.Path, "/__version/"))
		if err != nil {
			http.NotFound(w, r)
			return
		}
		b, ok := store.get(id)
		if !ok {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Write(b)
	})

	// The version timeline for the sidebar (metadata only, newest-first).
	mux.HandleFunc("/__versions", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(store.metaList())
	})

	// The live head id — bumps only on a real content change (dedup).
	mux.HandleFunc("/__mtime", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain")
		fmt.Fprintf(w, "%d", store.headID())
	})

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return err
	}
	if ready != nil {
		ready("http://" + ln.Addr().String())
	}

	server := &http.Server{Handler: mux}

	// Poll for file changes and append a version on each real change.
	go func() {
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
				store.add(renderContent(c, in), "edit")
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

// renderContent returns the pristine rendered document for in — Converter.Render
// output, or a minimal standalone error page on failure. No reload script is
// injected: the shell handles reloading (it reloads the iframe). The bytes here
// equal what `mkdown in` would write to disk.
func renderContent(c *Converter, in string) []byte {
	out, err := c.Render(in)
	if err != nil {
		return []byte("<!DOCTYPE html><html><head><meta charset=\"utf-8\"><title>mkdown error</title></head><body><pre>" +
			html.EscapeString(err.Error()) + "</pre></body></html>")
	}
	return out
}

// version is one snapshot of the document during the serve session.
type version struct {
	id   int
	kind string // "opened" | "edit"
	ts   time.Time
	html []byte
}

// versionMeta is the JSON shape sent to the sidebar (no html).
type versionMeta struct {
	ID    int    `json:"id"`
	Kind  string `json:"kind"`
	Label string `json:"label"`
}

// versionStore is the concurrency-safe ordered list of session versions, read
// by the HTTP handlers and written by the poll goroutine.
type versionStore struct {
	mu       sync.RWMutex
	versions []*version
	nextID   int
}

func newVersionStore() *versionStore {
	return &versionStore{nextID: 1}
}

// add appends a new version with the given rendered html and kind, unless html
// is identical to the current head (dedup — a save that didn't change the
// output adds nothing). Returns true if a version was appended.
func (s *versionStore) add(htmlBytes []byte, kind string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	if n := len(s.versions); n > 0 && bytes.Equal(s.versions[n-1].html, htmlBytes) {
		return false
	}
	s.versions = append(s.versions, &version{id: s.nextID, kind: kind, ts: time.Now(), html: htmlBytes})
	s.nextID++
	return true
}

// head returns the current (newest) version's html.
func (s *versionStore) head() []byte {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if len(s.versions) == 0 {
		return nil
	}
	return s.versions[len(s.versions)-1].html
}

// headID returns the current (newest) version's id, or 0 if empty.
func (s *versionStore) headID() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if len(s.versions) == 0 {
		return 0
	}
	return s.versions[len(s.versions)-1].id
}

// get returns the html for the version with the given id.
func (s *versionStore) get(id int) ([]byte, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	for _, v := range s.versions {
		if v.id == id {
			return v.html, true
		}
	}
	return nil, false
}

// metaList returns version metadata newest-first for the sidebar.
func (s *versionStore) metaList() []versionMeta {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]versionMeta, 0, len(s.versions))
	for i := len(s.versions) - 1; i >= 0; i-- {
		v := s.versions[i]
		out = append(out, versionMeta{ID: v.id, Kind: v.kind, Label: v.ts.Format("15:04:05")})
	}
	return out
}
