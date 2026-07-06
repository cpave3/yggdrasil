# Yggdrasil Usage Guide

A practical walkthrough for using `yg` in a codebase where `git worktree add`
leaves you with a broken tree — no `.env`, no installed deps, no local config.

## 0. Generate your config automatically

Before writing `.yggdrasil.toml` by hand, try `yg init` — it scans your repo
and generates a config with sensible defaults:

```bash
yg init
```

It auto-detects:

| What | How |
|------|-----|
| **Trunk branch** | `git rev-parse --abbrev-ref origin/HEAD`, falls back to current branch |
| **Env files** | Scans for `.env`, `.env.local`, `.env.development`, etc. |
| **Cert files** | Scans `certs/`, `cert/`, `ssl/`, `tls/` for `*.pem`, `*.crt`, `*.key` |
| **Ecosystem** | `package.json` → Node, `pyproject.toml` → Python, `go.mod` → Go, `Cargo.toml` → Rust, `*.csproj`/`*.sln` → .NET |
| **Package manager** | `pnpm-lock.yaml` → pnpm, `yarn.lock` → yarn, `bun.lockb` → bun, else npm |
| **Appsettings** | `appsettings.Development.json`, `appsettings.Local.json` |

The generated file has all sections, inline comments explaining each field,
and commented-out alternatives. Review it, adjust, and commit.

```bash
# Overwrite an existing config:
yg init --force
```

## 1. Write your `.yggdrasil.toml` by hand (if you prefer)

Put this at the repo root and commit it:

```toml
[general]
trunk = "main"                          # omit to auto-detect
path_template = "../{repo}.{branch}"    # worktrees go in sibling dirs

[provision]
# Files that should be COPIED (independent per tree — changes don't propagate)
copy = [
  ".env",                               # local env file
  ".env.*",                             # env variants (.env.local, .env.development)
  "appsettings.Development.json",       # .NET local config
]

# Files that should be LINKED (symlink — always tracks the primary's version)
# Use this for things that should stay in sync across all trees
link = [
  "certs/dev.pem",                      # shared dev cert
  ".vscode/settings.json",              # editor config (if you want it shared)
]

[hooks]
# Runs in the new worktree root after provisioning
post_create = [
  "pnpm install",                       # install deps (or: npm ci, pip install, go mod download, etc.)
]

# Runs in the primary before removal (cleanup, if needed)
pre_remove = []
```

## 2. The workflow

### Starting work on a feature

```bash
# From your primary checkout (where deps are installed, .env exists):
yg new feature-payment-integration

# Output:
# Created worktree for feature-payment-integration at /repos/myproject.feature-payment-integration

# cd into it:
cd $(yg new --print-path feature-payment-integration)
# or just:
cd ../myproject.feature-payment-integration
```

At this point the new worktree has:
- All committed files (shared git object store)
- `.env` **copied** from primary (you can edit it independently per tree)
- `.env.local` **copied** (matched by `.env.*`)
- `certs/dev.pem` **symlinked** to the primary's copy (edits propagate)
- `pnpm install` already ran (deps installed)

You're ready to work. No manual setup.

### Running multiple agents in parallel

```bash
# Each agent gets its own tree, own deps, own .env copy:
yg new agent-refactor-1 --agent-owned
yg new agent-refactor-2 --agent-owned
yg new agent-refactor-3 --agent-owned

# Check what's running:
yg list
# BRANCH           PATH                                        STATUS  AHEAD  BEHIND  LOCKED
# main             /repos/myproject                            clean   0      0
# agent-refactor-1 /repos/myproject.agent-refactor-1           clean   2      0       locked
# agent-refactor-2 /repos/myproject.agent-refactor-2           dirty   5      0       locked
# agent-refactor-3 /repos/myproject.agent-refactor-3           clean   3      1       locked
```

`--agent-owned` locks the worktree so a stray `git worktree prune` (from
another tool, a cron job, a CI cleanup) can't reap it while the agent is still
working.

### Cleaning up

```bash
# Remove a finished tree (fails if dirty — safety check):
yg remove feature-payment-integration

# Force remove a dirty tree:
yg remove --force feature-payment-integration

# Remove AND delete the branch ref:
yg remove --delete-branch feature-payment-integration

# Remove an agent-owned tree (auto-unlocks first):
yg remove agent-refactor-1
```

### Crash recovery

If an agent crashed and left the worktree in a half-provisioned state, or you
changed your `.yggdrasil.toml` and want to re-provision:

```bash
# Re-runs provisioning + post_create without recreating the worktree:
yg setup feature-payment-integration
```

This is idempotent — safe to run multiple times. It won't run `pre_create`
(that's a tree-creation guard, not a provisioning step).

## 3. Common patterns by ecosystem

### Node.js (pnpm)

```toml
[provision]
copy = [".env", ".env.*"]
link = []

[hooks]
post_create = ["pnpm install --frozen-lockfile"]
```

If you use a monorepo with workspace deps and they're slow to install, consider
`link` for `node_modules` instead — but note that symlinking `node_modules`
means all trees share the same install, so a `pnpm add` in one tree affects all
of them. Usually you want `copy` semantics (independent installs), which means
just running `pnpm install` in `post_create`.

### Python (poetry/venv)

```toml
[provision]
copy = [".env", ".env.*"]
link = []

[hooks]
post_create = [
  "python -m venv .venv",
  ".venv/bin/pip install -e .",
]
```

### Go

```toml
[provision]
copy = [".env"]
link = []

[hooks]
post_create = ["go mod download"]
```

Go is the easiest — deps are in the shared object store, so you mostly just
need the `.env`.

### .NET

```toml
[provision]
copy = [
  "appsettings.Development.json",
  ".env",
  "secrets/local-user-secrets.json",
]
link = ["certs/dev.pfx"]

[hooks]
post_create = ["dotnet restore", "dotnet build"]
```

## 4. Local override (without touching committed config)

If you have personal paths or a different package manager, use
`.yggdrasil.local.toml` (gitignored — it's in the default `.gitignore`):

```toml
# .yggdrasil.local.toml — not committed
[provision]
# Appends to project config's copy list
copy = [".env.cameron"]

[hooks]
# Replaces project config's post_create (replace-if-set semantics)
post_create = ["pnpm install --prefer-offline"]
```

Run `yg config` to see the effective merged config and verify your layers:

```bash
yg config
# [general]
#   trunk          = main
#   path_template  = ../{repo}.{branch}
#
# [provision]
#   copy = [.env, .env.*, .env.cameron]
#   link = []
#
# [hooks]
#   post_create = [pnpm install --prefer-offline]
```

## 5. Key things to know

| Concern | Answer |
|---------|--------|
| **Are copied files shared between trees?** | No. `copy` makes an independent file. Editing `.env` in one tree doesn't affect others. |
| **Are linked files shared?** | Yes. `link` creates a symlink to the primary's file. Edits propagate to all trees. |
| **What if the source file doesn't exist?** | It's silently skipped. No error. |
| **Can a committed config exfiltrate files?** | No. Path safety rejects absolute paths, `..` traversal, and symlinks in copy sources. Link targets must resolve within the primary. |
| **Is provisioning idempotent?** | Yes. Re-running `yg setup` won't error on existing files. Links are replaced. |
| **Does `--dry-run` show what will happen?** | Yes. `yg new --dry-run feature-x` prints planned actions without creating anything. |

The core value: you write `.yggdrasil.toml` once, commit it, and then every
`yg new <branch>` produces a tree that's ready to work in — deps installed,
config copied, hooks run. No more manual setup per tree.
