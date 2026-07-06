package worktree

import (
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestResolvePath_DefaultTemplate verifies that the default template
// ../{repo}.{branch} resolves to a sibling directory.
func TestResolvePath_DefaultTemplate(t *testing.T) {
	primaryDir := "/repos/myproject"
	path, err := ResolvePath("../{repo}.{branch}", primaryDir, "feature-x")
	require.NoError(t, err)

	expected := filepath.Join(primaryDir, "..", "myproject.feature-x")
	assert.Equal(t, filepath.Clean(expected), filepath.Clean(path))
}

// TestResolvePath_CustomTemplate verifies that a custom template with a
// sibling worktrees/ directory resolves correctly.
func TestResolvePath_CustomTemplate(t *testing.T) {
	primaryDir := "/repos/myproject"
	path, err := ResolvePath("../worktrees/{branch}", primaryDir, "feature-x")
	require.NoError(t, err)

	expected := filepath.Join(primaryDir, "..", "worktrees", "feature-x")
	assert.Equal(t, filepath.Clean(expected), filepath.Clean(path))
}

// TestResolvePath_AbsoluteTemplateRejected verifies that a template starting
// with / is rejected to prevent writing outside the expected area.
func TestResolvePath_AbsoluteTemplateRejected(t *testing.T) {
	_, err := ResolvePath("/tmp/{branch}", "/repos/myproject", "feature-x")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "absolute")
}

// TestResolvePath_TraversalRejected verifies that a template with .. that
// escapes the parent directory is rejected.
func TestResolvePath_TraversalRejected(t *testing.T) {
	_, err := ResolvePath("../../{branch}", "/repos/myproject", "feature-x")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "traversal")
}

// TestResolvePath_BranchWithSlash verifies that a branch name with a slash
// (e.g. feature/x) produces a valid nested path.
func TestResolvePath_BranchWithSlash(t *testing.T) {
	primaryDir := "/repos/myproject"
	path, err := ResolvePath("../{repo}.{branch}", primaryDir, "feature/x")
	require.NoError(t, err)

	expected := filepath.Join(primaryDir, "..", "myproject.feature-x")
	assert.Equal(t, filepath.Clean(expected), filepath.Clean(path))
}
