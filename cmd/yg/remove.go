package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"syscall"

	"github.com/cameron/yggdrasil/pkg/config"
	"github.com/cameron/yggdrasil/pkg/git"
	"github.com/cameron/yggdrasil/pkg/hooks"
	"github.com/cameron/yggdrasil/pkg/worktree"
	"github.com/spf13/cobra"
)

var (
	removeForce        bool
	removeDeleteBranch bool
	removeCurrent      bool
)

func removeCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "remove <branch>",
		Short: "Remove a worktree with safety checks",
		Long: `Runs pre_remove hooks, removes the worktree, then runs post_remove hooks. Aborts on dirty worktree unless --force. Deletes branch only on --delete-branch or if merged.

Use --current to remove the worktree containing the current directory instead of naming a branch. The branch ref is preserved unless --delete-branch is also given.`,
		Args: func(cmd *cobra.Command, args []string) error {
			if removeCurrent {
				return cobra.NoArgs(cmd, args)
			}
			return cobra.ExactArgs(1)(cmd, args)
		},
		RunE: runRemove,
	}

	cmd.Flags().BoolVar(&removeForce, "force", false, "override dirty-tree safety check")
	cmd.Flags().BoolVar(&removeDeleteBranch, "delete-branch", false, "delete the branch ref after removing the worktree")
	cmd.Flags().BoolVar(&removeCurrent, "current", false, "remove the worktree containing the current directory (branch is preserved)")

	return cmd
}

// runRemove resolves the target worktree then delegates to removeWorktree.
// The resolution strategy depends on whether --current was supplied: the
// branch path resolves a worktree path from a branch name, while the
// --current path discovers the worktree from the current directory.
func runRemove(cmd *cobra.Command, args []string) error {
	branch := ""
	wtPath := ""
	primaryDir := ""
	var wt *git.Worktree
	var cfg *config.Config

	if removeCurrent {
		cwd, err := os.Getwd()
		if err != nil {
			return fmt.Errorf("getting current directory: %w", err)
		}

		gCwd := git.New(cwd)
		top, err := gCwd.ShowToplevel()
		if err != nil {
			return fmt.Errorf("locating worktree root: %w", err)
		}

		worktrees, err := gCwd.WorktreeList()
		if err != nil {
			return fmt.Errorf("listing worktrees: %w", err)
		}
		if len(worktrees) == 0 {
			return fmt.Errorf("no worktrees found")
		}

		// The primary worktree is always the first entry.
		primaryDir = worktrees[0].Path

		if filepath.Clean(top) == filepath.Clean(primaryDir) {
			return fmt.Errorf("cannot remove the primary worktree — run from a linked worktree")
		}

		for i := range worktrees {
			if filepath.Clean(worktrees[i].Path) == filepath.Clean(top) {
				wt = &worktrees[i]
				break
			}
		}
		if wt == nil {
			return fmt.Errorf("current directory %s is not inside a linked worktree", top)
		}

		branch = wt.Branch
		wtPath = wt.Path

		cfg, err = config.Load(primaryDir)
		if err != nil {
			return fmt.Errorf("loading config: %w", err)
		}
	} else {
		branch = args[0]

		var err error
		primaryDir, err = os.Getwd()
		if err != nil {
			return fmt.Errorf("getting current directory: %w", err)
		}

		cfg, err = config.Load(primaryDir)
		if err != nil {
			return fmt.Errorf("loading config: %w", err)
		}

		wtPath, err = worktree.ResolvePath(cfg.General.PathTemplate, primaryDir, branch)
		if err != nil {
			return fmt.Errorf("resolving worktree path: %w", err)
		}

		g := git.New(primaryDir)
		worktrees, err := g.WorktreeList()
		if err != nil {
			return fmt.Errorf("listing worktrees: %w", err)
		}

		for i := range worktrees {
			if filepath.Clean(worktrees[i].Path) == filepath.Clean(wtPath) {
				wt = &worktrees[i]
				break
			}
		}
		if wt == nil {
			return fmt.Errorf("no worktree found for branch %s at %s", branch, wtPath)
		}
	}

	return removeWorktree(branch, wtPath, primaryDir, wt, cfg)
}

