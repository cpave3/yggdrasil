package git

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/cameron/yggdrasil/internal/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestCommonDir_ReturnsGitCommonDir verifies that CommonDir returns the
// .git common directory path for a repo.
func TestCommonDir_ReturnsGitCommonDir(t *testing.T) {
	repo := testutil.NewGitRepo(t)
	g := New(repo.Dir)

	commonDir, err := g.CommonDir()
	require.NoError(t, err)

	// CommonDir should be an absolute path containing .git
	assert.True(t, strings.HasSuffix(commonDir, ".git") || strings.Contains(commonDir, ".git"),
		"expected common dir to contain .git, got %s", commonDir)
	assert.True(t, filepath.IsAbs(commonDir), "expected absolute path, got %s", commonDir)
}

// TestCommonDir_Worktree_ReturnsCommonGitDir verifies that CommonDir returns
// the shared common dir (not the per-worktree .git file) when called from a
// linked worktree.
func TestCommonDir_Worktree_ReturnsCommonGitDir(t *testing.T) {
	repo := testutil.NewGitRepo(t)
	g := New(repo.Dir)

	wtPath := filepath.Join(repo.Dir, "..", "test-wt")
	err := g.WorktreeAdd("feature-x", wtPath, "main")
	require.NoError(t, err)

	g2 := New(wtPath)
	commonDir, err := g2.CommonDir()
	require.NoError(t, err)

	assert.Contains(t, commonDir, repo.Dir,
		"expected common dir to reference primary repo, got %s", commonDir)
}

// TestWorktreeList_ReturnsAllWorktrees verifies that WorktreeList returns
// all worktrees including the primary.
func TestWorktreeList_ReturnsAllWorktrees(t *testing.T) {
	repo := testutil.NewGitRepo(t)
	g := New(repo.Dir)

	wtPath := filepath.Join(repo.Dir, "..", "test-wt")
	err := g.WorktreeAdd("feature-a", wtPath, "main")
	require.NoError(t, err)

	worktrees, err := g.WorktreeList()
	require.NoError(t, err)

	assert.Len(t, worktrees, 2)
	assert.Equal(t, repo.Dir, worktrees[0].Path)
	assert.False(t, worktrees[0].Bare)
	assert.Equal(t, filepath.Clean(wtPath), filepath.Clean(worktrees[1].Path))
	assert.False(t, worktrees[1].Bare)
}

// TestWorktreeList_ParsesBranchName verifies that WorktreeList correctly
// parses the branch name from the porcelain output.
func TestWorktreeList_ParsesBranchName(t *testing.T) {
	repo := testutil.NewGitRepo(t)
	g := New(repo.Dir)

	wtPath := filepath.Join(repo.Dir, "..", "test-wt")
	err := g.WorktreeAdd("my-feature", wtPath, "main")
	require.NoError(t, err)

	worktrees, err := g.WorktreeList()
	require.NoError(t, err)

	var found bool
	for _, wt := range worktrees {
		if wt.Branch == "my-feature" {
			found = true
			break
		}
	}
	assert.True(t, found, "expected to find worktree with branch 'my-feature'")
}

// TestWorktreeRemove_RemovesDir verifies that WorktreeRemove deletes the
// worktree directory and removes it from the worktree list.
func TestWorktreeRemove_RemovesDir(t *testing.T) {
	repo := testutil.NewGitRepo(t)
	g := New(repo.Dir)

	wtPath := filepath.Join(repo.Dir, "..", "test-wt")
	err := g.WorktreeAdd("feature-a", wtPath, "main")
	require.NoError(t, err)

	err = g.WorktreeRemove(wtPath, false)
	require.NoError(t, err)

	worktrees, err := g.WorktreeList()
	require.NoError(t, err)
	assert.Len(t, worktrees, 1, "expected only primary worktree after remove")
}

// TestShowToplevel_ReturnsRepoRoot verifies that ShowToplevel returns the
// repository root when called from the primary working tree.
func TestShowToplevel_ReturnsRepoRoot(t *testing.T) {
	repo := testutil.NewGitRepo(t)
	g := New(repo.Dir)

	top, err := g.ShowToplevel()
	require.NoError(t, err)
	assert.Equal(t, repo.Dir, top)
}

