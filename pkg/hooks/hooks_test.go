package hooks

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestRun_ExecutesCommand verifies that a hook command runs via sh -c and
// can write to the worktree directory.
func TestRun_ExecutesCommand(t *testing.T) {
	worktree := t.TempDir()
	ctx := HookContext{
		Event:    PostCreate,
		Worktree: worktree,
		Commands: []string{`echo hello > marker.txt`},
	}

	err := Run(ctx)
	require.NoError(t, err)

	content, err := os.ReadFile(filepath.Join(worktree, "marker.txt"))
	require.NoError(t, err)
	assert.Equal(t, "hello\n", string(content))
}

// TestRun_EnvContract verifies that all YG_ env vars are set in the hook's
// environment.
func TestRun_EnvContract(t *testing.T) {
	worktree := t.TempDir()
	primary := t.TempDir()
	repo := t.TempDir()

	ctx := HookContext{
		Event:      PostCreate,
		Worktree:   worktree,
		Primary:    primary,
		Branch:     "feature-x",
		Trunk:      "main",
		Base:       "dev-branch",
		Repo:       repo,
		Profile:    "human",
		AgentOwned: "0",
		BranchNew:  "1",
		Commands:   []string{`env > env.txt`},
	}

	err := Run(ctx)
	require.NoError(t, err)

	content, err := os.ReadFile(filepath.Join(worktree, "env.txt"))
	require.NoError(t, err)
	envOutput := string(content)

	assert.Contains(t, envOutput, "YG_WORKTREE="+worktree)
	assert.Contains(t, envOutput, "YG_PRIMARY="+primary)
	assert.Contains(t, envOutput, "YG_BRANCH=feature-x")
	assert.Contains(t, envOutput, "YG_TRUNK=main")
	assert.Contains(t, envOutput, "YG_BASE=dev-branch")
	assert.Contains(t, envOutput, "YG_REPO="+repo)
	assert.Contains(t, envOutput, "YG_PROFILE=human")
	assert.Contains(t, envOutput, "YG_EVENT=post_create")
	assert.Contains(t, envOutput, "YG_AGENT_OWNED=0")
	assert.Contains(t, envOutput, "YG_BRANCH_NEW=1")
}

// TestRun_CWDPostCreate verifies that post_create hooks run in the worktree
// root (FR-16).
func TestRun_CWDPostCreate(t *testing.T) {
	worktree := t.TempDir()
	ctx := HookContext{
		Event:    PostCreate,
		Worktree: worktree,
		Commands: []string{`pwd > cwd.txt`},
	}

	err := Run(ctx)
	require.NoError(t, err)

	content, err := os.ReadFile(filepath.Join(worktree, "cwd.txt"))
	require.NoError(t, err)
	assert.Contains(t, string(content), worktree)
}

// TestRun_CWDPreCreate verifies that pre_create hooks run in the primary
// worktree (FR-16).
func TestRun_CWDPreCreate(t *testing.T) {
	worktree := t.TempDir()
	primary := t.TempDir()
	ctx := HookContext{
		Event:    PreCreate,
		Worktree: worktree,
		Primary:  primary,
		Commands: []string{`pwd > cwd.txt`},
	}

	err := Run(ctx)
	require.NoError(t, err)

	content, err := os.ReadFile(filepath.Join(primary, "cwd.txt"))
	require.NoError(t, err)
	assert.Contains(t, string(content), primary)
}

// TestRun_OrderedExecution verifies that hooks run in order.
func TestRun_OrderedExecution(t *testing.T) {
	worktree := t.TempDir()
	ctx := HookContext{
		Event:    PostCreate,
		Worktree: worktree,
		Commands: []string{
			`echo first > order.txt`,
			`echo second >> order.txt`,
			`echo third >> order.txt`,
		},
	}

	err := Run(ctx)
	require.NoError(t, err)

	content, err := os.ReadFile(filepath.Join(worktree, "order.txt"))
	require.NoError(t, err)
	assert.Equal(t, "first\nsecond\nthird\n", string(content))
}

// TestRun_FailureStopsExecution verifies that a failing hook stops execution
// of subsequent hooks and returns an error (FR-18).
func TestRun_FailureStopsExecution(t *testing.T) {
	worktree := t.TempDir()
	ctx := HookContext{
		Event:    PostCreate,
		Worktree: worktree,
		Commands: []string{
			`echo before > log.txt`,
			`exit 1`,
			`echo after >> log.txt`,
		},
	}

	err := Run(ctx)
	require.Error(t, err)

	content, err := os.ReadFile(filepath.Join(worktree, "log.txt"))
	require.NoError(t, err)
	assert.Equal(t, "before\n", string(content), "third hook should not have run")
}

// TestRun_EmptyCommandsIsNoop verifies that running with no commands is a no-op.
func TestRun_EmptyCommandsIsNoop(t *testing.T) {
	ctx := HookContext{
		Event:    PostCreate,
		Worktree: t.TempDir(),
		Commands: nil,
	}

	err := Run(ctx)
	require.NoError(t, err)
}

// TestRun_DryRunDoesNotExecute verifies that dry-run mode doesn't execute hooks.
func TestRun_DryRunDoesNotExecute(t *testing.T) {
	worktree := t.TempDir()
	ctx := HookContext{
		Event:    PostCreate,
		Worktree: worktree,
		Commands: []string{`echo hello > marker.txt`},
		DryRun:   true,
	}

	err := Run(ctx)
	require.NoError(t, err)

	_, err = os.Stat(filepath.Join(worktree, "marker.txt"))
	assert.True(t, os.IsNotExist(err), "hook should not have run in dry-run mode")
}
