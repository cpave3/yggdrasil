// Package main is the entry point for the yg CLI.
package main

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

var (
	verbose bool
	dryRun  bool
)

var rootCmd = &cobra.Command{
	Use:   "yg",
	Short: "Yggdrasil — manage Git worktrees with provisioning and hooks",
	Long: `Yggdrasil (yg) manages Git worktrees for parallel AI agents and humans.
It creates, provisions, and tears down worktrees with declarative hooks and path-safe file linking/copying.`,
	SilenceUsage: true,
}

func init() {
	rootCmd.PersistentFlags().BoolVarP(&verbose, "verbose", "v", false, "print resolved hook commands, provisioning operations, and results")
	rootCmd.PersistentFlags().BoolVar(&dryRun, "dry-run", false, "show what would be provisioned/run without executing hooks or writing files")
}

func init() {
	rootCmd.AddCommand(newCmd())
	rootCmd.AddCommand(listCmd())
	rootCmd.AddCommand(removeCmd())
	rootCmd.AddCommand(setupCmd())
	rootCmd.AddCommand(configCmd())
	rootCmd.AddCommand(initCmd())
}

func main() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
