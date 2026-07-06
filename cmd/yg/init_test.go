package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestInit_CreatesConfigWithDefaults verifies that `yg init` creates a
// .yggdrasil.toml with all sections and comments, even in an empty repo.
func TestInit_CreatesConfigWithDefaults(t *testing.T) {
	binary := buildBinary(t)
	repoDir := initTestRepo(t)

	cmd := exec.Command(binary, "init")
	cmd.Dir = repoDir
	cmd.Env = append(os.Environ(), "HOME="+t.TempDir())
	out, err := cmd.CombinedOutput()
	require.NoError(t, err, "yg init failed: %s", string(out))

	configPath := filepath.Join(repoDir, ".yggdrasil.toml")
	data, err := os.ReadFile(configPath)
	require.NoError(t, err, ".yggdrasil.toml should exist")
	content := string(data)

	// All sections should be present
	assert.Contains(t, content, "[general]")
	assert.Contains(t, content, "[provision]")
	assert.Contains(t, content, "[hooks]")
}

// TestInit_DoesNotOverwriteExistingConfig verifies that `yg init` refuses to
// overwrite an existing .yggdrasil.toml without --force.
func TestInit_DoesNotOverwriteExistingConfig(t *testing.T) {
	binary := buildBinary(t)
	repoDir := initTestRepo(t)

	existing := []byte("# my config\n[general]\ntrunk = \"main\"\n")
	require.NoError(t, os.WriteFile(filepath.Join(repoDir, ".yggdrasil.toml"), existing, 0644))

	cmd := exec.Command(binary, "init")
	cmd.Dir = repoDir
	cmd.Env = append(os.Environ(), "HOME="+t.TempDir())
	err := cmd.Run()
	assert.Error(t, err, "should refuse to overwrite without --force")

	// Verify original content is untouched
	data, err := os.ReadFile(filepath.Join(repoDir, ".yggdrasil.toml"))
	require.NoError(t, err)
	assert.Equal(t, string(existing), string(data))
}

// TestInit_ForceOverwritesExistingConfig verifies that `yg init --force`
// overwrites an existing .yggdrasil.toml.
func TestInit_ForceOverwritesExistingConfig(t *testing.T) {
	binary := buildBinary(t)
	repoDir := initTestRepo(t)

	require.NoError(t, os.WriteFile(filepath.Join(repoDir, ".yggdrasil.toml"), []byte("# old\n"), 0644))

	cmd := exec.Command(binary, "init", "--force")
	cmd.Dir = repoDir
	cmd.Env = append(os.Environ(), "HOME="+t.TempDir())
	out, err := cmd.CombinedOutput()
	require.NoError(t, err, "yg init --force failed: %s", string(out))

	data, err := os.ReadFile(filepath.Join(repoDir, ".yggdrasil.toml"))
	require.NoError(t, err)
	assert.NotContains(t, string(data), "# old")
	assert.Contains(t, string(data), "[general]")
}

// TestInit_DetectsEnvFiles verifies that `yg init` auto-detects .env files
// and adds them to the copy provisioning list.
func TestInit_DetectsEnvFiles(t *testing.T) {
	binary := buildBinary(t)
	repoDir := initTestRepo(t)

	// Create env files in the repo
	require.NoError(t, os.WriteFile(filepath.Join(repoDir, ".env"), []byte("KEY=val"), 0644))
	require.NoError(t, os.WriteFile(filepath.Join(repoDir, ".env.local"), []byte("KEY=val"), 0644))

	cmd := exec.Command(binary, "init")
	cmd.Dir = repoDir
	cmd.Env = append(os.Environ(), "HOME="+t.TempDir())
	out, err := cmd.CombinedOutput()
	require.NoError(t, err, "yg init failed: %s", string(out))

	data, err := os.ReadFile(filepath.Join(repoDir, ".yggdrasil.toml"))
	require.NoError(t, err)
	content := string(data)

	assert.Contains(t, content, ".env", "should detect .env file")
}

