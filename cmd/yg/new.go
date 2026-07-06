package main

import (
	"fmt"
	"os"

	"github.com/cameron/yggdrasil/pkg/config"
	"github.com/cameron/yggdrasil/pkg/git"
	"github.com/cameron/yggdrasil/pkg/hooks"
	"github.com/cameron/yggdrasil/pkg/provision"
	"github.com/cameron/yggdrasil/pkg/worktree"
	"github.com/spf13/cobra"
)

var (
	newAgentOwned bool
	newPrintPath  bool
	newBase       string
)

func newCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "new <branch> [base]",
		Short: "Create and provision a new worktree",
		Long:  `Creates a branch (if needed) and a worktree, runs pre_create hooks, provisions files, runs post_create hooks, then prints the path.`,
		Args:  cobra.RangeArgs(1, 2),
		RunE:  runNew,
	}

	cmd.Flags().BoolVar(&newAgentOwned, "agent-owned", false, "lock the worktree to prevent external pruning (sets YG_AGENT_OWNED=1)")
	cmd.Flags().BoolVar(&newPrintPath, "print-path", false, "print only the worktree path (no other output)")
	cmd.Flags().StringVar(&newBase, "base", "", "base branch/commit to create from (default: trunk)")

	return cmd
}

func runNew(cmd *cobra.Command, args []string) error {
	branch := args[0]
	base := newBase
	if len(args) >= 2 {
		base = args[1]
	}

	// Determine the primary worktree (cwd)
	primaryDir, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("getting current directory: %w", err)
	}

	// Load config
	cfg, err := config.Load(primaryDir)
	if err != nil {
		return fmt.Errorf("loading config: %w", err)
	}

	// Resolve worktree path
	wtPath, err := worktree.ResolvePath(cfg.General.PathTemplate, primaryDir, branch)
	if err != nil {
		return fmt.Errorf("resolving worktree path: %w", err)
	}

	// Detect trunk if not specified
	trunk := cfg.General.Trunk
	if trunk == "" {
		g := git.New(primaryDir)
		trunk, err = g.DetectTrunk()
		if err != nil {
			return fmt.Errorf("detecting trunk: %w", err)
		}
	}
	if base == "" {
		base = trunk
	}

	g := git.New(primaryDir)

	if dryRun {
		fmt.Printf("[dry-run] would create worktree at %s\n", wtPath)
		fmt.Printf("[dry-run] branch: %s (base: %s)\n", branch, base)
		if len(cfg.Provision.Copy) > 0 {
			fmt.Printf("[dry-run] provisioning copy: %v\n", cfg.Provision.Copy)
		}
		if len(cfg.Provision.Link) > 0 {
			fmt.Printf("[dry-run] provisioning link: %v\n", cfg.Provision.Link)
		}
		if len(cfg.Hooks.PreCreate) > 0 {
			fmt.Printf("[dry-run] pre_create hooks: %v\n", cfg.Hooks.PreCreate)
		}
		if len(cfg.Hooks.PostCreate) > 0 {
			fmt.Printf("[dry-run] post_create hooks: %v\n", cfg.Hooks.PostCreate)
		}

		// Dry-run provisioning (shows what would be copied)
		_ = provision.Provision(provision.ProvisionConfig{
			PrimaryDir: primaryDir,
			TargetDir:  wtPath,
			Copy:       cfg.Provision.Copy,
			Link:       cfg.Provision.Link,
			DryRun:     true,
		})

		// Dry-run hooks
		agentOwned := "0"
		if newAgentOwned {
			agentOwned = "1"
		}
		_ = hooks.Run(hooks.HookContext{
			Event:      hooks.PreCreate,
			Worktree:   wtPath,
			Primary:    primaryDir,
			Branch:     branch,
			Trunk:      trunk,
			Profile:    "human",
			AgentOwned: agentOwned,
			Commands:   cfg.Hooks.PreCreate,
			DryRun:     true,
		})
		_ = hooks.Run(hooks.HookContext{
			Event:      hooks.PostCreate,
			Worktree:   wtPath,
			Primary:    primaryDir,
			Branch:     branch,
			Trunk:      trunk,
			Profile:    "human",
			AgentOwned: agentOwned,
			Commands:   cfg.Hooks.PostCreate,
			DryRun:     true,
		})

		if newPrintPath {
			fmt.Println(wtPath)
		}
		return nil
	}

	// Check if branch exists, create if not
	exists, err := g.BranchExists(branch)
	if err != nil {
		return fmt.Errorf("checking branch: %w", err)
	}

	if !exists {
		if err := g.CreateBranch(branch, base); err != nil {
			return fmt.Errorf("creating branch %s from %s: %w", branch, base, err)
		}
	}

	// Create worktree
	if err := g.WorktreeAdd(branch, wtPath, base); err != nil {
		// If the branch already exists, try without -b (just add worktree)
		if err2 := g.WorktreeAddExisting(branch, wtPath); err2 != nil {
			return fmt.Errorf("creating worktree: %w", err)
		}
	}

	// Lock if agent-owned
	agentOwned := "0"
	if newAgentOwned {
		agentOwned = "1"
		if err := g.WorktreeLock(wtPath, "agent-owned"); err != nil {
			fmt.Fprintf(os.Stderr, "warning: failed to lock worktree: %v\n", err)
		}
	}

	// Run pre_create hooks (CWD: primary)
	if len(cfg.Hooks.PreCreate) > 0 {
		if err := hooks.Run(hooks.HookContext{
			Event:      hooks.PreCreate,
			Worktree:   wtPath,
			Primary:    primaryDir,
			Branch:     branch,
			Trunk:      trunk,
			Repo:       commonDirOr(primaryDir, primaryDir),
			Profile:    "human",
			AgentOwned: agentOwned,
			Commands:   cfg.Hooks.PreCreate,
		}); err != nil {
			return fmt.Errorf("pre_create hook failed: %w", err)
		}
	}

	// Provision files
	if len(cfg.Provision.Copy) > 0 || len(cfg.Provision.Link) > 0 {
		if err := provision.Provision(provision.ProvisionConfig{
			PrimaryDir: primaryDir,
			TargetDir:  wtPath,
			Copy:       cfg.Provision.Copy,
			Link:       cfg.Provision.Link,
		}); err != nil {
			return fmt.Errorf("provisioning failed: %w", err)
		}
	}

	// Run post_create hooks (CWD: worktree)
	if len(cfg.Hooks.PostCreate) > 0 {
		if err := hooks.Run(hooks.HookContext{
			Event:      hooks.PostCreate,
			Worktree:   wtPath,
			Primary:    primaryDir,
			Branch:     branch,
			Trunk:      trunk,
			Repo:       commonDirOr(primaryDir, primaryDir),
			Profile:    "human",
			AgentOwned: agentOwned,
			Commands:   cfg.Hooks.PostCreate,
		}); err != nil {
			return fmt.Errorf("post_create hook failed: %w", err)
		}
	}

	if newPrintPath {
		fmt.Println(wtPath)
	} else {
		fmt.Printf("Created worktree for %s at %s\n", branch, wtPath)
	}

	return nil
}

func commonDirOr(repoPath, fallback string) string {
	g := git.New(repoPath)
	commonDir, err := g.CommonDir()
	if err != nil {
		return fallback
	}
	return commonDir
}
