package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestRemove_RemovesWorktree verifies that `yg remove` removes the worktree.
func TestRemove_RemovesWorktree(t *testing.T) {
	binary := buildBinary(t)
	repoDir := initTestRepo(t)

	require.NoError(t, os.WriteFile(filepath.Join(repoDir, ".yggdrasil.toml"), []byte(`
[general]
path_template = "../{repo}.{branch}"
`), 0644))

	// Create a worktree
	createCmd := exec.Command(binary, "new", "to-remove")
	createCmd.Dir = repoDir
	createCmd.Env = append(os.Environ(), "HOME="+t.TempDir())
	require.NoError(t, createCmd.Run())

	wtPath := filepath.Join(filepath.Dir(repoDir), filepath.Base(repoDir)+".to-remove")

	// Remove it
	removeCmd := exec.Command(binary, "remove", "to-remove")
	removeCmd.Dir = repoDir
	removeCmd.Env = append(os.Environ(), "HOME="+t.TempDir())
	out, err := removeCmd.CombinedOutput()
	require.NoError(t, err, "yg remove failed: %s", string(out))

	// Verify worktree is gone
	_, err = os.Stat(wtPath)
	assert.True(t, os.IsNotExist(err), "worktree directory should be removed")
}

// TestRemove_DirtyTreeWarns verifies that removing a dirty worktree fails
// without --force.
func TestRemove_DirtyTreeWarns(t *testing.T) {
	binary := buildBinary(t)
	repoDir := initTestRepo(t)

	require.NoError(t, os.WriteFile(filepath.Join(repoDir, ".yggdrasil.toml"), []byte(`
[general]
path_template = "../{repo}.{branch}"
`), 0644))

	// Create a worktree
	createCmd := exec.Command(binary, "new", "dirty-branch")
	createCmd.Dir = repoDir
	createCmd.Env = append(os.Environ(), "HOME="+t.TempDir())
	require.NoError(t, createCmd.Run())

	wtPath := filepath.Join(filepath.Dir(repoDir), filepath.Base(repoDir)+".dirty-branch")
	require.NoError(t, os.WriteFile(filepath.Join(wtPath, "uncommitted.txt"), []byte("dirty"), 0644))

	// Try to remove without --force
	removeCmd := exec.Command(binary, "remove", "dirty-branch")
	removeCmd.Dir = repoDir
	removeCmd.Env = append(os.Environ(), "HOME="+t.TempDir())
	out, err := removeCmd.CombinedOutput()
	require.Error(t, err, "expected remove to fail on dirty worktree")

	// Verify worktree still exists
	_, statErr := os.Stat(wtPath)
	assert.False(t, os.IsNotExist(statErr), "worktree should still exist after failed remove")
	_ = out
}

// TestRemove_ForceOverridesDirty verifies that --force removes a dirty worktree.
func TestRemove_ForceOverridesDirty(t *testing.T) {
	binary := buildBinary(t)
	repoDir := initTestRepo(t)

	require.NoError(t, os.WriteFile(filepath.Join(repoDir, ".yggdrasil.toml"), []byte(`
[general]
path_template = "../{repo}.{branch}"
`), 0644))

	createCmd := exec.Command(binary, "new", "force-branch")
	createCmd.Dir = repoDir
	createCmd.Env = append(os.Environ(), "HOME="+t.TempDir())
	require.NoError(t, createCmd.Run())

	wtPath := filepath.Join(filepath.Dir(repoDir), filepath.Base(repoDir)+".force-branch")
	require.NoError(t, os.WriteFile(filepath.Join(wtPath, "uncommitted.txt"), []byte("dirty"), 0644))

	removeCmd := exec.Command(binary, "remove", "--force", "force-branch")
	removeCmd.Dir = repoDir
	removeCmd.Env = append(os.Environ(), "HOME="+t.TempDir())
	out, err := removeCmd.CombinedOutput()
	require.NoError(t, err, "yg remove --force failed: %s", string(out))

	_, err = os.Stat(wtPath)
	assert.True(t, os.IsNotExist(err), "worktree should be removed with --force")
}

