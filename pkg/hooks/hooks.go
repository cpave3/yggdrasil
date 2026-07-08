// Package hooks implements the lifecycle hook runner for Yggdrasil.
// Hooks run via `sh -c` with a defined env contract (FR-17) and CWD contract
// (FR-16). A failing hook stops execution and returns an error (FR-18).
package hooks

import (
	"fmt"
	"os"
	"os/exec"
)

// Event is a lifecycle hook event name.
type Event string

const (
	PreCreate  Event = "pre_create"
	PostCreate Event = "post_create"
	PreRemove  Event = "pre_remove"
	PostRemove Event = "post_remove"
	PreMerge   Event = "pre_merge"
	PostMerge  Event = "post_merge"
	PreSync    Event = "pre_sync"
	PostSync   Event = "post_sync"
)

// HookContext holds the environment and parameters for a hook execution.
type HookContext struct {
	Event      Event   // lifecycle event name
	Worktree   string  // new worktree path (YG_WORKTREE)
	Primary    string  // primary worktree path (YG_PRIMARY)
	Branch     string  // branch name (YG_BRANCH)
	Trunk      string  // trunk branch name (YG_TRUNK)
	Base       string  // base branch/commit the worktree was created from (YG_BASE)
	Repo       string  // git common dir (YG_REPO)
	Profile    string  // profile name (YG_PROFILE)
	AgentOwned string  // "1" or "0" (YG_AGENT_OWNED)
	BranchNew  string  // "1" if the branch was newly created, "0" if it pre-existed (YG_BRANCH_NEW)
	Commands   []string // ordered hook commands to run
	DryRun     bool     // if true, don't execute
}

// Run executes the hook commands in order via `sh -c`. It sets the env
// contract (FR-17) and CWD (FR-16) per the event type. A failing hook stops
// execution and returns an error; the worktree is left in place (FR-18).
func Run(ctx HookContext) error {
	if len(ctx.Commands) == 0 {
		return nil
	}

	cwd := ctx.Worktree
	// pre_create, pre_remove, and post_remove run in the primary (FR-16)
	if ctx.Event == PreCreate || ctx.Event == PreRemove || ctx.Event == PostRemove {
		cwd = ctx.Primary
	}

	env := buildEnv(ctx)

	for i, cmd := range ctx.Commands {
		if ctx.DryRun {
			fmt.Fprintf(os.Stderr, "  [%s] %s\n", ctx.Event, cmd)
			continue
		}

		ec := exec.Command("sh", "-c", cmd)
		ec.Dir = cwd
		ec.Env = env
		ec.Stdout = os.Stdout
		ec.Stderr = os.Stderr

		if err := ec.Run(); err != nil {
			return fmt.Errorf("hook [%s] command %d failed: %q: %w", ctx.Event, i+1, cmd, err)
		}
	}

	return nil
}

func buildEnv(ctx HookContext) []string {
	return append(os.Environ(),
		"YG_WORKTREE="+ctx.Worktree,
		"YG_PRIMARY="+ctx.Primary,
		"YG_BRANCH="+ctx.Branch,
		"YG_TRUNK="+ctx.Trunk,
		"YG_BASE="+ctx.Base,
		"YG_REPO="+ctx.Repo,
		"YG_PROFILE="+ctx.Profile,
		"YG_EVENT="+string(ctx.Event),
		"YG_AGENT_OWNED="+ctx.AgentOwned,
		"YG_BRANCH_NEW="+ctx.BranchNew,
	)
}
