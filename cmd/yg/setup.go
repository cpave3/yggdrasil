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

func setupCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "setup <branch>",
		Short: "Re-provision and re-hook an existing worktree",
		Long:  `Re-runs provisioning and post_create hooks on an existing worktree without recreating it. Does not run pre_create (which is a tree-creation guard). Useful for crash recovery and config changes.`,
		Args:  cobra.ExactArgs(1),
		RunE:  runSetup,
	}
	return cmd
}

func runSetup(cmd *cobra.Command, args []string) error {
	branch := args[0]

	primaryDir, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("getting current directory: %w", err)
	}

	cfg, err := config.Load(primaryDir)
	if err != nil {
		return fmt.Errorf("loading config: %w", err)
	}

	wtPath, err := worktree.ResolvePath(cfg.General.PathTemplate, primaryDir, branch)
	if err != nil {
		return fmt.Errorf("resolving worktree path: %w", err)
	}

	// Verify the worktree exists
	if _, err := os.Stat(wtPath); os.IsNotExist(err) {
		return fmt.Errorf("worktree for %s does not exist at %s — use 'yg new' to create it", branch, wtPath)
	}

	trunk := cfg.General.Trunk
	g := git.New(primaryDir)
	if trunk == "" {
		trunk, err = g.DetectTrunk()
		if err != nil {
			return fmt.Errorf("detecting trunk: %w", err)
		}
	}

	if dryRun {
		fmt.Printf("[dry-run] would re-provision worktree at %s\n", wtPath)
		_ = provision.Provision(provision.ProvisionConfig{
			PrimaryDir: primaryDir,
			TargetDir:  wtPath,
			Copy:       cfg.Provision.Copy,
			Link:       cfg.Provision.Link,
			DryRun:     true,
		})
		_ = hooks.Run(hooks.HookContext{
			Event:      hooks.PostCreate,
			Worktree:   wtPath,
			Primary:    primaryDir,
			Branch:     branch,
			Trunk:      trunk,
			Base:       "",
			Profile:    "human",
			AgentOwned: "0",
			BranchNew:  "0",
			Commands:   cfg.Hooks.PostCreate,
			DryRun:     true,
		})
		return nil
	}

	// Re-provision files (idempotent)
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
			Base:       "",
			Repo:       commonDirOr(primaryDir, primaryDir),
			Profile:    "human",
			AgentOwned: "0",
			BranchNew:  "0",
			Commands:   cfg.Hooks.PostCreate,
		}); err != nil {
			return fmt.Errorf("post_create hook failed: %w", err)
		}
	}

	fmt.Printf("Setup complete for %s at %s\n", branch, wtPath)
	return nil
}