// TestRemove_DeleteBranch verifies that --delete-branch removes the branch
// ref after removing the worktree.
func TestRemove_DeleteBranch(t *testing.T) {
	binary := buildBinary(t)
	repoDir := initTestRepo(t)

	require.NoError(t, os.WriteFile(filepath.Join(repoDir, ".yggdrasil.toml"), []byte(`
[general]
path_template = "../{repo}.{branch}"
`), 0644))

	createCmd := exec.Command(binary, "new", "branch-to-delete")
	createCmd.Dir = repoDir
	createCmd.Env = append(os.Environ(), "HOME="+t.TempDir())
	require.NoError(t, createCmd.Run())

	// Remove with --delete-branch
	removeCmd := exec.Command(binary, "remove", "--delete-branch", "branch-to-delete")
	removeCmd.Dir = repoDir
	removeCmd.Env = append(os.Environ(), "HOME="+t.TempDir())
	out, err := removeCmd.CombinedOutput()
	require.NoError(t, err, "yg remove --delete-branch failed: %s", string(out))

	// Verify branch is gone
	listCmd := exec.Command("git", "branch", "--list", "branch-to-delete")
	listCmd.Dir = repoDir
	branchOut, _ := listCmd.Output()
	assert.NotContains(t, string(branchOut), "branch-to-delete",
		"branch should be deleted after --delete-branch")
}

// TestRemove_UnlocksAgentOwned verifies that removing an agent-owned worktree
// unlocks it first.
func TestRemove_UnlocksAgentOwned(t *testing.T) {
	binary := buildBinary(t)
	repoDir := initTestRepo(t)

	require.NoError(t, os.WriteFile(filepath.Join(repoDir, ".yggdrasil.toml"), []byte(`
[general]
path_template = "../{repo}.{branch}"
`), 0644))

	// Create an agent-owned worktree
	createCmd := exec.Command(binary, "new", "--agent-owned", "agent-rm")
	createCmd.Dir = repoDir
	createCmd.Env = append(os.Environ(), "HOME="+t.TempDir())
	require.NoError(t, createCmd.Run())

	wtPath := filepath.Join(filepath.Dir(repoDir), filepath.Base(repoDir)+".agent-rm")

	// Remove it
	removeCmd := exec.Command(binary, "remove", "agent-rm")
	removeCmd.Dir = repoDir
	removeCmd.Env = append(os.Environ(), "HOME="+t.TempDir())
	out, err := removeCmd.CombinedOutput()
	require.NoError(t, err, "yg remove of agent-owned worktree failed: %s", string(out))

	_, err = os.Stat(wtPath)
	assert.True(t, os.IsNotExist(err), "agent-owned worktree should be removed")
}

// TestRemove_RunsPreRemoveHooks verifies that pre_remove hooks run before removal.
func TestRemove_RunsPreRemoveHooks(t *testing.T) {
	binary := buildBinary(t)
	repoDir := initTestRepo(t)

	require.NoError(t, os.WriteFile(filepath.Join(repoDir, ".yggdrasil.toml"), []byte(`
[general]
path_template = "../{repo}.{branch}"

[hooks]
pre_remove = ["echo pre-remove > $YG_PRIMARY/pre-remove-marker.txt"]
post_remove = ["echo post-remove > post-remove-marker.txt"]
`), 0644))

	createCmd := exec.Command(binary, "new", "hook-test")
	createCmd.Dir = repoDir
	createCmd.Env = append(os.Environ(), "HOME="+t.TempDir())
	require.NoError(t, createCmd.Run())

	// Remove it
	removeCmd := exec.Command(binary, "remove", "hook-test")
	removeCmd.Dir = repoDir
	removeCmd.Env = append(os.Environ(), "HOME="+t.TempDir())
	out, err := removeCmd.CombinedOutput()
	require.NoError(t, err, "yg remove failed: %s", string(out))

	// pre_remove and post_remove both run in primary, markers should be in repoDir
	preContent, err := os.ReadFile(filepath.Join(repoDir, "pre-remove-marker.txt"))
	require.NoError(t, err)
	assert.Equal(t, "pre-remove\n", string(preContent))

	postContent, err := os.ReadFile(filepath.Join(repoDir, "post-remove-marker.txt"))
	require.NoError(t, err)
	assert.Equal(t, "post-remove\n", string(postContent))
}
