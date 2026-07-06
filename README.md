# Yggdrasil (`yg`)

A single-binary CLI for managing Git worktrees, purpose-built for parallel AI
coding agents and humans. It turns a freshly created worktree from an unusable
shell — missing dependencies, missing gitignored config — into a ready-to-work
environment by running declarative lifecycle hooks and provisioning
non-reproducible files.

## v0.1 Scope (MVP)

This is the v0.1 MVP. It implements:

- **`yg init`** — generate a `.yggdrasil.toml` with auto-detected env files, certs, ecosystem, and trunk branch
- **`yg new <branch> [base]`** — create + provision + hook a worktree
- **`yg list`** / **`yg ls`** — status dashboard (branch, path, dirty/clean, ahead/behind, locked)
- **`yg remove <branch>`** — tear down with safety checks
- **`yg setup <branch>`** — re-provision an existing tree
- **`yg config`** — print effective merged config
- Declarative `copy`/`link` provisioning with path safety (FR-12)
- `post_create` hook with full env contract (FR-17) and CWD contract (FR-16)
- `--agent-owned` worktree locking (FR-6a)
- `--dry-run` mode for all operations
- Layered config: defaults → `.yggdrasil.toml` → `.yggdrasil.local.toml`

Not yet implemented (per PRD milestones v0.2+): trust boundary, JSON output,
event emission, profiles, secret scoping, merge/sync, prune, shell integration.

## Build & Test

```bash
task build      # Build the binary to ./yg
task test       # Run all tests
task lint       # Run golangci-lint
task check      # Build + test + lint
task cover      # Coverage report
```

## Quickstart

```bash
# Generate a .yggdrasil.toml tailored to your project:
yg init

# Create a worktree — provisions files, installs deps, runs hooks:
yg new feature-x

# List all worktrees:
yg list

# Remove when done:
yg remove --delete-branch feature-x
```

## Usage

### Initialize config

```bash
# Auto-detects env files, certs, ecosystem (Node/Python/Go/Rust/.NET),
# trunk branch, and package manager — generates a commented .yggdrasil.toml.
yg init

# Overwrite an existing config:
yg init --force
```

### Create a worktree

```bash
# Create a worktree for 'feature-x' based on trunk
yg new feature-x

# Create with a specific base
yg new feature-x main

# Print only the path (for scripting)
yg new --print-path feature-x

# Agent-owned (locks the worktree against external pruning)
yg new --agent-owned feature-x

# Dry run (show what would happen)
yg new --dry-run feature-x
```

### List worktrees

```bash
yg list
# or
yg ls
```

Output:
```
BRANCH       PATH                                  STATUS  AHEAD  BEHIND  LOCKED
main         /repos/myproject                      clean   0      0
feature-x    /repos/myproject.feature-x            clean   3      0
agent-feat   /repos/myproject.agent-feat          dirty    2      1       locked
```

### Remove a worktree

```bash
# Remove (fails if dirty)
yg remove feature-x

# Force remove despite dirty tree
yg remove --force feature-x

# Remove and delete the branch ref
yg remove --delete-branch feature-x
```

### Re-provision an existing tree

```bash
# Re-run provisioning + post_create on an existing worktree
yg setup feature-x
```

### View effective config

```bash
yg config
```

## Configuration

### Project config (`.yggdrasil.toml`, committed)

```toml
[general]
trunk = "main"                          # auto-detected if omitted
path_template = "../{repo}.{branch}"    # {repo} and {branch} are expanded

[provision]
copy = [".env", ".env.*"]               # copied from primary to new tree
link = ["certs/dev.pem"]                 # symlinked from primary into new tree

[hooks]
post_create = ["pnpm install"]
pre_remove = ["echo cleaning up"]
```

### Local override (`.yggdrasil.local.toml`, gitignored)

Local override is always loaded on top of project config. Provisioning lists
**append** across layers; hook arrays **replace if set** in a higher layer.

```toml
# .yggdrasil.local.toml
[provision]
copy = [".env.local"]                   # appends to project's [".env", ".env.*"]

[hooks]
post_create = ["pnpm install --prefer-offline"]  # replaces project's
```

### Path template

The template supports `{repo}` (repo directory basename) and `{branch}` (branch
name, with slashes replaced by dashes). The default is `../{repo}.{branch}`,
which places worktrees as siblings of the primary checkout.

### Hook environment contract

Every hook receives these environment variables:

| Variable         | Description                                    |
| ---------------- | ---------------------------------------------- |
| `YG_WORKTREE`    | New worktree path                              |
| `YG_PRIMARY`     | Primary worktree path                           |
| `YG_BRANCH`      | Branch name                                     |
| `YG_TRUNK`       | Trunk branch name                               |
| `YG_REPO`        | Git common directory                            |
| `YG_PROFILE`     | Profile name (always "human" in v0.1)          |
| `YG_EVENT`       | Lifecycle event name                            |
| `YG_AGENT_OWNED` | "1" or "0"                                      |

### CWD contract

| Event         | CWD              |
| ------------- | ---------------- |
| `pre_create`  | Primary worktree |
| `post_create` | New worktree     |
| `pre_remove`  | Primary worktree |
| `post_remove` | Primary worktree |

### Path safety

Provisioning patterns are validated for path safety:

- Absolute paths (leading `/`) are rejected
- `..` traversal is rejected
- Copy operations do not follow symlinks
- Link targets must resolve within the primary worktree

## Architecture

```
cmd/yg/          Entry point + CLI commands (package main)
internal/testutil/   Test helpers (GitRepo, isolated repos)
pkg/config/      Layered config loading and merge
pkg/git/         Git binary wrapper (worktree ops, branch ops)
pkg/worktree/    Path template resolution
pkg/provision/   File provisioning (copy, link, path safety)
pkg/hooks/       Hook runner (sh -c, env contract, failure semantics)
```

## Testing

All tests are integration-style: they exercise real git in temp dirs and real
file I/O. No mocks of internal collaborators. Tests verify behavior through
public interfaces.

```bash
task test         # Run all tests
task test:verbose # Verbose output
task test:race    # With race detector
task cover        # Coverage report
```
