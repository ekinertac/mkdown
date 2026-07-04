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
		if r.URL.Path != "/" {
			http.NotFound(w, r)
			return
		}
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
