package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// buildBinary builds the yg binary and returns its path.
func buildBinary(t *testing.T) string {
	t.Helper()
	binaryPath := filepath.Join(t.TempDir(), "yg")
	cmd := exec.Command("go", "build", "-o", binaryPath, ".")
	output, err := cmd.CombinedOutput()
	require.NoError(t, err, "go build failed: %s", string(output))
	return binaryPath
}

// initTestRepo creates a git repo with an initial commit and a .yggdrasil.toml.
func initTestRepo(t *testing.T) string {
	t.Helper()
	repoDir := t.TempDir()

	run := func(args ...string) {
		cmd := exec.Command("git", args...)
		cmd.Dir = repoDir
		out, err := cmd.CombinedOutput()
		require.NoError(t, err, "git %v: %s", args, string(out))
	}

	run("init", "-b", "main")
	run("config", "user.email", "test@example.com")
	run("config", "user.name", "Test User")
	os.WriteFile(filepath.Join(repoDir, "README.md"), []byte("# Test"), 0644)
	run("add", ".")
	run("commit", "-m", "initial")

	return repoDir
}

// TestNew_CreatesWorktreeAndProvisionsFiles verifies that `yg new` creates a
// worktree, provisions copied/linked files, and runs post_create hooks.
func TestNew_CreatesWorktreeAndProvisionsFiles(t *testing.T) {
	binary := buildBinary(t)
	repoDir := initTestRepo(t)

	// Create source files to provision
	require.NoError(t, os.WriteFile(filepath.Join(repoDir, ".env"), []byte("KEY=value"), 0644))
	require.NoError(t, os.MkdirAll(filepath.Join(repoDir, "certs"), 0755))
	require.NoError(t, os.WriteFile(filepath.Join(repoDir, "certs", "dev.pem"), []byte("PEM"), 0644))

	// Create config
	configContent := `
[general]
path_template = "../{repo}.{branch}"

[provision]
copy = [".env"]
link = ["certs/dev.pem"]

[hooks]
post_create = ["echo installed > .yg-installed"]
`
	require.NoError(t, os.WriteFile(filepath.Join(repoDir, ".yggdrasil.toml"), []byte(configContent), 0644))

	// Run yg new
	cmd := exec.Command(binary, "new", "feature-x")
	cmd.Dir = repoDir
	cmd.Env = append(os.Environ(), "HOME="+t.TempDir())
	out, err := cmd.CombinedOutput()
	require.NoError(t, err, "yg new failed: %s", string(out))

	// Verify worktree was created
	expectedPath := filepath.Join(filepath.Dir(repoDir), filepath.Base(repoDir)+".feature-x")
	_, err = os.Stat(expectedPath)
	assert.NoError(t, err, "expected worktree at %s", expectedPath)

	// Verify .env was copied
	envContent, err := os.ReadFile(filepath.Join(expectedPath, ".env"))
	require.NoError(t, err)
	assert.Equal(t, "KEY=value", string(envContent))

	// Verify cert.pem was linked (is a symlink)
	info, err := os.Lstat(filepath.Join(expectedPath, "certs", "dev.pem"))
	require.NoError(t, err)
	assert.True(t, info.Mode()&os.ModeSymlink != 0, "expected cert.pem to be a symlink")

	// Verify hook ran (marker file exists)
	markerContent, err := os.ReadFile(filepath.Join(expectedPath, ".yg-installed"))
	require.NoError(t, err)
	assert.Equal(t, "installed\n", string(markerContent))
}

// TestNew_PrintPath verifies that --print-path outputs only the worktree path.
func TestNew_PrintPath(t *testing.T) {
	binary := buildBinary(t)
	repoDir := initTestRepo(t)

	configContent := `
[general]
path_template = "../{repo}.{branch}"
`
	require.NoError(t, os.WriteFile(filepath.Join(repoDir, ".yggdrasil.toml"), []byte(configContent), 0644))

	cmd := exec.Command(binary, "new", "--print-path", "feature-y")
	cmd.Dir = repoDir
	cmd.Env = append(os.Environ(), "HOME="+t.TempDir())
	out, err := cmd.CombinedOutput()
	require.NoError(t, err, "yg new --print-path failed: %s", string(out))

	expectedPath := filepath.Join(filepath.Dir(repoDir), filepath.Base(repoDir)+".feature-y")
	assert.Equal(t, expectedPath+"\n", string(out), "output should be just the path")
}

