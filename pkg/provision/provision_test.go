package provision

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestProvision_CopiesFile verifies that provisioning a copy pattern copies
// the file from primary to the new worktree.
func TestProvision_CopiesFile(t *testing.T) {
	primary := t.TempDir()
	target := t.TempDir()

	// Create source file in primary
	require.NoError(t, os.WriteFile(filepath.Join(primary, ".env"), []byte("KEY=value"), 0644))

	err := Provision(ProvisionConfig{
		PrimaryDir: primary,
		TargetDir:  target,
		Copy:       []string{".env"},
		Link:       nil,
	})
	require.NoError(t, err)

	content, err := os.ReadFile(filepath.Join(target, ".env"))
	require.NoError(t, err)
	assert.Equal(t, "KEY=value", string(content))
}

// TestProvision_CopySymlinkLinksInternalTarget verifies that a literal copy
// pattern matching a symlink recreates the symlink against the target worktree.
func TestProvision_CopySymlinkLinksInternalTarget(t *testing.T) {
	primary := t.TempDir()
	target := t.TempDir()

	require.NoError(t, os.WriteFile(filepath.Join(primary, ".env.source"), []byte("source"), 0644))
	require.NoError(t, os.Symlink(filepath.Join(primary, ".env.source"), filepath.Join(primary, ".env.local")))

	err := Provision(ProvisionConfig{
		PrimaryDir: primary,
		TargetDir:  target,
		Copy:       []string{".env.local"},
	})
	require.NoError(t, err)

	linkPath := filepath.Join(target, ".env.local")
	info, err := os.Lstat(linkPath)
	require.NoError(t, err)
	assert.True(t, info.Mode()&os.ModeSymlink != 0, "expected symlink")

	linkTarget, err := os.Readlink(linkPath)
	require.NoError(t, err)
	assert.Equal(t, filepath.Join(target, ".env.source"), linkTarget)
}

// TestProvision_CreatesParentDirectories verifies that parent directories are
// created as needed for nested copy targets.
func TestProvision_CreatesParentDirectories(t *testing.T) {
	primary := t.TempDir()
	target := t.TempDir()

	require.NoError(t, os.MkdirAll(filepath.Join(primary, "config"), 0755))
	require.NoError(t, os.WriteFile(filepath.Join(primary, "config", "app.json"), []byte(`{"a":1}`), 0644))

	err := Provision(ProvisionConfig{
		PrimaryDir: primary,
		TargetDir:  target,
		Copy:       []string{"config/app.json"},
	})
	require.NoError(t, err)

	content, err := os.ReadFile(filepath.Join(target, "config", "app.json"))
	require.NoError(t, err)
	assert.Equal(t, `{"a":1}`, string(content))
}

// TestProvision_RecursiveCopy verifies that copying a directory copies all
// files recursively.
func TestProvision_RecursiveCopy(t *testing.T) {
	primary := t.TempDir()
	target := t.TempDir()

	require.NoError(t, os.MkdirAll(filepath.Join(primary, "config", "sub"), 0755))
	require.NoError(t, os.WriteFile(filepath.Join(primary, "config", "a.json"), []byte("a"), 0644))
	require.NoError(t, os.WriteFile(filepath.Join(primary, "config", "sub", "b.json"), []byte("b"), 0644))

	err := Provision(ProvisionConfig{
		PrimaryDir: primary,
		TargetDir:  target,
		Copy:       []string{"config"},
	})
	require.NoError(t, err)

	a, err := os.ReadFile(filepath.Join(target, "config", "a.json"))
	require.NoError(t, err)
	assert.Equal(t, "a", string(a))

	b, err := os.ReadFile(filepath.Join(target, "config", "sub", "b.json"))
	require.NoError(t, err)
	assert.Equal(t, "b", string(b))
}

// TestProvision_RecursiveCopyLinksInternalSymlinkTarget verifies that
// directory copy recreates symlinks and rewrites primary-internal targets to
// the equivalent path in the target worktree.
func TestProvision_RecursiveCopyLinksInternalSymlinkTarget(t *testing.T) {
	primary := t.TempDir()
	target := t.TempDir()

	require.NoError(t, os.MkdirAll(filepath.Join(primary, "config"), 0755))
	require.NoError(t, os.WriteFile(filepath.Join(primary, "config", "app.json"), []byte("app"), 0644))
	require.NoError(t, os.WriteFile(filepath.Join(primary, ".env.source"), []byte("secret"), 0644))
	require.NoError(t, os.Symlink(filepath.Join(primary, ".env.source"), filepath.Join(primary, "config", ".env.local")))

	err := Provision(ProvisionConfig{
		PrimaryDir: primary,
		TargetDir:  target,
		Copy:       []string{"config"},
	})
	require.NoError(t, err)

	content, err := os.ReadFile(filepath.Join(target, "config", "app.json"))
	require.NoError(t, err)
	assert.Equal(t, "app", string(content))

	linkPath := filepath.Join(target, "config", ".env.local")
	info, err := os.Lstat(linkPath)
	require.NoError(t, err)
	assert.True(t, info.Mode()&os.ModeSymlink != 0, "expected symlink")

	linkTarget, err := os.Readlink(linkPath)
	require.NoError(t, err)
	assert.Equal(t, filepath.Join(target, ".env.source"), linkTarget)
}

