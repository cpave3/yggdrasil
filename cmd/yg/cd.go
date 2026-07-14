package main

import (
	"fmt"
	"os"
	"strconv"

	"github.com/cameron/yggdrasil/pkg/git"
	"github.com/spf13/cobra"
)

func cdCmd() *cobra.Command {
	return &cobra.Command{
		Use:                "cd <index>",
		Short:              "Print the path of an indexed worktree",
		Args:               cobra.ExactArgs(1),
		DisableFlagParsing: true,
		RunE:               runCD,
	}
}

func runCD(cmd *cobra.Command, args []string) error {
	index, err := strconv.Atoi(args[0])
	if err != nil || index < 0 {
		return fmt.Errorf("index must be a non-negative integer: %q", args[0])
	}

	cwd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("getting current directory: %w", err)
	}

	worktrees, err := git.New(cwd).WorktreeList()
	if err != nil {
		return fmt.Errorf("listing worktrees: %w", err)
	}
	if index >= len(worktrees) {
		return fmt.Errorf("worktree index %d out of range (available: 0-%d)", index, len(worktrees)-1)
	}

	fmt.Fprintln(cmd.OutOrStdout(), worktrees[index].Path)
	return nil
}
