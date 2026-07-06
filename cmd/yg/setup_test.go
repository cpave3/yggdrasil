package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestSetup_ReprovisionsExistingTree verifies that `yg setup` re-runs
// provisioning and post_create on an existing worktree without recreating it.
func TestSetup_ReprovisionsExistingTree(t *testing.T) {
	binary := buildBinary(t)
	repoDir := initTestRepo(t)

	require.NoError(t, os.WriteFile(filepath.Join(repoDir, ".env"), []byte("KEY=v1"), 0644))
	require.NoError(t, os.WriteFile(filepath.Join(repoDir, ".yggdrasil.toml"), []byte(`
[general]
path_template = "../{repo}.{branch}"

[provision]
copy = [".env"]

[hooks]
post_create = ["echo setup-ran > .yg-setup-marker"]
`), 0644))

	// Create a worktree
	createCmd := exec.Command(binary, "new", "setup-test")
	createCmd.Dir = repoDir
	createCmd.Env = append(os.Environ(), "HOME="+t.TempDir())
	require.NoError(t, createCmd.Run())

	wtPath := filepath.Join(filepath.Dir(repoDir), filepath.Base(repoDir)+".setup-test")

	// Verify .env was copied
	envContent, err := os.ReadFile(filepath.Join(wtPath, ".env"))
	require.NoError(t, err)
	assert.Equal(t, "KEY=v1", string(envContent))

	// Change the source .env
	require.NoError(t, os.WriteFile(filepath.Join(repoDir, ".env"), []byte("KEY=v2"), 0644))

	// Remove the marker to verify setup re-runs hooks
	os.Remove(filepath.Join(wtPath, ".yg-setup-marker"))

	// Run setup
	setupCmd := exec.Command(binary, "setup", "setup-test")
	setupCmd.Dir = repoDir
	setupCmd.Env = append(os.Environ(), "HOME="+t.TempDir())
	out, err := setupCmd.CombinedOutput()
	require.NoError(t, err, "yg setup failed: %s", string(out))

	// Verify .env was re-provisioned (updated content)
	envContent, err = os.ReadFile(filepath.Join(wtPath, ".env"))
	require.NoError(t, err)
	assert.Equal(t, "KEY=v2", string(envContent))

	// Verify post_create hook ran (marker file exists)
	markerContent, err := os.ReadFile(filepath.Join(wtPath, ".yg-setup-marker"))
	require.NoError(t, err)
	assert.Equal(t, "setup-ran\n", string(markerContent))
}

// TestSetup_DoesNotRecreateWorktree verifies that setup doesn't try to create
// a new worktree — it operates on the existing one.
func TestSetup_DoesNotRecreateWorktree(t *testing.T) {
	binary := buildBinary(t)
	repoDir := initTestRepo(t)

	require.NoError(t, os.WriteFile(filepath.Join(repoDir, ".yggdrasil.toml"), []byte(`
[general]
path_template = "../{repo}.{branch}"

[hooks]
post_create = ["echo data > existing-file.txt"]
`), 0644))

	// Create a worktree with a marker file
	createCmd := exec.Command(binary, "new", "setup-existing")
	createCmd.Dir = repoDir
	createCmd.Env = append(os.Environ(), "HOME="+t.TempDir())
	require.NoError(t, createCmd.Run())

	wtPath := filepath.Join(filepath.Dir(repoDir), filepath.Base(repoDir)+".setup-existing")

	// Create a file in the worktree that wouldn't exist if it were recreated
	require.NoError(t, os.WriteFile(filepath.Join(wtPath, "user-file.txt"), []byte("user data"), 0644))

	// Run setup
	setupCmd := exec.Command(binary, "setup", "setup-existing")
	setupCmd.Dir = repoDir
	setupCmd.Env = append(os.Environ(), "HOME="+t.TempDir())
	out, err := setupCmd.CombinedOutput()
	require.NoError(t, err, "yg setup failed: %s", string(out))

	// Verify the user file still exists (worktree was not recreated)
	userContent, err := os.ReadFile(filepath.Join(wtPath, "user-file.txt"))
	require.NoError(t, err)
	assert.Equal(t, "user data", string(userContent))
}

// TestSetup_Idempotent verifies that running setup multiple times doesn't error.
func TestSetup_Idempotent(t *testing.T) {
	binary := buildBinary(t)
	repoDir := initTestRepo(t)

	require.NoError(t, os.WriteFile(filepath.Join(repoDir, ".env"), []byte("KEY=val"), 0644))
	require.NoError(t, os.WriteFile(filepath.Join(repoDir, ".yggdrasil.toml"), []byte(`
[general]
path_template = "../{repo}.{branch}"

[provision]
copy = [".env"]
`), 0644))

	// Create worktree
	createCmd := exec.Command(binary, "new", "setup-idempotent")
	createCmd.Dir = repoDir
	createCmd.Env = append(os.Environ(), "HOME="+t.TempDir())
	require.NoError(t, createCmd.Run())

	// Run setup twice
	for i := 0; i < 2; i++ {
		setupCmd := exec.Command(binary, "setup", "setup-idempotent")
		setupCmd.Dir = repoDir
		setupCmd.Env = append(os.Environ(), "HOME="+t.TempDir())
		out, err := setupCmd.CombinedOutput()
		require.NoError(t, err, "yg setup run %d failed: %s", i+1, string(out))
	}

	// Verify .env still exists and is correct
	wtPath := filepath.Join(filepath.Dir(repoDir), filepath.Base(repoDir)+".setup-idempotent")
	envContent, err := os.ReadFile(filepath.Join(wtPath, ".env"))
	require.NoError(t, err)
	assert.Equal(t, "KEY=val", string(envContent))
}