// TestProvision_RecursiveCopyLinksExternalSymlinkTarget verifies that
// symlink targets outside the primary worktree keep their resolved target.
func TestProvision_RecursiveCopyLinksExternalSymlinkTarget(t *testing.T) {
	primary := t.TempDir()
	target := t.TempDir()
	outside := t.TempDir()

	require.NoError(t, os.MkdirAll(filepath.Join(primary, "config"), 0755))
	require.NoError(t, os.WriteFile(filepath.Join(outside, ".env.source"), []byte("secret"), 0644))
	require.NoError(t, os.Symlink(filepath.Join(outside, ".env.source"), filepath.Join(primary, "config", ".env.local")))

	err := Provision(ProvisionConfig{
		PrimaryDir: primary,
		TargetDir:  target,
		Copy:       []string{"config"},
	})
	require.NoError(t, err)

	linkPath := filepath.Join(target, "config", ".env.local")
	info, err := os.Lstat(linkPath)
	require.NoError(t, err)
	assert.True(t, info.Mode()&os.ModeSymlink != 0, "expected symlink")

	linkTarget, err := os.Readlink(linkPath)
	require.NoError(t, err)
	assert.Equal(t, filepath.Join(outside, ".env.source"), linkTarget)
}

// TestProvision_LinksFile verifies that provisioning a link pattern creates a
// symlink in the target pointing to the primary.
func TestProvision_LinksFile(t *testing.T) {
	primary := t.TempDir()
	target := t.TempDir()

	require.NoError(t, os.WriteFile(filepath.Join(primary, "cert.pem"), []byte("cert"), 0644))

	err := Provision(ProvisionConfig{
		PrimaryDir: primary,
		TargetDir:  target,
		Link:       []string{"cert.pem"},
	})
	require.NoError(t, err)

	info, err := os.Lstat(filepath.Join(target, "cert.pem"))
	require.NoError(t, err)
	assert.True(t, info.Mode()&os.ModeSymlink != 0, "expected symlink")
}

// TestProvision_Idempotent verifies that re-running provisioning doesn't error
// on files that already exist.
func TestProvision_Idempotent(t *testing.T) {
	primary := t.TempDir()
	target := t.TempDir()

	require.NoError(t, os.WriteFile(filepath.Join(primary, ".env"), []byte("KEY=value"), 0644))

	pc := ProvisionConfig{
		PrimaryDir: primary,
		TargetDir:  target,
		Copy:       []string{".env"},
	}

	// First run
	require.NoError(t, Provision(pc))

	// Second run should not error
	require.NoError(t, Provision(pc))

	// Verify file still exists and is correct
	content, err := os.ReadFile(filepath.Join(target, ".env"))
	require.NoError(t, err)
	assert.Equal(t, "KEY=value", string(content))
}

// TestProvision_DryRun verifies that dry-run mode doesn't write any files.
func TestProvision_DryRun(t *testing.T) {
	primary := t.TempDir()
	target := t.TempDir()

	require.NoError(t, os.WriteFile(filepath.Join(primary, ".env"), []byte("KEY=value"), 0644))

	err := Provision(ProvisionConfig{
		PrimaryDir: primary,
		TargetDir:  target,
		Copy:       []string{".env"},
		DryRun:     true,
	})
	require.NoError(t, err)

	// File should not exist in target
	_, err = os.Stat(filepath.Join(target, ".env"))
	assert.True(t, os.IsNotExist(err), "expected .env to not exist in target during dry run")
}

// TestProvision_GlobPattern verifies that glob patterns like .env.* match
// multiple files.
func TestProvision_GlobPattern(t *testing.T) {
	primary := t.TempDir()
	target := t.TempDir()

	require.NoError(t, os.WriteFile(filepath.Join(primary, ".env"), []byte("base"), 0644))
	require.NoError(t, os.WriteFile(filepath.Join(primary, ".env.local"), []byte("local"), 0644))
	require.NoError(t, os.WriteFile(filepath.Join(primary, ".env.production"), []byte("prod"), 0644))

	err := Provision(ProvisionConfig{
		PrimaryDir: primary,
		TargetDir:  target,
		Copy:       []string{".env.*"},
	})
	require.NoError(t, err)

	// .env should NOT be copied (pattern is .env.*, not .env)
	_, err = os.Stat(filepath.Join(target, ".env"))
	assert.True(t, os.IsNotExist(err), ".env should not be matched by .env.*")

	// .env.local and .env.production should be copied
	content, err := os.ReadFile(filepath.Join(target, ".env.local"))
	require.NoError(t, err)
	assert.Equal(t, "local", string(content))

	content, err = os.ReadFile(filepath.Join(target, ".env.production"))
	require.NoError(t, err)
	assert.Equal(t, "prod", string(content))
}

// TestProvision_GlobPatternLinksSymlinks verifies that globbed copy sources
// recreate symlink matches.
func TestProvision_GlobPatternLinksSymlinks(t *testing.T) {
	primary := t.TempDir()
	target := t.TempDir()

	require.NoError(t, os.WriteFile(filepath.Join(primary, ".env.production"), []byte("prod"), 0644))
	require.NoError(t, os.WriteFile(filepath.Join(primary, ".env.source"), []byte("source"), 0644))
	require.NoError(t, os.Symlink(filepath.Join(primary, ".env.source"), filepath.Join(primary, ".env.local")))

	err := Provision(ProvisionConfig{
		PrimaryDir: primary,
		TargetDir:  target,
		Copy:       []string{".env.*"},
	})
	require.NoError(t, err)

	content, err := os.ReadFile(filepath.Join(target, ".env.production"))
	require.NoError(t, err)
	assert.Equal(t, "prod", string(content))

	linkPath := filepath.Join(target, ".env.local")
	info, err := os.Lstat(linkPath)
	require.NoError(t, err)
	assert.True(t, info.Mode()&os.ModeSymlink != 0, "expected symlink")

	linkTarget, err := os.Readlink(linkPath)
	require.NoError(t, err)
	assert.Equal(t, filepath.Join(target, ".env.source"), linkTarget)
}
