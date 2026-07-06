package main

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/cameron/yggdrasil/pkg/config"
	"github.com/cameron/yggdrasil/pkg/git"
	"github.com/cameron/yggdrasil/pkg/hooks"
	"github.com/cameron/yggdrasil/pkg/worktree"
	"github.com/spf13/cobra"
)

var (
	removeForce        bool
	removeDeleteBranch bool
)

func removeCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "remove <branch>",
		Short: "Remove a worktree with safety checks",
		Long:  `Runs pre_remove hooks, removes the worktree, then runs post_remove hooks. Aborts on dirty worktree unless --force. Deletes branch only on --delete-branch or if merged.`,
		Args:  cobra.ExactArgs(1),
		RunE:  runRemove,
	}

	cmd.Flags().BoolVar(&removeForce, "force", false, "override dirty-tree safety check")
	cmd.Flags().BoolVar(&removeDeleteBranch, "delete-branch", false, "delete the branch ref after removing the worktree")

	return cmd
}

func runRemove(cmd *cobra.Command, args []string) error {
	branch := args[0]

	primaryDir, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("getting current directory: %w", err)
	}

	cfg, err := config.Load(primaryDir)
	if err != nil {
		return fmt.Errorf("loading config: %w", err)
	}

	// Resolve worktree path
	wtPath, err := worktree.ResolvePath(cfg.General.PathTemplate, primaryDir, branch)
	if err != nil {
		return fmt.Errorf("resolving worktree path: %w", err)
	}

	g := git.New(primaryDir)

	// Detect trunk
	trunk := cfg.General.Trunk
	if trunk == "" {
		trunk, err = g.DetectTrunk()
		if err != nil {
			return fmt.Errorf("detecting trunk: %w", err)
		}
	}

	// Check if worktree exists
	worktrees, err := g.WorktreeList()
	if err != nil {
		return fmt.Errorf("listing worktrees: %w", err)
	}

	var wt *git.Worktree
	for i := range worktrees {
		if filepath.Clean(worktrees[i].Path) == filepath.Clean(wtPath) {
			wt = &worktrees[i]
			break
		}
	}
	if wt == nil {
		return fmt.Errorf("no worktree found for branch %s at %s", branch, wtPath)
	}

	// Safety check: dirty worktree
	if !removeForce {
		dirty, err := g.IsDirty(wtPath)
		if err != nil {
			return fmt.Errorf("checking worktree status: %w", err)
		}
		if dirty {
			return fmt.Errorf("worktree is dirty — use --force to remove anyway")
		}
	}

	// Unlock if locked (agent-owned)
	if wt.Locked {
		if err := g.WorktreeUnlock(wtPath); err != nil {
			fmt.Fprintf(os.Stderr, "warning: failed to unlock worktree: %v\n", err)
		}
	}

	// Run pre_remove hooks (CWD: primary)
	agentOwned := "0"
	if wt.Locked {
		agentOwned = "1"
	}
	if len(cfg.Hooks.PreRemove) > 0 {
		if err := hooks.Run(hooks.HookContext{
			Event:      hooks.PreRemove,
			Worktree:   wtPath,
			Primary:    primaryDir,
			Branch:     branch,
			Trunk:      trunk,
			Repo:       commonDirOr(primaryDir, primaryDir),
			Profile:    "human",
			AgentOwned: agentOwned,
			Commands:   cfg.Hooks.PreRemove,
		}); err != nil {
			return fmt.Errorf("pre_remove hook failed: %w", err)
		}
	}

	// Remove worktree. We always pass --force to git because provisioning
	// and hooks may have created files that git sees as untracked. Our own
	// dirty check above is the real safety gate unless --force was passed.
	if err := g.WorktreeRemove(wtPath, true); err != nil {
		return fmt.Errorf("removing worktree: %w", err)
	}

	// Delete branch if requested or if merged
	if removeDeleteBranch {
		if err := g.DeleteBranch(branch, true); err != nil {
			fmt.Fprintf(os.Stderr, "warning: failed to delete branch %s: %v\n", branch, err)
		}
	}

	// Run post_remove hooks (CWD: primary, worktree is gone)
	if len(cfg.Hooks.PostRemove) > 0 {
		if err := hooks.Run(hooks.HookContext{
			Event:      hooks.PostRemove,
			Worktree:   wtPath,
			Primary:    primaryDir,
			Branch:     branch,
			Trunk:      trunk,
			Repo:       commonDirOr(primaryDir, primaryDir),
			Profile:    "human",
			AgentOwned: agentOwned,
			Commands:   cfg.Hooks.PostRemove,
		}); err != nil {
			return fmt.Errorf("post_remove hook failed: %w", err)
		}
	}

	fmt.Printf("Removed worktree for %s\n", branch)
	return nil
}