// removeWorktree performs the shared teardown: dirty check, unlock, hooks,
// removal, optional branch deletion, and post_remove hooks.
func removeWorktree(branch, wtPath, primaryDir string, wt *git.Worktree, cfg *config.Config) error {
	g := git.New(primaryDir)

	// Detect trunk
	trunk := cfg.General.Trunk
	if trunk == "" {
		var err error
		trunk, err = g.DetectTrunk()
		if err != nil {
			return fmt.Errorf("detecting trunk: %w", err)
		}
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
			Base:       "",
			Repo:       commonDirOr(primaryDir, primaryDir),
			Profile:    "human",
			AgentOwned: agentOwned,
			BranchNew:  "0",
			Commands:   cfg.Hooks.PreRemove,
		}); err != nil {
			return fmt.Errorf("pre_remove hook failed: %w", err)
		}
	}

	// Remove worktree. We always pass --force to git because provisioning
	// and hooks may have created files that git sees as untracked. Our own
	// dirty check above is the real safety gate unless --force was passed.
	if err := g.WorktreeRemove(wtPath, true); err != nil {
		// git may have deregistered the worktree before hitting a permission
		// error on a root-owned file. Try to recover: detect foreign-owned
		// files and offer a sudo removal if we're in an interactive shell.
		if recovered := handleForeignOwnedRemoval(primaryDir, wtPath, err); recovered != nil {
			return recovered
		}
		return fmt.Errorf("removing worktree: %w", err)
	}

	// Delete branch if requested (guarded: detached-HEAD worktrees have no branch)
	if removeDeleteBranch && branch != "" {
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
			Base:       "",
			Repo:       commonDirOr(primaryDir, primaryDir),
			Profile:    "human",
			AgentOwned: agentOwned,
			BranchNew:  "0",
			Commands:   cfg.Hooks.PostRemove,
		}); err != nil {
			return fmt.Errorf("post_remove hook failed: %w", err)
		}
	}

	if branch != "" {
		fmt.Printf("Removed worktree for %s\n", branch)
	} else {
		fmt.Printf("Removed worktree at %s\n", wtPath)
	}

	// The shell's CWD may now point at a removed directory.
	if removeCurrent {
		fmt.Fprintf(os.Stderr, "note: your shell is still in the removed directory — run: cd %s\n", primaryDir)
	}
	return nil
}

// handleForeignOwnedRemoval is called when git worktree remove fails. It
// checks whether the failure is caused by files owned by another user (e.g.
// root-owned docker volumes left by a dev-workspace provisioning step). If
// so, and stdin is a TTY, it offers to retry the removal with sudo. Returns
// nil if the worktree was successfully removed (recovered), or the original
// error if recovery was not possible or declined.
func handleForeignOwnedRemoval(primaryDir, wtPath string, removeErr error) error {
	if !hasForeignOwnedFiles(wtPath) {
		return removeErr
	}

	if !isInteractive() {
		fmt.Fprintf(os.Stderr, "worktree removal failed: %v\n", removeErr)
		fmt.Fprintf(os.Stderr, "the worktree contains files owned by another user.\n")
		fmt.Fprintf(os.Stderr, "remove them manually: sudo rm -rf %s\n", wtPath)
		return removeErr
	}

	fmt.Fprintf(os.Stderr, "\nworktree removal failed: %v\n", removeErr)
	fmt.Fprintf(os.Stderr, "the worktree contains files owned by another user (e.g. root-owned\n")
	fmt.Fprintf(os.Stderr, "docker volumes from provisioning). git has already deregistered the\n")
	fmt.Fprintf(os.Stderr, "worktree, but the directory remains on disk.\n\n")
	fmt.Fprintf(os.Stderr, "Remove %s with sudo? [y/N] ", wtPath)

	var answer string
	fmt.Scanln(&answer)
	if answer != "y" && answer != "Y" && answer != "yes" {
		fmt.Fprintf(os.Stderr, "declined — leaving directory in place.\n")
		fmt.Fprintf(os.Stderr, "remove manually: sudo rm -rf %s\n", wtPath)
		return removeErr
	}

	// Prune the stale git admin entry first (git deregistered the worktree
	// but may have left metadata pointing at the now-unremovable path).
	if err := git.New(primaryDir).WorktreePrune(); err != nil {
		fmt.Fprintf(os.Stderr, "warning: git worktree prune failed (continuing): %v\n", err)
	}

	rm := exec.Command("sudo", "rm", "-rf", wtPath)
	rm.Stdin = os.Stdin
	rm.Stdout = os.Stdout
	rm.Stderr = os.Stderr
	if err := rm.Run(); err != nil {
		return fmt.Errorf("sudo removal of %s: %w", wtPath, err)
	}

	fmt.Fprintf(os.Stderr, "removed %s with sudo\n", wtPath)
	return nil
}

// hasForeignOwnedFiles reports whether the directory tree at path contains
// any file or directory not owned by the current user. It returns false if
// the path doesn't exist or cannot be walked.
func hasForeignOwnedFiles(path string) bool {
	if _, err := os.Stat(path); err != nil {
		return false
	}
	uid := os.Getuid()
	if uid == 0 {
		// root can delete anything; "foreign" is meaningless.
		return false
	}
	found := false
	filepath.Walk(path, func(p string, info os.FileInfo, err error) error {
		if err != nil {
			// Permission error reading an entry is itself evidence of
			// foreign ownership.
			found = true
			return filepath.SkipDir
		}
		if stat, ok := info.Sys().(*syscall.Stat_t); ok && int(stat.Uid) != uid {
			found = true
			return filepath.SkipAll
		}
		return nil
	})
	return found
}

// isInteractive reports whether stdin is a terminal, i.e. it's safe to
// prompt the user for confirmation.
func isInteractive() bool {
	fi, err := os.Stdin.Stat()
	if err != nil {
		return false
	}
	return (fi.Mode() & os.ModeCharDevice) != 0
}
