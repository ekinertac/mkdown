// gitlog.go — read a file's git history by shelling out to `git`, for the serve
// version-history sidebar (sub-project A2). Read-only; every failure mode (no
// git on PATH, not a repo, file untracked, hung git) degrades to "no history"
// rather than an error, so the sidebar simply omits the git section. See
// docs/superpowers/specs/2026-07-05-serve-git-version-history-design.md.
package internal

import (
	"context"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

// gitFieldSep is the ASCII unit separator used between git-log format fields, so
// commit subjects containing any printable character parse unambiguously.
const gitFieldSep = "\x1f"

// gitCommit is one commit that touched the file (newest-first from git log).
type gitCommit struct {
	sha     string // full sha (for `git show`)
	short   string // abbreviated sha (for display)
	subject string
	date    time.Time
}

// gitHistory returns the commits that touched file, newest-first, plus the
// file's repo-relative path (used later by gitShow). It returns (nil, "") when
// git is unavailable, the path isn't in a repo, or the file isn't tracked — the
// caller treats that as "no git history" and shows the live section only.
func gitHistory(file string) (commits []gitCommit, relPath string) {
	dir := filepath.Dir(file)
	base := filepath.Base(file)

	// Repo-relative path; also confirms the file is tracked in a git repo.
	rel, err := runGit(dir, "ls-files", "--full-name", "--", base)
	if err != nil {
		return nil, ""
	}
	rel = firstLine(strings.TrimSpace(rel))
	if rel == "" {
		return nil, ""
	}

	out, err := runGit(dir, "log",
		"--format=%H"+gitFieldSep+"%h"+gitFieldSep+"%s"+gitFieldSep+"%cI", "--", base)
	if err != nil {
		return nil, rel
	}
	for _, line := range strings.Split(strings.TrimRight(out, "\n"), "\n") {
		if line == "" {
			continue
		}
		f := strings.SplitN(line, gitFieldSep, 4)
		if len(f) != 4 {
			continue
		}
		ts, _ := time.Parse(time.RFC3339, f[3])
		commits = append(commits, gitCommit{sha: f[0], short: f[1], subject: f[2], date: ts})
	}
	return commits, rel
}

// gitShow returns the file's raw bytes at the given commit (relPath is
// repo-relative, as returned by gitHistory).
func gitShow(dir, sha, relPath string) ([]byte, error) {
	return runGitBytes(dir, "show", sha+":"+relPath)
}

// runGit runs a git subcommand in dir and returns stdout as a string.
func runGit(dir string, args ...string) (string, error) {
	b, err := runGitBytes(dir, args...)
	return string(b), err
}

// runGitBytes runs `git -C dir args...` with a timeout and returns stdout.
func runGitBytes(dir string, args ...string) ([]byte, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, "git", append([]string{"-C", dir}, args...)...)
	return cmd.Output()
}

func firstLine(s string) string {
	if i := strings.IndexByte(s, '\n'); i != -1 {
		return s[:i]
	}
	return s
}