// TestInit_DetectsNodeEcosystem verifies that `yg init` detects a Node.js
// project (package.json) and suggests appropriate hooks.
func TestInit_DetectsNodeEcosystem(t *testing.T) {
	binary := buildBinary(t)
	repoDir := initTestRepo(t)

	// Create package.json and a lockfile
	require.NoError(t, os.WriteFile(filepath.Join(repoDir, "package.json"), []byte(`{"name":"test"}`), 0644))
	require.NoError(t, os.WriteFile(filepath.Join(repoDir, "pnpm-lock.yaml"), []byte(""), 0644))
	require.NoError(t, os.WriteFile(filepath.Join(repoDir, ".env"), []byte("KEY=val"), 0644))

	cmd := exec.Command(binary, "init")
	cmd.Dir = repoDir
	cmd.Env = append(os.Environ(), "HOME="+t.TempDir())
	out, err := cmd.CombinedOutput()
	require.NoError(t, err, "yg init failed: %s", string(out))

	data, err := os.ReadFile(filepath.Join(repoDir, ".yggdrasil.toml"))
	require.NoError(t, err)
	content := string(data)

	assert.Contains(t, content, "pnpm install", "should detect pnpm and suggest pnpm install")
	assert.Contains(t, content, ".env", "should detect .env")
}

// TestInit_DetectsPythonEcosystem verifies that `yg init` detects a Python
// project (pyproject.toml) and suggests appropriate hooks.
func TestInit_DetectsPythonEcosystem(t *testing.T) {
	binary := buildBinary(t)
	repoDir := initTestRepo(t)

	require.NoError(t, os.WriteFile(filepath.Join(repoDir, "pyproject.toml"), []byte(`[project]`), 0644))
	require.NoError(t, os.WriteFile(filepath.Join(repoDir, ".env"), []byte("KEY=val"), 0644))

	cmd := exec.Command(binary, "init")
	cmd.Dir = repoDir
	cmd.Env = append(os.Environ(), "HOME="+t.TempDir())
	out, err := cmd.CombinedOutput()
	require.NoError(t, err, "yg init failed: %s", string(out))

	data, err := os.ReadFile(filepath.Join(repoDir, ".yggdrasil.toml"))
	require.NoError(t, err)
	content := string(data)

	assert.Contains(t, content, "pip install", "should detect Python and suggest pip install")
}

// TestInit_DetectsGoEcosystem verifies that `yg init` detects a Go project
// (go.mod) and suggests appropriate hooks.
func TestInit_DetectsGoEcosystem(t *testing.T) {
	binary := buildBinary(t)
	repoDir := initTestRepo(t)

	require.NoError(t, os.WriteFile(filepath.Join(repoDir, "go.mod"), []byte("module example.com/test\ngo 1.26\n"), 0644))

	cmd := exec.Command(binary, "init")
	cmd.Dir = repoDir
	cmd.Env = append(os.Environ(), "HOME="+t.TempDir())
	out, err := cmd.CombinedOutput()
	require.NoError(t, err, "yg init failed: %s", string(out))

	data, err := os.ReadFile(filepath.Join(repoDir, ".yggdrasil.toml"))
	require.NoError(t, err)
	content := string(data)

	assert.Contains(t, content, "go mod download", "should detect Go and suggest go mod download")
}

// TestInit_DetectsRustEcosystem verifies that `yg init` detects a Rust
// project (Cargo.toml) and suggests appropriate hooks.
func TestInit_DetectsRustEcosystem(t *testing.T) {
	binary := buildBinary(t)
	repoDir := initTestRepo(t)

	require.NoError(t, os.WriteFile(filepath.Join(repoDir, "Cargo.toml"), []byte(`[package]`), 0644))

	cmd := exec.Command(binary, "init")
	cmd.Dir = repoDir
	cmd.Env = append(os.Environ(), "HOME="+t.TempDir())
	out, err := cmd.CombinedOutput()
	require.NoError(t, err, "yg init failed: %s", string(out))

	data, err := os.ReadFile(filepath.Join(repoDir, ".yggdrasil.toml"))
	require.NoError(t, err)
	content := string(data)

	assert.Contains(t, content, "cargo build", "should detect Rust and suggest cargo build")
}

