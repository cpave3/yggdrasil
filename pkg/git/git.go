// Package git provides a thin wrapper around the git binary for worktree
// operations. It shells out to the user's git as the source of truth.
package git

import (
	"fmt"
	"os/exec"
	"path/filepath"
	"strings"
)

// Runner wraps git subprocess calls for a specific repository.
type Runner struct {
	repoPath string
}

// New creates a git Runner for the given repository path.
func New(repoPath string) *Runner {
	return &Runner{repoPath: repoPath}
}

// run executes a git command in the repo and returns trimmed stdout.
func (r *Runner) run(args ...string) (string, error) {
	cmd := exec.Command("git", args...)
	cmd.Dir = r.repoPath
	output, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("git %s: %w\n%s", strings.Join(args, " "), err, output)
	}
	return strings.TrimSpace(string(output)), nil
}

// CommonDir returns the absolute path to the git common directory.
func (r *Runner) CommonDir() (string, error) {
	out, err := r.run("rev-parse", "--git-common-dir")
	if err != nil {
		return "", err
	}
	if !filepath.IsAbs(out) {
		out = filepath.Join(r.repoPath, out)
	}
	return filepath.Abs(out)
}

// Worktree represents a single git worktree entry.
type Worktree struct {
	Path   string
	Head   string
	Branch string
	Bare   bool
	Locked bool
}

// WorktreeList returns all worktrees for the repository.
func (r *Runner) WorktreeList() ([]Worktree, error) {
	out, err := r.run("worktree", "list", "--porcelain")
	if err != nil {
		return nil, err
	}

	var worktrees []Worktree
	var current *Worktree
	for _, line := range strings.Split(out, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			if current != nil {
				worktrees = append(worktrees, *current)
				current = nil
			}
			continue
		}

		if strings.HasPrefix(line, "worktree ") {
			current = &Worktree{Path: strings.TrimPrefix(line, "worktree ")}
		} else if current != nil {
			switch {
			case strings.HasPrefix(line, "HEAD "):
				current.Head = strings.TrimPrefix(line, "HEAD ")
			case strings.HasPrefix(line, "branch "):
				current.Branch = strings.TrimPrefix(line, "branch refs/heads/")
			case line == "bare":
				current.Bare = true
			case strings.HasPrefix(line, "locked"):
				current.Locked = true
			}
		}
	}
	if current != nil {
		worktrees = append(worktrees, *current)
	}

	return worktrees, nil
}

// WorktreeAdd creates a worktree at the given path for the given branch,
// optionally based on a specific start point. If the branch doesn't exist
// yet, it is created.
func (r *Runner) WorktreeAdd(branch, path, base string) error {
	args := []string{"worktree", "add", "-b", branch, path}
	if base != "" {
		args = append(args, base)
	}
	_, err := r.run(args...)
	return err
}

// WorktreeAddExisting adds a worktree for an existing branch without creating
// a new branch.
func (r *Runner) WorktreeAddExisting(branch, path string) error {
	_, err := r.run("worktree", "add", path, branch)
	return err
}

// WorktreeRemove removes a worktree at the given path. If force is true,
// removes even if the worktree contains modified or untracked files.
func (r *Runner) WorktreeRemove(path string, force bool) error {
	args := []string{"worktree", "remove"}
	if force {
		args = append(args, "--force")
	}
	args = append(args, path)
	_, err := r.run(args...)
	return err
}

// WorktreePrune cleans up stale worktree administrative entries (e.g. after
// a worktree directory was removed out-of-band). It is the equivalent of
// `git worktree prune`.
func (r *Runner) WorktreePrune() error {
	_, err := r.run("worktree", "prune")
	return err
}

// WorktreeLock locks a worktree, preventing it from being pruned.
func (r *Runner) WorktreeLock(path, reason string) error {
	args := []string{"worktree", "lock", path}
	if reason != "" {
		args = append(args, "--reason", reason)
	}
	_, err := r.run(args...)
	return err
}

// WorktreeUnlock unlocks a worktree, allowing it to be pruned.
func (r *Runner) WorktreeUnlock(path string) error {
	_, err := r.run("worktree", "unlock", path)
	return err
}

// ShowToplevel returns the absolute root path of the working tree that
// contains the Runner's directory. Unlike os.Getwd() it canonicalises
// subdirectories to the worktree root, which is what WorktreeList reports.
// Returns an error if the directory is not inside a git worktree.
func (r *Runner) ShowToplevel() (string, error) {
	out, err := r.run("rev-parse", "--show-toplevel")
	if err != nil {
		return "", err
	}
	return out, nil
}

// DetectTrunk returns the trunk branch name. It tries origin/HEAD first,
// then falls back to the current branch.
func (r *Runner) DetectTrunk() (string, error) {
	out, err := r.run("rev-parse", "--abbrev-ref", "origin/HEAD")
	if err == nil && out != "" && out != "origin/HEAD" {
		// origin/HEAD is a symbolic ref like "origin/main"
		return strings.TrimPrefix(out, "origin/"), nil
	}
	// Fall back to current branch
	return r.run("rev-parse", "--abbrev-ref", "HEAD")
}

// BranchExists returns true if the given branch exists.
func (r *Runner) BranchExists(name string) (bool, error) {
	_, err := r.run("rev-parse", "--verify", "refs/heads/"+name)
	if err != nil {
		return false, nil
	}
	return true, nil
}

// CreateBranch creates a new branch from the given base point without
// checking it out.
func (r *Runner) CreateBranch(name, base string) error {
	_, err := r.run("branch", name, base)
	return err
}

// DeleteBranch deletes a branch ref. If force is true, uses -D instead of -d.
func (r *Runner) DeleteBranch(name string, force bool) error {
	flag := "-d"
	if force {
		flag = "-D"
	}
	_, err := r.run("branch", flag, name)
	return err
}

// IsMerged returns true if branch is fully merged into target.
func (r *Runner) IsMerged(branch, target string) (bool, error) {
	out, err := r.run("merge-base", "--is-ancestor", branch, target)
	if err != nil {
		return false, nil
	}
	_ = out
	return true, nil
}

// IsDirty returns true if the worktree at the given path has uncommitted changes.
func (r *Runner) IsDirty(path string) (bool, error) {
	cmd := exec.Command("git", "status", "--porcelain")
	cmd.Dir = path
	output, err := cmd.CombinedOutput()
	if err != nil {
		return false, fmt.Errorf("git status in %s: %w\n%s", path, err, output)
	}
	return len(strings.TrimSpace(string(output))) > 0, nil
}

// AheadBehind returns the number of commits a branch is ahead and behind
// relative to the trunk.
func (r *Runner) AheadBehind(branch, trunk string) (ahead, behind int, err error) {
	out, err := r.run("rev-list", "--left-right", "--count", trunk+"..."+branch)
	if err != nil {
		return 0, 0, fmt.Errorf("counting ahead/behind: %w", err)
	}

	_, err = fmt.Sscanf(out, "%d %d", &behind, &ahead)
	if err != nil {
		return 0, 0, fmt.Errorf("parsing ahead/behind output %q: %w", out, err)
	}

	return ahead, behind, nil
}
