// serve.go — `mkdown serve`: live-preview HTTP server with a right-side version
// history sidebar (live edits + git history).
//
// The server serves a shell page (iframe + sidebar) at "/". Live edit versions
// and the file's git history live in an in-memory versionStore. A poll goroutine
// (reusing watch.go's statSig) re-renders on each file change and appends a live
// version, deduping no-op saves. Git versions are read once at startup (see
// gitlog.go) and rendered lazily on first view. Content served to the iframe is
// pristine Converter output — no injection — so it equals the standalone output.
// Localhost-only. See docs/superpowers/specs/2026-07-05-serve-git-version-history-design.md.
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
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"
)

//go:embed serve_assets/shell.html
var shellHTML []byte

// gitPageSize is how many git versions a /__git page returns.
const gitPageSize = 25

// Serve renders in and serves it on a localhost port with a version-history
// sidebar (live edits + git history), appending a version on every content
// change and signalling the shell to reload, until ctx is cancelled. If ready
// is non-nil it is called once with the base URL after the listener is bound.
func Serve(ctx context.Context, c *Converter, in string, interval time.Duration, ready func(url string)) error {
	// Snapshot the file signature before the initial render so a save racing
	// that render is still caught on the next tick. Mirrors the fix in Watch.
	mod, size := statSig(in)

	store := newVersionStore()
	// Git history first (oldest at the bottom of the timeline), rendered lazily.
	dir := filepath.Dir(in)
	commits, rel := gitHistory(in)
	for _, ci := range commits { // newest-first; addGit keeps that order
		ci := ci
		store.addGit(ci.short, ci.subject, ci.date, func() []byte {
			src, err := gitShow(dir, ci.sha, rel)
			if err != nil {
				return errorPage(err)
			}
			out, err := c.RenderBytes(src)
			if err != nil {
				return errorPage(err)
			}
			return out
		})
	}
	// The working-tree version — always the live head.
	store.addLive(renderContent(c, in), "opened")

	mux := http.NewServeMux()

	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/" {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Write(shellHTML)
	})
	mux.HandleFunc("/__content", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Write(store.head())
	})
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
	mux.HandleFunc("/__versions", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(store.liveMeta())
	})
	mux.HandleFunc("/__git", func(w http.ResponseWriter, r *http.Request) {
		offset, _ := strconv.Atoi(r.URL.Query().Get("offset"))
		items, more := store.gitPage(offset, gitPageSize)
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(struct {
			Items []versionMeta `json:"items"`
			More  bool          `json:"more"`
		}{items, more})
	})
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
				store.addLive(renderContent(c, in), "edit")
			}
		}
	}()

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
// output, or a standalone error page on failure. No reload script is injected;
// the shell reloads the iframe. The bytes equal what `mkdown in` writes to disk.
func renderContent(c *Converter, in string) []byte {
	out, err := c.Render(in)
	if err != nil {
		return errorPage(err)
	}
	return out
}

// errorPage is a minimal standalone page showing an escaped error message.
func errorPage(err error) []byte {
	return []byte("<!DOCTYPE html><html><head><meta charset=\"utf-8\"><title>mkdown error</title></head><body><pre>" +
		html.EscapeString(err.Error()) + "</pre></body></html>")
}

// version is one snapshot in the serve session — a live edit or a git commit.
type version struct {
	id      int
	kind    string    // "opened" | "edit" | "git"
	ts      time.Time
	sha     string    // short sha (git only)
	subject string    // commit subject (git only)
	html    []byte    // rendered bytes; nil for a not-yet-rendered git version
	render  func() []byte
	once    sync.Once
}

// versionMeta is the JSON shape sent to the sidebar (no html).
type versionMeta struct {
	ID      int    `json:"id"`
	Kind    string `json:"kind"`
	Label   string `json:"label,omitempty"`   // live: HH:MM:SS
	SHA     string `json:"sha,omitempty"`     // git
	Subject string `json:"subject,omitempty"` // git
	Date    string `json:"date,omitempty"`    // git: ISO 8601
}

// versionStore holds the session's live edits and git history, read by the HTTP
// handlers and written (live) by the poll goroutine.
type versionStore struct {
	mu       sync.RWMutex
	gitVers  []*version // newest-first, fixed after startup
	liveVers []*version // oldest -> newest, grows during the session
	byID     map[int]*version
	nextID   int
}

func newVersionStore() *versionStore {
	return &versionStore{byID: map[int]*version{}, nextID: 1}
}

// addLive appends a live version unless its html equals the current live head
// (dedup — a save that didn't change the output adds nothing). Git versions
// never suppress a live add. Returns true if appended.
func (s *versionStore) addLive(htmlBytes []byte, kind string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	if n := len(s.liveVers); n > 0 && bytes.Equal(s.liveVers[n-1].html, htmlBytes) {
		return false
	}
	v := &version{id: s.nextID, kind: kind, ts: time.Now(), html: htmlBytes}
	s.liveVers = append(s.liveVers, v)
	s.byID[v.id] = v
	s.nextID++
	return true
}

// addGit appends a git version (called newest-first at startup, so gitVers stays
// newest-first). Content renders lazily via render on first get.
func (s *versionStore) addGit(short, subject string, ts time.Time, render func() []byte) {
	s.mu.Lock()
	defer s.mu.Unlock()
	v := &version{id: s.nextID, kind: "git", ts: ts, sha: short, subject: subject, render: render}
	s.gitVers = append(s.gitVers, v)
	s.byID[v.id] = v
	s.nextID++
}

// head returns the live head's html (the current document).
func (s *versionStore) head() []byte {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if n := len(s.liveVers); n > 0 {
		return s.liveVers[n-1].html
	}
	return nil
}

// headID returns the live head's id, or 0 if there are no live versions.
func (s *versionStore) headID() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if n := len(s.liveVers); n > 0 {
		return s.liveVers[n-1].id
	}
	return 0
}

// get returns the html for the version with the given id, rendering a git
// version's content lazily (once) outside the store lock.
func (s *versionStore) get(id int) ([]byte, bool) {
	s.mu.RLock()
	v, ok := s.byID[id]
	s.mu.RUnlock()
	if !ok {
		return nil, false
	}
	if v.render != nil {
		v.once.Do(func() { v.html = v.render() })
	}
	return v.html, true
}

// liveMeta returns the live versions newest-first for the sidebar.
func (s *versionStore) liveMeta() []versionMeta {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]versionMeta, 0, len(s.liveVers))
	for i := len(s.liveVers) - 1; i >= 0; i-- {
		v := s.liveVers[i]
		out = append(out, versionMeta{ID: v.id, Kind: v.kind, Label: v.ts.Format("15:04:05")})
	}
	return out
}

// gitPage returns up to limit git versions (newest-first) starting at offset,
// plus whether more remain.
func (s *versionStore) gitPage(offset, limit int) ([]versionMeta, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if offset < 0 {
		offset = 0
	}
	total := len(s.gitVers)
	if offset >= total {
		return []versionMeta{}, false
	}
	end := offset + limit
	more := end < total
	if end > total {
		end = total
	}
	out := make([]versionMeta, 0, end-offset)
	for _, v := range s.gitVers[offset:end] {
		out = append(out, versionMeta{ID: v.id, Kind: "git", SHA: v.sha, Subject: v.subject, Date: v.ts.Format(time.RFC3339)})
	}
	return out, more
}
