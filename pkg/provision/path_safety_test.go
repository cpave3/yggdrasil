package provision

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestValidatePattern_AcceptsValidPatterns verifies that normal relative
// patterns are accepted.
func TestValidatePattern_AcceptsValidPatterns(t *testing.T) {
	valid := []string{
		".env",
		".env.*",
		"config/appsettings.json",
		"certs/dev.pem",
		"node_modules",
		"*.config",
	}
	for _, pattern := range valid {
		t.Run(pattern, func(t *testing.T) {
			err := ValidatePattern(pattern)
			assert.NoError(t, err)
		})
	}
}

// TestValidatePattern_RejectsAbsolutePath verifies that patterns starting
// with / are rejected to prevent reading/writing outside the worktree.
func TestValidatePattern_RejectsAbsolutePath(t *testing.T) {
	invalid := []string{
		"/etc/passwd",
		"/tmp/.env",
		"/",
	}
	for _, pattern := range invalid {
		t.Run(pattern, func(t *testing.T) {
			err := ValidatePattern(pattern)
			require.Error(t, err)
			assert.Contains(t, err.Error(), "absolute")
		})
	}
}

// TestValidatePattern_RejectsTraversal verifies that patterns containing ..
// are rejected to prevent path traversal attacks.
func TestValidatePattern_RejectsTraversal(t *testing.T) {
	invalid := []string{
		"../.ssh/id_rsa",
		"../../etc/passwd",
		"config/../../../secret",
		"./../escape",
	}
	for _, pattern := range invalid {
		t.Run(pattern, func(t *testing.T) {
			err := ValidatePattern(pattern)
			require.Error(t, err)
			assert.Contains(t, err.Error(), "traversal")
		})
	}
}

// TestResolveWithinPrimary_AcceptsContainedPath verifies that paths within
// the primary worktree are accepted.
func TestResolveWithinPrimary_AcceptsContainedPath(t *testing.T) {
	primary := t.TempDir()
	relPath := ".env"
	resolved, err := ResolveWithinPrimary(primary, relPath)
	require.NoError(t, err)
	assert.Equal(t, filepath.Join(primary, relPath), resolved)
}

// TestResolveWithinPrimary_RejectsTraversal verifies that paths attempting to
// escape the primary worktree are rejected.
func TestResolveWithinPrimary_RejectsTraversal(t *testing.T) {
	primary := t.TempDir()
	escape := []string{
		"../.ssh/id_rsa",
		"../../etc/passwd",
		"subdir/../../escape",
	}
	for _, relPath := range escape {
		t.Run(relPath, func(t *testing.T) {
			_, err := ResolveWithinPrimary(primary, relPath)
			require.Error(t, err)
			assert.Contains(t, err.Error(), "outside primary")
		})
	}
}

// TestResolveWithinPrimary_RejectsAbsolute verifies that absolute paths are
// rejected.
func TestResolveWithinPrimary_RejectsAbsolute(t *testing.T) {
	primary := t.TempDir()
	_, err := ResolveWithinPrimary(primary, "/etc/passwd")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "absolute")
}

// TestCopyFile_RejectsSymlink verifies that copying a file that is actually a
// symlink fails instead of following the symlink.
func TestCopyFile_RejectsSymlink(t *testing.T) {
	primary := t.TempDir()
	target := t.TempDir()

	// Create a real file outside primary
	outsideFile := filepath.Join(primary, "..", "secret.txt")
	require.NoError(t, os.WriteFile(outsideFile, []byte("secret"), 0644))

	// Create a symlink inside primary pointing to the outside file
	symlinkPath := filepath.Join(primary, "evil.env")
	require.NoError(t, os.Symlink(outsideFile, symlinkPath))

	err := CopyFile(symlinkPath, filepath.Join(target, "evil.env"))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "symlink")
}

// TestCopyFile_CopiesRegularFile verifies that copying a regular file works.
func TestCopyFile_CopiesRegularFile(t *testing.T) {
	primary := t.TempDir()
	target := t.TempDir()

	srcFile := filepath.Join(primary, ".env")
	require.NoError(t, os.WriteFile(srcFile, []byte("KEY=value"), 0644))

	dstFile := filepath.Join(target, ".env")
	err := CopyFile(srcFile, dstFile)
	require.NoError(t, err)

	content, err := os.ReadFile(dstFile)
	require.NoError(t, err)
	assert.Equal(t, "KEY=value", string(content))
}

// TestLinkFile_RejectsTargetOutsidePrimary verifies that linking to a target
// outside the primary worktree is rejected.
func TestLinkFile_RejectsTargetOutsidePrimary(t *testing.T) {
	primary := t.TempDir()
	target := t.TempDir()

	// Create a file outside primary
	outsideFile := filepath.Join(primary, "..", "outside.txt")
	require.NoError(t, os.WriteFile(outsideFile, []byte("data"), 0644))

	_, err := LinkFile(outsideFile, primary, filepath.Join(target, "link.txt"))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "outside primary")
}

// TestLinkFile_CreatesSymlink verifies that linking creates a valid symlink
// to a file inside primary.
func TestLinkFile_CreatesSymlink(t *testing.T) {
	primary := t.TempDir()
	target := t.TempDir()

	srcFile := filepath.Join(primary, "cert.pem")
	require.NoError(t, os.WriteFile(srcFile, []byte("cert"), 0644))

	linkPath, err := LinkFile(srcFile, primary, filepath.Join(target, "cert.pem"))
	require.NoError(t, err)

	// Verify it's a symlink
	info, err := os.Lstat(linkPath)
	require.NoError(t, err)
	assert.True(t, info.Mode()&os.ModeSymlink != 0, "expected symlink")

	// Verify the link resolves to the source
	resolved, err := os.Readlink(linkPath)
	require.NoError(t, err)
	assert.Equal(t, srcFile, resolved)
}
