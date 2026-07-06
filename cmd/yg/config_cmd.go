package main

import (
	"fmt"
	"os"
	"strings"

	"github.com/cameron/yggdrasil/pkg/config"
	"github.com/spf13/cobra"
)

func configCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "config",
		Short: "Print the effective merged configuration",
		Long:  `Prints the effective merged configuration from all layers (defaults, project, local override).`,
		RunE:  runConfig,
	}
	return cmd
}

func runConfig(cmd *cobra.Command, args []string) error {
	primaryDir, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("getting current directory: %w", err)
	}

	cfg, err := config.Load(primaryDir)
	if err != nil {
		return fmt.Errorf("loading config: %w", err)
	}

	fmt.Println("[general]")
	fmt.Printf("  trunk          = %s\n", cfg.General.Trunk)
	fmt.Printf("  path_template  = %s\n", cfg.General.PathTemplate)
	fmt.Printf("  merge_strategy = %s\n", cfg.General.MergeStrategy)
	fmt.Printf("  sync_strategy  = %s\n", cfg.General.SyncStrategy)
	fmt.Println()

	fmt.Println("[provision]")
	fmt.Printf("  copy = %s\n", formatList(cfg.Provision.Copy))
	fmt.Printf("  link = %s\n", formatList(cfg.Provision.Link))
	fmt.Println()

	fmt.Println("[hooks]")
	fmt.Printf("  pre_create  = %s\n", formatList(cfg.Hooks.PreCreate))
	fmt.Printf("  post_create = %s\n", formatList(cfg.Hooks.PostCreate))
	fmt.Printf("  pre_remove  = %s\n", formatList(cfg.Hooks.PreRemove))
	fmt.Printf("  post_remove = %s\n", formatList(cfg.Hooks.PostRemove))
	fmt.Printf("  pre_merge   = %s\n", formatList(cfg.Hooks.PreMerge))
	fmt.Printf("  post_merge  = %s\n", formatList(cfg.Hooks.PostMerge))
	fmt.Printf("  pre_sync    = %s\n", formatList(cfg.Hooks.PreSync))
	fmt.Printf("  post_sync   = %s\n", formatList(cfg.Hooks.PostSync))

	return nil
}

func formatList(items []string) string {
	if len(items) == 0 {
		return "[]"
	}
	return "[" + strings.Join(items, ", ") + "]"
}
