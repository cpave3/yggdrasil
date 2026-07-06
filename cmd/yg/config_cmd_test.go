package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestConfig_PrintsEffectiveConfig verifies that `yg config` prints the
// merged configuration including values from project config.
func TestConfig_PrintsEffectiveConfig(t *testing.T) {
	binary := buildBinary(t)
	repoDir := initTestRepo(t)

	require.NoError(t, os.WriteFile(filepath.Join(repoDir, ".yggdrasil.toml"), []byte(`
[general]
trunk = "main"
path_template = "../worktrees/{branch}"

[provision]
copy = [".env", ".env.*"]

[hooks]
post_create = ["pnpm install"]
`), 0644))

	cmd := exec.Command(binary, "config")
	cmd.Dir = repoDir
	cmd.Env = append(os.Environ(), "HOME="+t.TempDir())
	out, err := cmd.CombinedOutput()
	require.NoError(t, err, "yg config failed: %s", string(out))

	output := string(out)
	assert.Contains(t, output, "main")
	assert.Contains(t, output, "../worktrees/{branch}")
	assert.Contains(t, output, ".env")
	assert.Contains(t, output, "pnpm install")
}

// TestConfig_ShowsDefaultsWhenNoConfigFile verifies that `yg config` shows
// built-in defaults when no config file exists.
func TestConfig_ShowsDefaultsWhenNoConfigFile(t *testing.T) {
	binary := buildBinary(t)
	repoDir := initTestRepo(t)

	// No .yggdrasil.toml — should show defaults
	cmd := exec.Command(binary, "config")
	cmd.Dir = repoDir
	cmd.Env = append(os.Environ(), "HOME="+t.TempDir())
	out, err := cmd.CombinedOutput()
	require.NoError(t, err, "yg config failed: %s", string(out))

	output := string(out)
	assert.Contains(t, output, "../{repo}.{branch}", "should show default path template")
}

// TestConfig_ShowsLocalOverrideMerged verifies that local override values
// are merged into the output.
func TestConfig_ShowsLocalOverrideMerged(t *testing.T) {
	binary := buildBinary(t)
	repoDir := initTestRepo(t)

	require.NoError(t, os.WriteFile(filepath.Join(repoDir, ".yggdrasil.toml"), []byte(`
[provision]
copy = [".env"]
`), 0644))

	require.NoError(t, os.WriteFile(filepath.Join(repoDir, ".yggdrasil.local.toml"), []byte(`
[provision]
copy = [".env.local"]
`), 0644))

	cmd := exec.Command(binary, "config")
	cmd.Dir = repoDir
	cmd.Env = append(os.Environ(), "HOME="+t.TempDir())
	out, err := cmd.CombinedOutput()
	require.NoError(t, err, "yg config failed: %s", string(out))

	output := string(out)
	assert.Contains(t, output, ".env", "should contain project .env")
	assert.Contains(t, output, ".env.local", "should contain local .env.local")
}
