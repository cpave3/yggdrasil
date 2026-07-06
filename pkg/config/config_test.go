package config

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestLoad_NoConfigFile_ReturnsDefaults verifies that loading config from a repo
// with no .yggdrasil.toml returns built-in defaults.
func TestLoad_NoConfigFile_ReturnsDefaults(t *testing.T) {
	tmpDir := t.TempDir()
	cfg, err := Load(tmpDir)
	require.NoError(t, err)

	assert.Equal(t, DefaultPathTemplate, cfg.General.PathTemplate)
	assert.Empty(t, cfg.General.Trunk)
	assert.Empty(t, cfg.Provision.Copy)
	assert.Empty(t, cfg.Provision.Link)
	assert.Empty(t, cfg.Hooks.PostCreate)
}

// TestLoad_ParsesProjectConfig verifies that a .yggdrasil.toml with provision
// and hooks is parsed correctly.
func TestLoad_ParsesProjectConfig(t *testing.T) {
	tmpDir := t.TempDir()
	configFile := filepath.Join(tmpDir, ".yggdrasil.toml")
	writeFile(t, configFile, `# yggdrasil config
[general]
trunk = "main"
path_template = "../worktrees/{branch}"

[provision]
copy = [".env", ".env.*"]
link = ["certs/dev.pem"]

[hooks]
post_create = ["pnpm install"]
pre_merge = ["pnpm test", "pnpm lint"]
`)

	cfg, err := Load(tmpDir)
	require.NoError(t, err)

	assert.Equal(t, "main", cfg.General.Trunk)
	assert.Equal(t, "../worktrees/{branch}", cfg.General.PathTemplate)
	assert.Equal(t, []string{".env", ".env.*"}, cfg.Provision.Copy)
	assert.Equal(t, []string{"certs/dev.pem"}, cfg.Provision.Link)
	assert.Equal(t, []string{"pnpm install"}, cfg.Hooks.PostCreate)
	assert.Equal(t, []string{"pnpm test", "pnpm lint"}, cfg.Hooks.PreMerge)
}

// TestLoad_LocalOverride_AppendsProvisioning verifies that local override config
// appends to (not replaces) provisioning copy/link lists per FR-8.
func TestLoad_LocalOverride_AppendsProvisioning(t *testing.T) {
	tmpDir := t.TempDir()
	writeFile(t, filepath.Join(tmpDir, ".yggdrasil.toml"), `
[provision]
copy = [".env"]
link = ["certs/dev.pem"]
`)
	writeFile(t, filepath.Join(tmpDir, ".yggdrasil.local.toml"), `
[provision]
copy = [".env.local"]
link = ["certs/local.pem"]
`)

	cfg, err := Load(tmpDir)
	require.NoError(t, err)

	assert.Equal(t, []string{".env", ".env.local"}, cfg.Provision.Copy)
	assert.Equal(t, []string{"certs/dev.pem", "certs/local.pem"}, cfg.Provision.Link)
}

// TestLoad_LocalOverride_ReplacesHooks verifies that local override config
// replaces (not appends) hook arrays when set in a higher layer per FR-8.
func TestLoad_LocalOverride_ReplacesHooks(t *testing.T) {
	tmpDir := t.TempDir()
	writeFile(t, filepath.Join(tmpDir, ".yggdrasil.toml"), `
[hooks]
post_create = ["pnpm install"]
pre_merge = ["pnpm test"]
`)
	writeFile(t, filepath.Join(tmpDir, ".yggdrasil.local.toml"), `
[hooks]
post_create = ["pnpm install --prefer-offline"]
`)

	cfg, err := Load(tmpDir)
	require.NoError(t, err)

	assert.Equal(t, []string{"pnpm install --prefer-offline"}, cfg.Hooks.PostCreate)
	assert.Equal(t, []string{"pnpm test"}, cfg.Hooks.PreMerge, "unreplaced hooks inherited from project")
}

func writeFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatalf("failed to write %s: %v", path, err)
	}
}