// TestNew_AgentOwnedLocksWorktree verifies that --agent-owned adds a git
// worktree lock.
func TestNew_AgentOwnedLocksWorktree(t *testing.T) {
	binary := buildBinary(t)
	repoDir := initTestRepo(t)

	require.NoError(t, os.WriteFile(filepath.Join(repoDir, ".yggdrasil.toml"), []byte(`[general]
path_template = "../{repo}.{branch}"
`), 0644))

	cmd := exec.Command(binary, "new", "--agent-owned", "feature-z")
	cmd.Dir = repoDir
	cmd.Env = append(os.Environ(), "HOME="+t.TempDir())
	out, err := cmd.CombinedOutput()
	require.NoError(t, err, "yg new --agent-owned failed: %s", string(out))

	// Check worktree is locked
	listCmd := exec.Command("git", "worktree", "list", "--porcelain")
	listCmd.Dir = repoDir
	listOut, err := listCmd.Output()
	require.NoError(t, err)
	assert.Contains(t, string(listOut), "locked", "expected worktree to be locked")
}

// TestNew_DryRunCreatesNothing verifies that --dry-run prints planned actions
// without creating anything.
func TestNew_DryRunCreatesNothing(t *testing.T) {
	binary := buildBinary(t)
	repoDir := initTestRepo(t)

	require.NoError(t, os.WriteFile(filepath.Join(repoDir, ".env"), []byte("KEY=value"), 0644))
	require.NoError(t, os.WriteFile(filepath.Join(repoDir, ".yggdrasil.toml"), []byte(`
[general]
path_template = "../{repo}.{branch}"

[provision]
copy = [".env"]

[hooks]
post_create = ["echo should-not-run > marker.txt"]
`), 0644))

	cmd := exec.Command(binary, "new", "--dry-run", "feature-dry")
	cmd.Dir = repoDir
	cmd.Env = append(os.Environ(), "HOME="+t.TempDir())
	out, err := cmd.CombinedOutput()
	require.NoError(t, err, "yg new --dry-run failed: %s", string(out))

	// Verify no worktree directory was created
	expectedPath := filepath.Join(filepath.Dir(repoDir), filepath.Base(repoDir)+".feature-dry")
	_, err = os.Stat(expectedPath)
	assert.True(t, os.IsNotExist(err), "expected no worktree directory in dry-run mode")
}

// TestNew_BranchAlreadyExists verifies that creating a worktree for an
// existing branch uses the existing branch.
func TestNew_BranchAlreadyExists(t *testing.T) {
	binary := buildBinary(t)
	repoDir := initTestRepo(t)

	require.NoError(t, os.WriteFile(filepath.Join(repoDir, ".yggdrasil.toml"), []byte(`
[general]
path_template = "../{repo}.{branch}"
`), 0644))

	// Create the branch first
	exec.Command("git", "-C", repoDir, "branch", "existing-branch").Run()

	cmd := exec.Command(binary, "new", "existing-branch")
	cmd.Dir = repoDir
	cmd.Env = append(os.Environ(), "HOME="+t.TempDir())
	out, err := cmd.CombinedOutput()
	require.NoError(t, err, "yg new with existing branch failed: %s", string(out))

	expectedPath := filepath.Join(filepath.Dir(repoDir), filepath.Base(repoDir)+".existing-branch")
	_, err = os.Stat(expectedPath)
	assert.NoError(t, err, "expected worktree at %s", expectedPath)
}

