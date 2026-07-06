// Package worktree handles worktree path resolution and orchestration.
package worktree

import (
	"fmt"
	"path/filepath"
	"strings"
)

// ResolvePath expands a path template to produce the worktree path.
// The template supports {repo} (repo basename) and {branch} (branch name).
// The result is resolved relative to the primary worktree directory.
// Absolute templates and ../ traversal beyond the parent are rejected.
func ResolvePath(template, primaryDir, branch string) (string, error) {
	// Evaluate symlinks so the path matches what git reports (which stores
	// real paths, not symlink paths). This matters on systems like btrfs
	// where /home is a symlink to /var/home. If the dir doesn't exist (e.g.
	// in unit tests with fictional paths), fall back to Abs.
	if real, err := filepath.EvalSymlinks(primaryDir); err == nil {
		primaryDir = real
	} else if real, err := filepath.Abs(primaryDir); err == nil {
		primaryDir = real
	}

	repo := filepath.Base(primaryDir)
	branchSafe := sanitizeBranchName(branch)

	result := strings.ReplaceAll(template, "{repo}", repo)
	result = strings.ReplaceAll(result, "{branch}", branchSafe)

	if filepath.IsAbs(result) {
		return "", fmt.Errorf("path template resolves to absolute path: %s", result)
	}

	// Resolve relative to primary dir
	resolved := filepath.Join(primaryDir, result)
	resolved, err := filepath.Abs(resolved)
	if err != nil {
		return "", fmt.Errorf("resolving path: %w", err)
	}

	// Check that the resolved path is within the primary's parent dir
	parentDir := filepath.Dir(primaryDir)
	parentAbs, err := filepath.Abs(parentDir)
	if err != nil {
		return "", fmt.Errorf("resolving parent: %w", err)
	}

	if !isWithinDir(resolved, parentAbs) {
		return "", fmt.Errorf("path template traversal: resolved path %s escapes parent %s", resolved, parentAbs)
	}

	return resolved, nil
}

// sanitizeBranchName replaces slashes with dashes for use in directory names.
func sanitizeBranchName(branch string) string {
	return strings.ReplaceAll(branch, "/", "-")
}

// isWithinDir checks if path is equal to or a descendant of dir.
func isWithinDir(path, dir string) bool {
	rel, err := filepath.Rel(dir, path)
	if err != nil {
		return false
	}
	return rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator))
}
