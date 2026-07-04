# Design: `serve` git version history (sub-project A2)

**Date:** 2026-07-05
**Status:** Approved (design), pending implementation
**Component:** `internal` (converter + serve layer)

## Summary

Extend the `mkdown serve` version sidebar (built in A1) with the file's **git
history**: at startup, read the commits that touched the file and add them to
the same version store, below the live editing versions. Clicking a git version
renders and shows the file as it was at that commit. Git history is paginated
(25 at a time, "Load more"). Serve-only; standalone `mkdown file.md` output is
unaffected.

```
mkdown serve readme.md
→ sidebar: [ live edits… ] [ opened ] then git history:
    a1b2c3d · Fix parser · 3d ago
    9f8e7d6 · Add tables · 5d ago
    … (25 shown)  [ Load 25 more ]
[click a git row] → preview shows the file at that commit
```

## Decisions (settled in brainstorming)

1. **Shell out to `git`** (decided in A1) — no new Go dependency. The feature
   only activates when the file is git-tracked, so `git` on PATH is a safe
   precondition; otherwise the git section is simply absent.
2. **Lazy rendering.** Startup runs one `git log` (metadata only). A git
   version's content (`git show` + render) is produced on first view and cached.
3. **Paginate at 25** with a "Load more" button.
4. **No `--follow`** (rename tracking) in v1 — keeps path resolution simple;
   history truncates at a rename. Noted as future.
5. **Git-row content:** short sha · commit subject · relative date.

## Architecture

### `Converter.RenderBytes` — `internal/converter.go`

Git content is bytes, not a file on disk. Add:

```go
// RenderBytes renders markdown source bytes to a full standalone HTML document,
// without reading or writing any file.
func (c *Converter) RenderBytes(source []byte) ([]byte, error)
```

Refactor: `buildDocument` currently reads the file. Split it so the shared work
takes `source []byte`:
- `buildDocumentFromSource(source []byte) (*Document, error)` — frontmatter →
  math → markdown.Convert → math restore → inject scripts (the existing body).
- `Render(inputPath)` and `Convert(inputPath, out)` read the file, then call it.
- `RenderBytes(source)` calls it directly, then executes the template into a
  buffer (like `Render`).

No behavior change to `Render`/`Convert`; existing tests still pass.

### Git reader — new `internal/gitlog.go`

All git commands run with the file's directory as cwd (`git -C <dir>`). Read
once at startup:

