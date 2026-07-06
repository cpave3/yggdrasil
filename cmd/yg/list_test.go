package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestList_ShowsAllWorktrees verifies that `yg list` shows all worktrees
// with their branch and path.
func TestList_ShowsAllWorktrees(t *testing.T) {
	binary := buildBinary(t)
	repoDir := initTestRepo(t)

	require.NoError(t, os.WriteFile(filepath.Join(repoDir, ".yggdrasil.toml"), []byte(`
[general]
path_template = "../{repo}.{branch}"
`), 0644))

	// Create a worktree
	createCmd := exec.Command(binary, "new", "feature-a")
	createCmd.Dir = repoDir
	createCmd.Env = append(os.Environ(), "HOME="+t.TempDir())
	require.NoError(t, createCmd.Run())

	// Run list
	listCmd := exec.Command(binary, "list")
	listCmd.Dir = repoDir
	listCmd.Env = append(os.Environ(), "HOME="+t.TempDir())
	out, err := listCmd.CombinedOutput()
	require.NoError(t, err, "yg list failed: %s", string(out))

	output := string(out)
	assert.Contains(t, output, "main", "expected to see trunk branch")
	assert.Contains(t, output, "feature-a", "expected to see feature-a branch")
	assert.Contains(t, output, repoDir, "expected to see primary path")
}

// TestList_ShowsDirtyStatus verifies that a dirty worktree shows dirty status.
func TestList_ShowsDirtyStatus(t *testing.T) {
	binary := buildBinary(t)
	repoDir := initTestRepo(t)

	require.NoError(t, os.WriteFile(filepath.Join(repoDir, ".yggdrasil.toml"), []byte(`
[general]
path_template = "../{repo}.{branch}"
`), 0644))

	// Create a worktree
	createCmd := exec.Command(binary, "new", "feature-b")
	createCmd.Dir = repoDir
	createCmd.Env = append(os.Environ(), "HOME="+t.TempDir())
	require.NoError(t, createCmd.Run())

	// Make the worktree dirty
	wtPath := filepath.Join(filepath.Dir(repoDir), filepath.Base(repoDir)+".feature-b")
	require.NoError(t, os.WriteFile(filepath.Join(wtPath, "dirty.txt"), []byte("dirty"), 0644))

	// Run list
	listCmd := exec.Command(binary, "list")
	listCmd.Dir = repoDir
	listCmd.Env = append(os.Environ(), "HOME="+t.TempDir())
	out, err := listCmd.CombinedOutput()
	require.NoError(t, err, "yg list failed: %s", string(out))

	output := string(out)
	assert.Contains(t, output, "dirty", "expected dirty status for modified worktree")
}

// TestList_ShowsLockedStatus verifies that a locked (agent-owned) worktree
// shows locked status.
func TestList_ShowsLockedStatus(t *testing.T) {
	binary := buildBinary(t)
	repoDir := initTestRepo(t)

	require.NoError(t, os.WriteFile(filepath.Join(repoDir, ".yggdrasil.toml"), []byte(`
[general]
path_template = "../{repo}.{branch}"
`), 0644))

	// Create an agent-owned worktree (locked)
	createCmd := exec.Command(binary, "new", "--agent-owned", "agent-feat")
	createCmd.Dir = repoDir
	createCmd.Env = append(os.Environ(), "HOME="+t.TempDir())
	require.NoError(t, createCmd.Run())

	// Run list
	listCmd := exec.Command(binary, "list")
	listCmd.Dir = repoDir
	listCmd.Env = append(os.Environ(), "HOME="+t.TempDir())
	out, err := listCmd.CombinedOutput()
	require.NoError(t, err, "yg list failed: %s", string(out))

	output := string(out)
	assert.Contains(t, output, "locked", "expected locked status for agent-owned worktree")
}