// TestShowToplevel_FromSubdirectory verifies that ShowToplevel canonicalises
// a subdirectory up to the worktree root.
func TestShowToplevel_FromSubdirectory(t *testing.T) {
	repo := testutil.NewGitRepo(t)
	repo.WriteFile(t, "src/main.txt", "x")
	repo.Commit(t, "add src")

	sub := filepath.Join(repo.Dir, "src")
	g := New(sub)

	top, err := g.ShowToplevel()
	require.NoError(t, err)
	assert.Equal(t, repo.Dir, top)
}

// TestShowToplevel_WorktreeReturnsWorktreeRoot verifies that ShowToplevel
// returns the linked worktree's own root, not the primary repo root, when
// called from inside a linked worktree.
func TestShowToplevel_WorktreeReturnsWorktreeRoot(t *testing.T) {
	repo := testutil.NewGitRepo(t)
	g := New(repo.Dir)

	wtPath := filepath.Join(repo.Dir, "..", "test-wt")
	err := g.WorktreeAdd("feature-x", wtPath, "main")
	require.NoError(t, err)

	g2 := New(wtPath)
	top, err := g2.ShowToplevel()
	require.NoError(t, err)
	assert.Equal(t, filepath.Clean(wtPath), filepath.Clean(top))
}

// TestDetectTrunk_ReturnsMainBranch verifies that DetectTrunk returns "main"
// for a repo whose default branch is main.
func TestDetectTrunk_ReturnsMainBranch(t *testing.T) {
	repo := testutil.NewGitRepo(t)
	g := New(repo.Dir)

	trunk, err := g.DetectTrunk()
	require.NoError(t, err)
	assert.Equal(t, "main", trunk)
}

// TestWorktreePrune_CleansStaleEntry verifies that WorktreePrune removes the
// administrative entry for a worktree whose directory was deleted out-of-band.
func TestWorktreePrune_CleansStaleEntry(t *testing.T) {
	repo := testutil.NewGitRepo(t)
	g := New(repo.Dir)

	wtPath := filepath.Join(repo.Dir, "..", "prune-test")
	err := g.WorktreeAdd("feature-prune", wtPath, "main")
	require.NoError(t, err)

	// Delete the directory without going through git worktree remove.
	require.NoError(t, os.RemoveAll(wtPath))

	// The stale entry should still be listed (prune hasn't run yet).
	worktrees, err := g.WorktreeList()
	require.NoError(t, err)
	assert.Len(t, worktrees, 2, "stale entry should still be present before prune")

	// Prune and verify it's gone.
	require.NoError(t, g.WorktreePrune())
	worktrees, err = g.WorktreeList()
	require.NoError(t, err)
	assert.Len(t, worktrees, 1, "stale entry should be removed after prune")
}

// TestWorktreeLock_AndUnlock verifies that lock sets and unlock clears the
// locked state on a worktree.
func TestWorktreeLock_AndUnlock(t *testing.T) {
	repo := testutil.NewGitRepo(t)
	g := New(repo.Dir)

	wtPath := filepath.Join(repo.Dir, "..", "test-wt")
	err := g.WorktreeAdd("feature-a", wtPath, "main")
	require.NoError(t, err)

	err = g.WorktreeLock(wtPath, "agent-owned")
	require.NoError(t, err)

	worktrees, err := g.WorktreeList()
	require.NoError(t, err)
	var locked bool
	for _, wt := range worktrees {
		if filepath.Clean(wt.Path) == filepath.Clean(wtPath) {
			locked = wt.Locked
			break
		}
	}
	assert.True(t, locked, "expected worktree to be locked after WorktreeLock")

	err = g.WorktreeUnlock(wtPath)
	require.NoError(t, err)

	worktrees, err = g.WorktreeList()
	require.NoError(t, err)
	for _, wt := range worktrees {
		if filepath.Clean(wt.Path) == filepath.Clean(wtPath) {
			assert.False(t, wt.Locked, "expected worktree unlocked after WorktreeUnlock")
			break
		}
	}
}

// TestBranchExists_DetectsExistingBranch verifies BranchExists returns true
// for an existing branch and false for a non-existent one.
func TestBranchExists_DetectsExistingBranch(t *testing.T) {
	repo := testutil.NewGitRepo(t)
	g := New(repo.Dir)

	exists, err := g.BranchExists("main")
	require.NoError(t, err)
	assert.True(t, exists)

	exists, err = g.BranchExists("nonexistent")
	require.NoError(t, err)
	assert.False(t, exists)
}