// TestNew_BranchNewSignalForNewBranch verifies that YG_BRANCH_NEW=1 and
// YG_BASE=<base> are set in the hook environment when the branch is newly
// created.
func TestNew_BranchNewSignalForNewBranch(t *testing.T) {
	binary := buildBinary(t)
	repoDir := initTestRepo(t)

	configContent := `
[general]
path_template = "../{repo}.{branch}"

[hooks]
post_create = ["env > .yg-env-capture"]
`
	require.NoError(t, os.WriteFile(filepath.Join(repoDir, ".yggdrasil.toml"), []byte(configContent), 0644))

	cmd := exec.Command(binary, "new", "fresh-feature")
	cmd.Dir = repoDir
	cmd.Env = append(os.Environ(), "HOME="+t.TempDir())
	out, err := cmd.CombinedOutput()
	require.NoError(t, err, "yg new failed: %s", string(out))

	wtPath := filepath.Join(filepath.Dir(repoDir), filepath.Base(repoDir)+".fresh-feature")
	envCapture, err := os.ReadFile(filepath.Join(wtPath, ".yg-env-capture"))
	require.NoError(t, err, "expected .yg-env-capture in worktree")
	envStr := string(envCapture)

	assert.Contains(t, envStr, "YG_BRANCH_NEW=1", "YG_BRANCH_NEW should be 1 for a newly created branch")
	assert.Contains(t, envStr, "YG_BASE=main", "YG_BASE should be the trunk (main) when no explicit base")
	assert.Contains(t, envStr, "YG_BRANCH=fresh-feature")
}

// TestNew_BranchNewSignalForExistingBranch verifies that YG_BRANCH_NEW=0 is
// set when the branch already existed before `yg new`.
func TestNew_BranchNewSignalForExistingBranch(t *testing.T) {
	binary := buildBinary(t)
	repoDir := initTestRepo(t)

	configContent := `
[general]
path_template = "../{repo}.{branch}"

[hooks]
post_create = ["env > .yg-env-capture"]
`
	require.NoError(t, os.WriteFile(filepath.Join(repoDir, ".yggdrasil.toml"), []byte(configContent), 0644))

	// Pre-create the branch so yg new doesn't create it
	exec.Command("git", "-C", repoDir, "branch", "pre-existing").Run()

	cmd := exec.Command(binary, "new", "pre-existing")
	cmd.Dir = repoDir
	cmd.Env = append(os.Environ(), "HOME="+t.TempDir())
	out, err := cmd.CombinedOutput()
	require.NoError(t, err, "yg new with existing branch failed: %s", string(out))

	wtPath := filepath.Join(filepath.Dir(repoDir), filepath.Base(repoDir)+".pre-existing")
	envCapture, err := os.ReadFile(filepath.Join(wtPath, ".yg-env-capture"))
	require.NoError(t, err, "expected .yg-env-capture in worktree")
	envStr := string(envCapture)

	assert.Contains(t, envStr, "YG_BRANCH_NEW=0", "YG_BRANCH_NEW should be 0 for a pre-existing branch")
}

// TestNew_BaseFromExplicitArg verifies that YG_BASE reflects an explicit base
// argument, not just the trunk default.
func TestNew_BaseFromExplicitArg(t *testing.T) {
	binary := buildBinary(t)
	repoDir := initTestRepo(t)

	// Create a non-trunk base branch
	exec.Command("git", "-C", repoDir, "branch", "dev-staging").Run()

	configContent := `
[general]
path_template = "../{repo}.{branch}"

[hooks]
post_create = ["env > .yg-env-capture"]
`
	require.NoError(t, os.WriteFile(filepath.Join(repoDir, ".yggdrasil.toml"), []byte(configContent), 0644))

	cmd := exec.Command(binary, "new", "feature-from-dev", "dev-staging")
	cmd.Dir = repoDir
	cmd.Env = append(os.Environ(), "HOME="+t.TempDir())
	out, err := cmd.CombinedOutput()
	require.NoError(t, err, "yg new with explicit base failed: %s", string(out))

	wtPath := filepath.Join(filepath.Dir(repoDir), filepath.Base(repoDir)+".feature-from-dev")
	envCapture, err := os.ReadFile(filepath.Join(wtPath, ".yg-env-capture"))
	require.NoError(t, err, "expected .yg-env-capture in worktree")
	envStr := string(envCapture)

	assert.Contains(t, envStr, "YG_BASE=dev-staging", "YG_BASE should reflect the explicit base argument")
	assert.Contains(t, envStr, "YG_BRANCH_NEW=1")
}

// avoid unused import warning
