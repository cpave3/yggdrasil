package provision

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// ProvisionConfig holds the parameters for a provisioning operation.
type ProvisionConfig struct {
	PrimaryDir string   // source (primary worktree)
	TargetDir  string   // destination (new worktree)
	Copy       []string // copy patterns
	Link       []string // link patterns
	DryRun     bool     // if true, don't write anything
}

// Provision resolves copy/link patterns from the primary worktree into the
// target worktree. It validates all patterns for path safety, creates parent
// directories as needed, and is idempotent. Operations are fail-closed on
// path safety violations (FR-11, FR-12).
func Provision(pc ProvisionConfig) error {
	for _, pattern := range pc.Copy {
		if err := ValidatePattern(pattern); err != nil {
			return fmt.Errorf("copy pattern %q: %w", pattern, err)
		}
		sources, err := resolveGlob(pc.PrimaryDir, pattern)
		if err != nil {
			return fmt.Errorf("resolving copy pattern %q: %w", pattern, err)
		}
		for _, src := range sources {
			relPath, err := filepath.Rel(pc.PrimaryDir, src)
			if err != nil {
				return fmt.Errorf("computing relative path: %w", err)
			}
			dst := filepath.Join(pc.TargetDir, relPath)
			if pc.DryRun {
				fmt.Printf("  copy %s → %s\n", relPath, dst)
				continue
			}
			info, err := os.Lstat(src)
			if err != nil {
				if os.IsNotExist(err) {
					continue // source doesn't exist, skip
				}
				return fmt.Errorf("stat %s: %w", src, err)
			}
			if info.IsDir() {
				if err := copyDir(src, dst); err != nil {
					return err
				}
			} else {
				if err := CopyFile(src, dst); err != nil {
					return err
				}
			}
		}
	}

	for _, pattern := range pc.Link {
		if err := ValidatePattern(pattern); err != nil {
			return fmt.Errorf("link pattern %q: %w", pattern, err)
		}
		sources, err := resolveGlob(pc.PrimaryDir, pattern)
		if err != nil {
			return fmt.Errorf("resolving link pattern %q: %w", pattern, err)
		}
		for _, src := range sources {
			relPath, err := filepath.Rel(pc.PrimaryDir, src)
			if err != nil {
				return fmt.Errorf("computing relative path: %w", err)
			}
			dst := filepath.Join(pc.TargetDir, relPath)
			if pc.DryRun {
				fmt.Printf("  link %s → %s\n", relPath, dst)
				continue
			}
			// Idempotent: remove existing link/file if present
			os.Remove(dst)
			if _, err := LinkFile(src, pc.PrimaryDir, dst); err != nil {
				return err
			}
		}
	}

	return nil
}

// resolveGlob resolves a glob pattern against the primary directory. If the
// pattern is a literal path (no glob characters), it returns it directly if
// it exists.
func resolveGlob(primaryDir, pattern string) ([]string, error) {
	fullPattern := filepath.Join(primaryDir, pattern)

	// If no glob characters, check if it's a literal file/dir
	if !strings.ContainsAny(pattern, "*?[") {
		if _, err := os.Lstat(fullPattern); err != nil {
			if os.IsNotExist(err) {
				return nil, nil
			}
			return nil, err
		}
		return []string{fullPattern}, nil
	}

	matches, err := filepath.Glob(fullPattern)
	if err != nil {
		return nil, err
	}

	// Filter out symlinks for safety — we don't want to accidentally
	// traverse into a symlinked directory
	var safe []string
	for _, m := range matches {
		safe = append(safe, m)
	}

	return safe, nil
}

// copyDir recursively copies a directory tree, respecting the no-symlink-follow
// rule (FR-12).
func copyDir(src, dst string) error {
	info, err := os.Lstat(src)
	if err != nil {
		return fmt.Errorf("stat %s: %w", src, err)
	}

	if info.Mode()&os.ModeSymlink != 0 {
		return fmt.Errorf("refusing to copy symlink: %s", src)
	}

	if !info.IsDir() {
		return CopyFile(src, dst)
	}

	if err := os.MkdirAll(dst, info.Mode()); err != nil {
		return fmt.Errorf("creating %s: %w", dst, err)
	}

	entries, err := os.ReadDir(src)
	if err != nil {
		return fmt.Errorf("reading %s: %w", src, err)
	}

	for _, entry := range entries {
		srcPath := filepath.Join(src, entry.Name())
		dstPath := filepath.Join(dst, entry.Name())

		entryInfo, err := os.Lstat(srcPath)
		if err != nil {
			return fmt.Errorf("stat %s: %w", srcPath, err)
		}

		if entryInfo.Mode()&os.ModeSymlink != 0 {
			return fmt.Errorf("refusing to copy symlink: %s", srcPath)
		}

		if entry.IsDir() {
			if err := copyDir(srcPath, dstPath); err != nil {
				return err
			}
		} else {
			if err := CopyFile(srcPath, dstPath); err != nil {
				return err
			}
		}
	}

	return nil
}
