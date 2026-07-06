package worktree

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestResolvePath_SymlinkedPrimaryDir verifies that when the primary dir is
// accessed through a symlink (e.g. /home → /var/home on btrfs), the resolved
// worktree path uses the real path so it matches what git reports.
func TestResolvePath_SymlinkedPrimaryDir(t *testing.T) {
	// Create a real dir, a symlink to it, and resolve through the symlink
	realDir := t.TempDir()
	symlinkDir := filepath.Join(t.TempDir(), "link")
	require.NoError(t, os.Symlink(realDir, symlinkDir))

	// Create the repo dir inside realDir so {repo} resolves correctly
	repoName := "myproject"
	realRepo := filepath.Join(realDir, repoName)
	require.NoError(t, os.MkdirAll(realRepo, 0755))

	// Access through symlink
	symlinkRepo := filepath.Join(symlinkDir, repoName)

	path, err := ResolvePath("../{repo}.{branch}", symlinkRepo, "feature-x")
	require.NoError(t, err)

	// The resolved path should be in the real dir, not the symlink dir
	expected := filepath.Join(realDir, "myproject.feature-x")
	assert.Equal(t, expected, path,
		"resolved path should use real (EvalSymlinks) path, not symlink path")
}
