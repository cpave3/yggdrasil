// Package config handles loading and merging Yggdrasil configuration layers.
package config

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/pelletier/go-toml/v2"
)

// DefaultPathTemplate is the built-in default for worktree path resolution.
const DefaultPathTemplate = "../{repo}.{branch}"

// Config holds the effective merged configuration for a repository.
type Config struct {
	General  General   `toml:"general"`
	Provision Provision `toml:"provision"`
	Hooks    Hooks     `toml:"hooks"`
}

type General struct {
	Trunk         string `toml:"trunk"`
	PathTemplate  string `toml:"path_template"`
	MergeStrategy string `toml:"merge_strategy"`
	SyncStrategy  string `toml:"sync_strategy"`
}

type Provision struct {
	Copy []string `toml:"copy"`
	Link []string `toml:"link"`
}

type Hooks struct {
	PreCreate  []string `toml:"pre_create"`
	PostCreate []string `toml:"post_create"`
	PreRemove  []string `toml:"pre_remove"`
	PostRemove []string `toml:"post_remove"`
	PreMerge   []string `toml:"pre_merge"`
	PostMerge  []string `toml:"post_merge"`
	PreSync    []string `toml:"pre_sync"`
	PostSync   []string `toml:"post_sync"`
}

// DefaultConfig returns the built-in default configuration.
func DefaultConfig() Config {
	return Config{
		General: General{
			PathTemplate: DefaultPathTemplate,
		},
	}
}

// Load reads configuration from the given repo root. It loads, in order of
// increasing precedence: built-in defaults, project config (.yggdrasil.toml),
// and local override (.yggdrasil.local.toml). Provisioning lists append across
// layers; hook arrays replace if set in a higher layer (FR-8).
func Load(repoRoot string) (*Config, error) {
	cfg := DefaultConfig()

	projectFile := filepath.Join(repoRoot, ".yggdrasil.toml")
	if data, err := os.ReadFile(projectFile); err == nil {
		var project Config
		if err := toml.Unmarshal(data, &project); err != nil {
			return nil, fmt.Errorf("parsing %s: %w", projectFile, err)
		}
		mergeConfig(&cfg, &project)
	} else if !os.IsNotExist(err) {
		return nil, fmt.Errorf("reading %s: %w", projectFile, err)
	}

	localFile := filepath.Join(repoRoot, ".yggdrasil.local.toml")
	if data, err := os.ReadFile(localFile); err == nil {
		var local Config
		if err := toml.Unmarshal(data, &local); err != nil {
			return nil, fmt.Errorf("parsing %s: %w", localFile, err)
		}
		mergeConfig(&cfg, &local)
	} else if !os.IsNotExist(err) {
		return nil, fmt.Errorf("reading %s: %w", localFile, err)
	}

	return &cfg, nil
}

// mergeConfig merges src into dst in place. Provisioning lists append; hook
// arrays replace if non-empty in src. General fields replace if non-empty.
func mergeConfig(dst, src *Config) {
	if src.General.Trunk != "" {
		dst.General.Trunk = src.General.Trunk
	}
	if src.General.PathTemplate != "" {
		dst.General.PathTemplate = src.General.PathTemplate
	}
	if src.General.MergeStrategy != "" {
		dst.General.MergeStrategy = src.General.MergeStrategy
	}
	if src.General.SyncStrategy != "" {
		dst.General.SyncStrategy = src.General.SyncStrategy
	}

	dst.Provision.Copy = append(dst.Provision.Copy, src.Provision.Copy...)
	dst.Provision.Link = append(dst.Provision.Link, src.Provision.Link...)

	mergeHooks(&dst.Hooks, &src.Hooks)
}

func mergeHooks(dst, src *Hooks) {
	if len(src.PreCreate) > 0 {
		dst.PreCreate = src.PreCreate
	}
	if len(src.PostCreate) > 0 {
		dst.PostCreate = src.PostCreate
	}
	if len(src.PreRemove) > 0 {
		dst.PreRemove = src.PreRemove
	}
	if len(src.PostRemove) > 0 {
		dst.PostRemove = src.PostRemove
	}
	if len(src.PreMerge) > 0 {
		dst.PreMerge = src.PreMerge
	}
	if len(src.PostMerge) > 0 {
		dst.PostMerge = src.PostMerge
	}
	if len(src.PreSync) > 0 {
		dst.PreSync = src.PreSync
	}
	if len(src.PostSync) > 0 {
		dst.PostSync = src.PostSync
	}
}
