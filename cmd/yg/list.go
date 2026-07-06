package main

import (
	"fmt"
	"os"
	"text/tabwriter"

	"github.com/cameron/yggdrasil/pkg/config"
	"github.com/cameron/yggdrasil/pkg/git"
	"github.com/spf13/cobra"
)

func listCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "list",
		Aliases: []string{"ls"},
		Short:   "List all worktrees with status",
		Long:    `Lists all worktrees with branch, path, dirty/clean status, locked state, and ahead/behind trunk.`,
		RunE:    runList,
	}
	return cmd
}

func runList(cmd *cobra.Command, args []string) error {
	primaryDir, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("getting current directory: %w", err)
	}

	cfg, err := config.Load(primaryDir)
	if err != nil {
		return fmt.Errorf("loading config: %w", err)
	}

	trunk := cfg.General.Trunk
	g := git.New(primaryDir)
	if trunk == "" {
		trunk, err = g.DetectTrunk()
		if err != nil {
			return fmt.Errorf("detecting trunk: %w", err)
		}
	}

	worktrees, err := g.WorktreeList()
	if err != nil {
		return fmt.Errorf("listing worktrees: %w", err)
	}

	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, "BRANCH\tPATH\tSTATUS\tAHEAD\tBEHIND\tLOCKED")

	for _, wt := range worktrees {
		branch := wt.Branch
		if branch == "" {
			branch = "(detached)"
		}

		// Dirty/clean status
		status := "clean"
		if dirty, err := g.IsDirty(wt.Path); err == nil && dirty {
			status = "dirty"
		}

		// Ahead/behind
		var ahead, behind int
		if branch != "(detached)" && branch != trunk {
			a, b, err := g.AheadBehind(branch, trunk)
			if err == nil {
				ahead, behind = a, b
			}
		}

		// Locked state
		locked := ""
		if wt.Locked {
			locked = "locked"
		}

		fmt.Fprintf(w, "%s\t%s\t%s\t%d\t%d\t%s\n", branch, wt.Path, status, ahead, behind, locked)
	}

	return w.Flush()
}