// TestInit_DetectsDotnetEcosystem verifies that `yg init` detects a .NET
// project (*.csproj/*.sln) and suggests appropriate hooks.
func TestInit_DetectsDotnetEcosystem(t *testing.T) {
	binary := buildBinary(t)
	repoDir := initTestRepo(t)

	require.NoError(t, os.WriteFile(filepath.Join(repoDir, "MyApp.csproj"), []byte(`<Project Sdk="Microsoft.NET.Sdk">`), 0644))
	require.NoError(t, os.WriteFile(filepath.Join(repoDir, "appsettings.Development.json"), []byte(`{}`), 0644))

	cmd := exec.Command(binary, "init")
	cmd.Dir = repoDir
	cmd.Env = append(os.Environ(), "HOME="+t.TempDir())
	out, err := cmd.CombinedOutput()
	require.NoError(t, err, "yg init failed: %s", string(out))

	data, err := os.ReadFile(filepath.Join(repoDir, ".yggdrasil.toml"))
	require.NoError(t, err)
	content := string(data)

	assert.Contains(t, content, "dotnet restore", "should detect .NET and suggest dotnet restore")
	assert.Contains(t, content, "appsettings.Development.json", "should detect appsettings")
}

// TestInit_DetectsTrunkBranch verifies that `yg init` auto-detects the trunk
// branch and writes it to the config.
func TestInit_DetectsTrunkBranch(t *testing.T) {
	binary := buildBinary(t)
	repoDir := initTestRepo(t)

	cmd := exec.Command(binary, "init")
	cmd.Dir = repoDir
	cmd.Env = append(os.Environ(), "HOME="+t.TempDir())
	out, err := cmd.CombinedOutput()
	require.NoError(t, err, "yg init failed: %s", string(out))

	data, err := os.ReadFile(filepath.Join(repoDir, ".yggdrasil.toml"))
	require.NoError(t, err)
	content := string(data)

	assert.Contains(t, content, "main", "should detect trunk branch 'main'")
}

// TestInit_DetectsCertFiles verifies that `yg init` detects certificate files
// and suggests them for linking.
func TestInit_DetectsCertFiles(t *testing.T) {
	binary := buildBinary(t)
	repoDir := initTestRepo(t)

	require.NoError(t, os.MkdirAll(filepath.Join(repoDir, "certs"), 0755))
	require.NoError(t, os.WriteFile(filepath.Join(repoDir, "certs", "dev.pem"), []byte("PEM"), 0644))

	cmd := exec.Command(binary, "init")
	cmd.Dir = repoDir
	cmd.Env = append(os.Environ(), "HOME="+t.TempDir())
	out, err := cmd.CombinedOutput()
	require.NoError(t, err, "yg init failed: %s", string(out))

	data, err := os.ReadFile(filepath.Join(repoDir, ".yggdrasil.toml"))
	require.NoError(t, err)
	content := string(data)

	assert.Contains(t, content, "certs/dev.pem", "should detect cert file")
}

// TestInit_ConfigHasComments verifies that the generated config contains
// helpful comments explaining each section.
func TestInit_ConfigHasComments(t *testing.T) {
	binary := buildBinary(t)
	repoDir := initTestRepo(t)

	cmd := exec.Command(binary, "init")
	cmd.Dir = repoDir
	cmd.Env = append(os.Environ(), "HOME="+t.TempDir())
	require.NoError(t, cmd.Run())

	data, err := os.ReadFile(filepath.Join(repoDir, ".yggdrasil.toml"))
	require.NoError(t, err)
	content := string(data)

	// Should have comments (lines starting with #)
	assert.Contains(t, content, "# Yggdrasil", "should have a header comment")
	assert.Contains(t, content, "# ", "should have inline comments")
	assert.Contains(t, content, "path_template", "should document path_template")
	assert.Contains(t, content, "copy", "should document copy")
	assert.Contains(t, content, "link", "should document link")
}