// TestCreateBranch_CreatesNewBranch verifies that CreateBranch creates a new
// branch from the given base point without checking it out.
func TestCreateBranch_CreatesNewBranch(t *testing.T) {
	repo := testutil.NewGitRepo(t)
	g := New(repo.Dir)

	err := g.CreateBranch("new-feature", "main")
	require.NoError(t, err)

	exists, err := g.BranchExists("new-feature")
	require.NoError(t, err)
	assert.True(t, exists)
}

// TestIsDirty_CleanRepo verifies that IsDirty returns false for a clean repo.
func TestIsDirty_CleanRepo(t *testing.T) {
	repo := testutil.NewGitRepo(t)
	g := New(repo.Dir)

	dirty, err := g.IsDirty(repo.Dir)
	require.NoError(t, err)
	assert.False(t, dirty)
}

// TestIsDirty_DirtyRepo verifies that IsDirty returns true when there are
// uncommitted changes.
func TestIsDirty_DirtyRepo(t *testing.T) {
	repo := testutil.NewGitRepo(t)
	g := New(repo.Dir)

	repo.WriteFile(t, "new-file.txt", "untracked")
	dirty, err := g.IsDirty(repo.Dir)
	require.NoError(t, err)
	assert.True(t, dirty)
}

// TestAheadBehind_BranchWithCommits verifies that AheadBehind returns correct
// counts for a branch that is ahead of trunk.
func TestAheadBehind_BranchAhead(t *testing.T) {
	repo := testutil.NewGitRepo(t)
	g := New(repo.Dir)

	// Create a feature branch with one commit
	repo.WriteFile(t, "feature.txt", "feature data")
	repo.Commit(t, "feature commit")

	// We're on main, but the HEAD is now ahead. Let's use branches properly.
	// Reset to main first
	repo.RunGit("checkout", "main")
	repo.RunGit("branch", "feat")
	repo.RunGit("checkout", "feat")
	repo.WriteFile(t, "feat.txt", "feat")
	repo.Commit(t, "feat commit")

	ahead, behind, err := g.AheadBehind("feat", "main")
	require.NoError(t, err)
	assert.Equal(t, 1, ahead, "expected 1 commit ahead")
	assert.Equal(t, 0, behind, "expected 0 commits behind")
}

// TestDeleteBranch verifies that DeleteBranch removes a branch ref.
func TestDeleteBranch(t *testing.T) {
	repo := testutil.NewGitRepo(t)
	g := New(repo.Dir)

	g.CreateBranch("to-delete", "main")
	exists, _ := g.BranchExists("to-delete")
	assert.True(t, exists)

	err := g.DeleteBranch("to-delete", false)
	require.NoError(t, err)

	exists, _ = g.BranchExists("to-delete")
	assert.False(t, exists)
}

// TestIsMerged_MergedBranch verifies that IsMerged returns true for a branch
// that is fully merged into the target.
func TestIsMerged_MergedBranch(t *testing.T) {
	repo := testutil.NewGitRepo(t)
	g := New(repo.Dir)

	// Create a branch and merge it back
	g.CreateBranch("feature", "main")
	repo.RunGit("checkout", "feature")
	repo.WriteFile(t, "feat.txt", "data")
	repo.Commit(t, "feature work")
	repo.RunGit("checkout", "main")
	repo.RunGit("merge", "feature")

	merged, err := g.IsMerged("feature", "main")
	require.NoError(t, err)
	assert.True(t, merged)
}

// TestIsMerged_UnmergedBranch verifies that IsMerged returns false for a
// branch that has commits not in the target.
func TestIsMerged_UnmergedBranch(t *testing.T) {
	repo := testutil.NewGitRepo(t)
	g := New(repo.Dir)

	g.CreateBranch("feature", "main")
	repo.RunGit("checkout", "feature")
	repo.WriteFile(t, "feat.txt", "data")
	repo.Commit(t, "feature work")

	merged, err := g.IsMerged("feature", "main")
	require.NoError(t, err)
	assert.False(t, merged)
}
