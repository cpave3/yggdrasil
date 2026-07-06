// Package testutil provides test helpers for yg.
package testutil

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

// GitRepo is a test helper for managing temporary git repositories.
type GitRepo struct {
	Dir    string
	origin string
}

// NewGitRepo creates a new temporary git repository with an initial commit.
func NewGitRepo(t *testing.T) *GitRepo {
	t.Helper()
	tmpDir := t.TempDir()

	runGit(t, tmpDir, "init", "-b", "main")
	runGit(t, tmpDir, "config", "user.email", "test@example.com")
	runGit(t, tmpDir, "config", "user.name", "Test User")

	repo := &GitRepo{Dir: tmpDir}

	// Create an initial commit so HEAD exists
	repo.WriteFile(t, "README.md", "# Test Repo\n")
	repo.Commit(t, "initial commit")

	return repo
}

func runGit(t *testing.T, dir string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	cmd.Env = gitEnv(dir)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git %v in %s failed: %v\nOutput: %s", args, dir, err, out)
	}
}

func gitEnv(dir string) []string {
	env := os.Environ()
	var newEnv []string
	for _, e := range env {
		if e != "HOME=" && !startsWith(e, "HOME=") {
			newEnv = append(newEnv, e)
		}
	}
	newEnv = append(newEnv, "HOME="+dir, "GIT_TERMINAL_PROMPT=0")
	return newEnv
}

func startsWith(s, prefix string) bool {
	return len(s) >= len(prefix) && s[:len(prefix)] == prefix
}

// WriteFile creates or overwrites a file in the repo.
func (r *GitRepo) WriteFile(t *testing.T, name, content string) {
	t.Helper()
	path := filepath.Join(r.Dir, name)
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		t.Fatalf("mkdir for %s: %v", name, err)
	}
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatalf("write %s: %v", name, err)
	}
}

// Commit stages all changes and commits.
func (r *GitRepo) Commit(t *testing.T, message string) {
	t.Helper()
	runGit(t, r.Dir, "add", ".")
	runGit(t, r.Dir, "commit", "-m", message)
}

// RunGit runs an arbitrary git command and returns trimmed stdout.
func (r *GitRepo) RunGit(args ...string) (string, error) {
	cmd := exec.Command("git", args...)
	cmd.Dir = r.Dir
	cmd.Env = gitEnv(r.Dir)
	out, err := cmd.CombinedOutput()
	return string(out), err
}

// AddRemote creates a bare repo in a temp dir and adds it as "origin".
func (r *GitRepo) AddRemote(t *testing.T) {
	t.Helper()
	bareDir := t.TempDir()
	runGit(t, bareDir, "init", "--bare", "-b", "main")
	r.origin = bareDir
	runGit(t, r.Dir, "remote", "add", "origin", bareDir)
	runGit(t, r.Dir, "push", "-u", "origin", "main")
}

// OriginDir returns the path to the bare origin repo (empty if no remote).
func (r *GitRepo) OriginDir() string {
	return r.origin
}