1. **Repo-relative path:** `git ls-files --full-name -- <basename>` (run in the
   file's dir). Empty output / non-zero exit / `git` not found → **no git
   history** (return an empty list; the sidebar shows the live section only).
2. **Commit list:** `git log --format=<sha>%x1f<subject>%x1f<committer-ISO-date> -- <relpath>`
   using an ASCII unit separator (`%x1f`) between fields so subjects with any
   character parse safely; split lines, then fields. Newest-first (git's default).
3. Return a slice of `gitCommit{ sha, subject, date time.Time }`.
4. **Content on demand:** `gitShow(dir, sha, relpath) ([]byte, error)` runs
   `git show <sha>:<relpath>` and returns the raw file bytes at that commit.

Everything here is read-only shelling; a `context`/timeout guards against a
hung git. Errors from git are treated as "no history" (fallback), never fatal.

### Store population & timeline — `internal/serve.go`

The `versionStore` from A1 gains git versions. A `version` grows a lazy content
path:

```go
type version struct {
    id      int
    kind    string      // "opened" | "edit" | "git"
    ts      time.Time
    subject string      // git subject (git only)
    sha     string      // short sha (git only)
    html    []byte      // rendered snapshot; nil for a not-yet-rendered git version
    render  func() []byte  // nil for live versions; renders+caches for git
    once    sync.Once
}
```

- **Startup order:** add git versions oldest→newest first (so ids ascend with
  history), each with `kind:"git"`, its `sha`/`subject`/`ts`, `html:nil`, and a
  `render` closure (`gitShow` → `RenderBytes`, or an escaped error page on
  failure). Then add the working-tree `opened` version (rendered eagerly, as in
  A1). Then live edits append during the session.
- **Head / live:** the head (highest id) is the live head that follows edits.
  **`opened` is always added** — dedup applies only *among live versions* (`add`
  skips only when the current head is a live kind, `opened`/`edit`, with
  identical html). So git versions never suppress `opened`, and the live section
  is never empty. If the working tree equals the newest commit, you'll see both
  the live `opened` row and the identical newest-commit git row — intentional,
  it reads as "current state matches commit X".
- **Lazy get:** `store.get(id)` returns `html` if set; otherwise runs
  `version.once.Do(...)` to call `render()` (outside the store's lock so a slow
  `git show`+render doesn't block other handlers), caches into `html`, returns it.

### Endpoints — `internal/serve.go`

- `GET /__versions` → **live** versions only (kinds `opened`/`edit`); the handler
  now filters out `git` kinds, which are served via `/__git` instead. (In A1 all
  versions were live, so this is a filter added here.)
- `GET /__git?offset=N` → JSON `{ items: [ {id, sha, subject, date} … up to 25 ], more: bool }`
  — a page of git versions starting at offset N (static during the session).
- `GET /__version/{id}` → the version's rendered HTML (lazy for git). `404` if
  unknown.
- `GET /__content`, `GET /__mtime` → unchanged (live head).

### Sidebar — `internal/serve_assets/shell.html`

Two labelled sections:
- **Live** (top): rendered from `/__versions`, polled (unchanged from A1).
- **Git history** (below): rendered from `/__git?offset=0` on load; each row is
  `sha · subject · relative-date`. When the response's `more` is true, a **Load
  25 more** button fetches `/__git?offset=<next>` and appends. Clicking any git
  row sets `iframe.src = /__version/{id}` and marks it selected (same mechanism
  as live snapshots; **Back to Live** returns to following).

Relative dates ("3d ago") computed client-side from the ISO `date`.

## Error handling

- Not a repo / `git` missing / file untracked → git section absent, live-only.
- A single commit's `git show`/render failure → that version's `render` returns
  an escaped standalone error page (viewing it shows the error; the rest work).
- `/__git` with a bad `offset` → treat as 0.

## Concurrency

Git versions are all added at startup (before serving), then immutable except
for the lazy `html` cache, which is guarded by the per-version `sync.Once`. The
`render()` runs outside the store lock. Live versions unchanged. Verified under
`-race`.

## Testing

- **`RenderBytes` unit test** (`converter_test.go`): render source bytes → full
  document with the content; no file involved.
- **Git reader test** (`gitlog_test.go`): build a throwaway repo in a temp dir
  (`git init`; write + commit the file twice with different content); assert the
  reader returns two commits newest-first with correct sha/subject, and
  `gitShow` returns each commit's historical bytes. A separate case: a non-repo
  temp dir → empty list (fallback).
- **Serve integration** (`serve_test.go`): in a temp git repo with one committed
  version + an uncommitted edit, start serve; `/__git?offset=0` returns the
  commit(s); `/__version/{gitID}` renders the historical content; a `>25`-commit
  case exercises pagination (`more:true`, `offset=25` returns the next page); a
  non-repo dir → `/__git` returns an empty page. Run under `-race`.

## Out of scope (A2)

- Rename-following (`--follow`).
- Inline section diff / highlight (sub-project **B**).
- Diffing a git version against another or against live (B).
- Showing branches/tags, blame, or commit author/body beyond subject+date.
- Server pagination of the *live* list (live sections stays small).

## Future (context)

Sub-project **B** diffs any two of this store's versions (live or git) and
highlights changed regions — it consumes this store unchanged. `--follow` and
richer commit metadata are additive later.
