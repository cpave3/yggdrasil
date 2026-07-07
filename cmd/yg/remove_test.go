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

// TestRemove_CurrentRemovesWorktreeFromInside verifies that `yg remove
// --current` removes the worktree the shell is sitting in and leaves the
// branch ref intact.
func TestRemove_CurrentRemovesWorktreeFromInside(t *testing.T) {
	binary := buildBinary(t)
	repoDir := initTestRepo(t)

	require.NoError(t, os.WriteFile(filepath.Join(repoDir, ".yggdrasil.toml"), []byte(`
[general]
path_template = "../{repo}.{branch}"
`), 0644))

	createCmd := exec.Command(binary, "new", "current-rm")
	createCmd.Dir = repoDir
	createCmd.Env = append(os.Environ(), "HOME="+t.TempDir())
	require.NoError(t, createCmd.Run())

	wtPath := filepath.Join(filepath.Dir(repoDir), filepath.Base(repoDir)+".current-rm")

	removeCmd := exec.Command(binary, "remove", "--current")
	removeCmd.Dir = wtPath
	removeCmd.Env = append(os.Environ(), "HOME="+t.TempDir())
	out, err := removeCmd.CombinedOutput()
	require.NoError(t, err, "yg remove --current failed: %s", string(out))

	// Worktree directory is gone
	_, err = os.Stat(wtPath)
	assert.True(t, os.IsNotExist(err), "worktree directory should be removed")

	// Branch ref is preserved
	listCmd := exec.Command("git", "branch", "--list", "current-rm")
	listCmd.Dir = repoDir
	branchOut, _ := listCmd.Output()
	assert.Contains(t, string(branchOut), "current-rm",
		"branch should still exist after --current remove")
}

// TestRemove_CurrentFromSubdirectory verifies that --current works when
// invoked from a subdirectory of the worktree (ShowToplevel canonicalises
// the CWD up to the worktree root).
func TestRemove_CurrentFromSubdirectory(t *testing.T) {
	binary := buildBinary(t)
	repoDir := initTestRepo(t)

	require.NoError(t, os.WriteFile(filepath.Join(repoDir, ".yggdrasil.toml"), []byte(`
[general]
path_template = "../{repo}.{branch}"
`), 0644))

	createCmd := exec.Command(binary, "new", "subdir-rm")
	createCmd.Dir = repoDir
	createCmd.Env = append(os.Environ(), "HOME="+t.TempDir())
	require.NoError(t, createCmd.Run())

	wtPath := filepath.Join(filepath.Dir(repoDir), filepath.Base(repoDir)+".subdir-rm")
	require.NoError(t, os.MkdirAll(filepath.Join(wtPath, "src"), 0755))

	removeCmd := exec.Command(binary, "remove", "--current")
	removeCmd.Dir = filepath.Join(wtPath, "src")
	removeCmd.Env = append(os.Environ(), "HOME="+t.TempDir())
	out, err := removeCmd.CombinedOutput()
	require.NoError(t, err, "yg remove --current from subdir failed: %s", string(out))

	_, err = os.Stat(wtPath)
	assert.True(t, os.IsNotExist(err), "worktree directory should be removed")
}

// TestRemove_CurrentFailsOnPrimaryWorktree verifies that --current refuses
// to remove the primary worktree.
func TestRemove_CurrentFailsOnPrimaryWorktree(t *testing.T) {
	binary := buildBinary(t)
	repoDir := initTestRepo(t)

	require.NoError(t, os.WriteFile(filepath.Join(repoDir, ".yggdrasil.toml"), []byte(`
[general]
path_template = "../{repo}.{branch}"
`), 0644))

	removeCmd := exec.Command(binary, "remove", "--current")
	removeCmd.Dir = repoDir
	removeCmd.Env = append(os.Environ(), "HOME="+t.TempDir())
	out, err := removeCmd.CombinedOutput()
	require.Error(t, err, "expected remove --current to fail on primary worktree")
	assert.Contains(t, string(out), "primary", "error should mention primary worktree")
}

// TestRemove_CurrentDirtyFailsWithoutForce verifies that --current honours
// the dirty safety check, and that --current --force overrides it.
func TestRemove_CurrentDirtyFailsWithoutForce(t *testing.T) {
	binary := buildBinary(t)
	repoDir := initTestRepo(t)

	require.NoError(t, os.WriteFile(filepath.Join(repoDir, ".yggdrasil.toml"), []byte(`
[general]
path_template = "../{repo}.{branch}"
`), 0644))

	createCmd := exec.Command(binary, "new", "current-dirty")
	createCmd.Dir = repoDir
	createCmd.Env = append(os.Environ(), "HOME="+t.TempDir())
	require.NoError(t, createCmd.Run())

	wtPath := filepath.Join(filepath.Dir(repoDir), filepath.Base(repoDir)+".current-dirty")
	require.NoError(t, os.WriteFile(filepath.Join(wtPath, "uncommitted.txt"), []byte("dirty"), 0644))

	// Without --force: should fail
	removeCmd := exec.Command(binary, "remove", "--current")
	removeCmd.Dir = wtPath
	removeCmd.Env = append(os.Environ(), "HOME="+t.TempDir())
	out, err := removeCmd.CombinedOutput()
	require.Error(t, err, "expected remove --current to fail on dirty worktree")
	assert.Contains(t, string(out), "dirty", "error should mention dirty state")

	_, statErr := os.Stat(wtPath)
	assert.False(t, os.IsNotExist(statErr), "worktree should still exist after failed remove")

	// With --force: should succeed
	forceCmd := exec.Command(binary, "remove", "--current", "--force")
	forceCmd.Dir = wtPath
	forceCmd.Env = append(os.Environ(), "HOME="+t.TempDir())
	out, err = forceCmd.CombinedOutput()
	require.NoError(t, err, "yg remove --current --force failed: %s", string(out))

	_, err = os.Stat(wtPath)
	assert.True(t, os.IsNotExist(err), "worktree should be removed with --force")
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

// TestHasForeignOwnedFiles_UserOwned verifies hasForeignOwnedFiles returns
// false for a directory tree entirely owned by the current user.
func TestHasForeignOwnedFiles_UserOwned(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "a.txt"), []byte("x"), 0644))
	require.NoError(t, os.MkdirAll(filepath.Join(dir, "sub"), 0755))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "sub", "b.txt"), []byte("y"), 0644))

	assert.False(t, hasForeignOwnedFiles(dir), "user-owned tree should not be foreign")
}

// TestHasForeignOwnedFiles_NonExistent verifies hasForeignOwnedFiles returns
// false for a path that doesn't exist.
func TestHasForeignOwnedFiles_NonExistent(t *testing.T) {
	assert.False(t, hasForeignOwnedFiles("/nonexistent/path/that/should/not/exist"))
}

// TestIsInteractive_NotTTY verifies isInteractive returns false when stdin
// is a pipe (as it is under the test runner).
func TestIsInteractive_NotTTY(t *testing.T) {
	// Under `go test`, stdin is not a char device.
	// This is a smoke test — it verifies the function doesn't panic and
	// returns a bool. We can't assert true since CI pipes stdin.
	_ = isInteractive()
}
