// Package provision implements the file provisioning logic that copies and
// links files from the primary worktree into a new worktree.
package provision

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

// ValidatePattern checks a provisioning pattern for path safety violations.
// It rejects absolute paths and ../ traversal (FR-12).
func ValidatePattern(pattern string) error {
	if filepath.IsAbs(pattern) {
		return fmt.Errorf("absolute paths are not allowed: %s", pattern)
	}

	// Reject any path component that is ".."
	clean := filepath.Clean(pattern)
	for _, part := range filepath.SplitList(clean) {
		if part == ".." {
			return fmt.Errorf("path traversal is not allowed: %s", pattern)
		}
	}

	// Also reject if the cleaned path starts with ..
	if strings.HasPrefix(clean, "..") {
		return fmt.Errorf("path traversal is not allowed: %s", pattern)
	}

	// Reject if the original pattern contains ".." segments anywhere
	parts := strings.Split(pattern, string(filepath.Separator))
	for _, part := range parts {
		if part == ".." {
			return fmt.Errorf("path traversal is not allowed: %s", pattern)
		}
	}

	return nil
}

// ResolveWithinPrimary resolves a relative path against the primary worktree
// root and ensures the result stays within it. Returns an error if the path
// would escape the primary worktree (FR-12).
func ResolveWithinPrimary(primaryRoot, relPath string) (string, error) {
	if filepath.IsAbs(relPath) {
		return "", fmt.Errorf("absolute paths are not allowed: %s", relPath)
	}

	resolved := filepath.Join(primaryRoot, relPath)
	primaryAbs, err := filepath.Abs(primaryRoot)
	if err != nil {
		return "", fmt.Errorf("resolving primary root: %w", err)
	}

	rel, err := filepath.Rel(primaryAbs, resolved)
	if err != nil {
		return "", fmt.Errorf("computing relative path: %w", err)
	}

	if strings.HasPrefix(rel, "..") {
		return "", fmt.Errorf("path outside primary worktree: %s", relPath)
	}

	return resolved, nil
}

// CopyFile copies a single file from src to dst. It fails if src is a symlink
// (FR-12: copy operations do not follow symlinks).
func CopyFile(src, dst string) error {
	info, err := os.Lstat(src)
	if err != nil {
		return fmt.Errorf("stat %s: %w", src, err)
	}

	if info.Mode()&os.ModeSymlink != 0 {
		return fmt.Errorf("refusing to copy symlink: %s", src)
	}

	if !info.Mode().IsRegular() {
		return fmt.Errorf("not a regular file: %s", src)
	}

	if err := os.MkdirAll(filepath.Dir(dst), 0755); err != nil {
		return fmt.Errorf("creating destination directory: %w", err)
	}

	srcFile, err := os.Open(src)
	if err != nil {
		return fmt.Errorf("opening %s: %w", src, err)
	}
	defer srcFile.Close()

	dstFile, err := os.Create(dst)
	if err != nil {
		return fmt.Errorf("creating %s: %w", dst, err)
	}
	defer dstFile.Close()

	if _, err := io.Copy(dstFile, srcFile); err != nil {
		return fmt.Errorf("copying %s to %s: %w", src, dst, err)
	}

	return nil
}

// LinkFile creates a symlink at linkPath pointing to srcFile. It verifies
// that srcFile is within primaryRoot (FR-12: link targets must resolve within
// the primary worktree). Returns the absolute link path.
func LinkFile(srcFile, primaryRoot, linkPath string) (string, error) {
	srcAbs, err := filepath.Abs(srcFile)
	if err != nil {
		return "", fmt.Errorf("resolving source: %w", err)
	}

	primaryAbs, err := filepath.Abs(primaryRoot)
	if err != nil {
		return "", fmt.Errorf("resolving primary root: %w", err)
	}

	rel, err := filepath.Rel(primaryAbs, srcAbs)
	if err != nil {
		return "", fmt.Errorf("computing relative path: %w", err)
	}

	if strings.HasPrefix(rel, "..") {
		return "", fmt.Errorf("link target outside primary worktree: %s", srcFile)
	}

	if err := os.MkdirAll(filepath.Dir(linkPath), 0755); err != nil {
		return "", fmt.Errorf("creating link directory: %w", err)
	}

	if err := os.Symlink(srcAbs, linkPath); err != nil {
		return "", fmt.Errorf("creating symlink: %w", err)
	}

	return linkPath, nil
}
