package internal

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func gitAvailable() bool {
	_, err := exec.LookPath("git")
	return err == nil
}

// initTestRepo creates a temp git repo with doc.md committed twice (different
// content each time) and returns the repo dir and the file path.
func initTestRepo(t *testing.T) (dir, file string) {
	t.Helper()
	dir = t.TempDir()
	run := func(args ...string) {
		cmd := exec.Command("git", append([]string{"-C", dir}, args...)...)
		cmd.Env = append(os.Environ(),
			"GIT_AUTHOR_NAME=t", "GIT_AUTHOR_EMAIL=t@example.com",
			"GIT_COMMITTER_NAME=t", "GIT_COMMITTER_EMAIL=t@example.com")
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}
	run("init")
	run("config", "user.email", "t@example.com")
	run("config", "user.name", "t")
	run("config", "commit.gpgsign", "false")

	file = filepath.Join(dir, "doc.md")
	if err := os.WriteFile(file, []byte("# First"), 0644); err != nil {
		t.Fatal(err)
	}
	run("add", "doc.md")
	run("commit", "-m", "first commit")
	if err := os.WriteFile(file, []byte("# Second"), 0644); err != nil {
		t.Fatal(err)
	}
	run("add", "doc.md")
	run("commit", "-m", "second commit")
	return dir, file
}

func TestGitHistory(t *testing.T) {
	if !gitAvailable() {
		t.Skip("git not on PATH")
	}
	dir, file := initTestRepo(t)

	commits, rel := gitHistory(file)
	if rel != "doc.md" {
		t.Fatalf("relPath = %q, want doc.md", rel)
	}
	if len(commits) != 2 {
		t.Fatalf("got %d commits, want 2", len(commits))
	}
	// newest-first
	if commits[0].subject != "second commit" {
		t.Fatalf("commits[0].subject = %q, want %q", commits[0].subject, "second commit")
	}
	if commits[1].subject != "first commit" {
		t.Fatalf("commits[1].subject = %q, want %q", commits[1].subject, "first commit")
	}
	if commits[0].short == "" || len(commits[0].short) >= len(commits[0].sha) {
		t.Fatalf("short sha looks wrong: short=%q sha=%q", commits[0].short, commits[0].sha)
	}

	// gitShow returns the historical bytes for each commit.
	oldest, err := gitShow(dir, commits[1].sha, rel)
	if err != nil {
		t.Fatalf("gitShow oldest: %v", err)
	}
	if strings.TrimSpace(string(oldest)) != "# First" {
		t.Fatalf("oldest content = %q, want # First", oldest)
	}
	newest, err := gitShow(dir, commits[0].sha, rel)
	if err != nil {
		t.Fatalf("gitShow newest: %v", err)
	}
	if strings.TrimSpace(string(newest)) != "# Second" {
		t.Fatalf("newest content = %q, want # Second", newest)
	}
}

func TestGitHistoryNonRepo(t *testing.T) {
	dir := t.TempDir()
	file := filepath.Join(dir, "doc.md")
	if err := os.WriteFile(file, []byte("# Hi"), 0644); err != nil {
		t.Fatal(err)
	}
	commits, _ := gitHistory(file)
	if len(commits) != 0 {
		t.Fatalf("non-repo dir should yield no commits, got %d", len(commits))
	}
}
