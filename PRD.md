# Yggdrasil (`yg`) — Product Requirements Document

**Status:** Draft v0.1
**Owner:** Cameron
**Last updated:** 2026-06-23

---

## 1. Summary

Yggdrasil (`yg`) is a single-binary CLI for managing Git worktrees, purpose-built for a workflow where multiple AI coding agents and humans run in parallel against the same repository. It turns a freshly created worktree from an unusable shell — missing dependencies, missing gitignored config — into a ready-to-work environment by running declarative, **trust-gated** lifecycle hooks and provisioning non-reproducible files.

It is deliberately not just another worktree wrapper. The two differentiators are (a) a security model that treats committed hook config as untrusted code until explicitly approved, and (b) a machine-first integration surface so it can be driven headlessly by an orchestrator (Chimera) and slot in as a Claude Code worktree hook.

The name is the Norse world-tree whose branches hold the worlds together — a fitting metaphor for the tool that holds and provisions every branch's working tree.

---

## 2. Background & motivation

Git worktrees give each agent or task an isolated working directory that shares one object store, which is what makes parallel autonomous agents practical without cloning the repo N times. But native `git worktree add` produces a directory that does not contain gitignored artifacts (`node_modules`, `vendor/`, `bin`/`obj`, `.venv`) or non-reproducible local config (`.env`, dev appsettings, local certs, seeded SQLite). The tree boots into a broken state and the human or agent has to hand-run setup every time.

Several tools already solve the mechanical part — worktrunk, nekocode/agent-worktree, CodeRabbit's git-worktree-runner — and Claude Code itself now ships `WorktreeCreate`/`WorktreeRemove` hooks. The space is close to commoditized. We are building our own anyway for two reasons that the off-the-shelf tools do not serve:

1. **Ownership and integration.** `yg` must emit lifecycle events onto the Chimera message bus and expose a stable machine interface so the orchestrator can provision, track, and reap worktrees without screen-scraping CLI output.
2. **Safety gaps no existing tool fully closes.** Two specifically: arbitrary hook execution from shared config (a code-execution vector that matters _more_ when autonomous agents pull untrusted branches), and handing live secrets to an agent's worktree.

---

## 3. Goals & non-goals

### Goals

- Make a fresh worktree usable in one command, reproducibly, via declarative provisioning + hooks.
- Treat committed hook config as untrusted until explicitly approved, with trust invalidated on change.
- Support secret-scoping per profile so agent trees never receive full production credentials by default.
- Expose a machine-first interface (JSON output, stable exit codes, event emission) for orchestrator control.
- Be a single static binary, fast to invoke, cross-platform (Linux first; macOS; Windows via Git Bash best-effort).
- Be a drop-in `WorktreeCreate`/`WorktreeRemove` hook provider for Claude Code.

### Non-goals

- `yg` is **not** a merge-conflict resolver or a multi-agent task scheduler. It manages worktree lifecycle and provisioning only; orchestration logic lives in Chimera.
- Not a TUI/desktop app (v1). The dashboard is a `list` command, not a long-running UI.
- Not a sandbox. `yg` does not jail what hooks or agents can do; isolation is at the filesystem/worktree level only. Sandboxing is explicitly deferred (see §10).
- Not a Git reimplementation. We shell out to the user's `git` binary as the source of truth rather than reimplementing worktree semantics.

---

## 4. Users & primary use cases

- **Solo dev running N parallel agents.** Spins up several worktrees, one agent per tree, each provisioned and ready, with full secrets withheld from agent trees.
- **Human quality-of-life.** Replaces the three-times-typed `git worktree add` dance with `yg new <branch>`, auto-provisioned and `cd`-ready.
- **Chimera orchestrator.** Programmatically creates/lists/reaps worktrees, consumes lifecycle events from the bus, drives provisioning headlessly with pre-authorized trust.
- **Claude Code.** Invokes `yg` as its worktree create/remove hook to get our provisioning + naming conventions while keeping Claude Code's native session handling.

---

## 5. Key concepts

| Term                 | Meaning                                                                                                   |
| -------------------- | --------------------------------------------------------------------------------------------------------- |
| **Primary worktree** | The main checkout that holds canonical local config (the source for linked/copied files).                 |
| **Linked worktree**  | A `yg`-managed worktree for a branch, addressed by branch name; path derived from a template.             |
| **Project config**   | Committed `.yggdrasil.toml` at repo root — shared, version-controlled, **untrusted by default**.          |
| **Local config**     | Per-user override (gitignored / outside repo) — **always trusted** because it is the user's own.          |
| **Hook**             | A shell command run at a lifecycle event, in a defined CWD, with a defined env contract.                  |
| **Profile**          | A named bundle of hooks + provisioning + secret policy (e.g. `human`, `agent`).                           |
| **Trust record**     | A local, per-user record that a specific repo's hook set has been approved, keyed by a hash of the hooks. |

---

## 6. Functional requirements

### 6.1 Worktree lifecycle

- **FR-1** `yg new <branch> [base]` creates a branch (if needed) and a worktree, runs `pre_create` → provisions files → runs `post_create`, then prints the path (and optionally `cd`s via shell integration).
- **FR-2** Worktree paths are computed from a configurable template (e.g. `../{repo}.{branch}` or a sibling `worktrees/` dir), never requiring the user to type the path.
- **FR-3** `yg list` (`yg ls`) shows all worktrees with status: branch, ahead/behind trunk, dirty/clean, unpushed, profile, agent-owned flag, age. Supports `--json`.
- **FR-4** `yg switch <branch>` / `yg cd <branch>` moves between trees (via shell integration); `yg cd` with no arg returns to primary.
- **FR-5** `yg remove <branch>` runs `pre_remove` (with safety checks) → removes the worktree → runs `post_remove`. `--force` overrides safety checks; deletes the branch only on explicit `--delete-branch` or if merged.
- **FR-6** `yg prune` reconciles stale state: detects worktrees whose directory was deleted by a crashed agent but whose registry entry is **locked** (native `git worktree prune` skips locked entries), offers to unlock + prune, and reports orphaned branches. `--yes`/`--force` for non-interactive use.
- **FR-6a** `yg` adds a native `git worktree lock` on creation when `--agent-owned` is set (or `YG_AGENT_OWNED=1`), preventing external `git worktree prune` from reaping agent trees while an agent is still working. `yg remove` unlocks before removing. This is the only mechanism that locks a worktree; locked state is surfaced in `yg ls` output.

### 6.2 Configuration

- **FR-7** Project config lives at `.yggdrasil.toml` (repo root, committed). Local override lives outside version control (e.g. `~/.config/yggdrasil/<repo-id>.toml` and/or `.yggdrasil.local.toml` which `yg` will offer to gitignore).
- **FR-8** Precedence: local override > project config > built-in defaults. Merge semantics are explicit per field — provisioning lists append; hook arrays replace if set in a higher layer (matching agent-worktree's behavior, which is the least surprising).
- **FR-9** `yg config` prints the effective merged config (with `--json`) and the provenance of each value (which layer it came from).
- **FR-10** Trunk branch is auto-detected (origin/HEAD) with config override.

### 6.3 Provisioning (the "usable fresh tree" core)

- **FR-11** Declarative `link` and `copy` lists, gitignore-style patterns, resolved from the **primary worktree** into the new tree. Parent directories are created as needed. Operations are idempotent (safe to re-run).
- **FR-12** **Path safety (hard requirement):** patterns rejecting leading `/` (absolute) and `..` traversal; copy operations do not follow symlinks. `link` targets must resolve within the primary worktree (the symlink is created inside the new tree pointing at a path inside the primary — never an absolute or out-of-tree target). Directory matches in `copy` are recursive but subject to the same path-safety and no-symlink-follow rules per file. A shared committed config must not be able to exfiltrate or write outside the worktree. Violations fail closed with a clear error.
- **FR-13** `yg setup <branch>` re-runs provisioning + `post_create` on an existing tree without recreating it (for crash recovery and config changes). It does **not** re-run `pre_create` (which is a tree-creation guard, not a provisioning step). If `post_create` hooks are already-trusted, `setup` skips the trust prompt and runs immediately; if the hook set changed since creation, trust must be re-approved first (consistent with FR-22).
- **FR-14** Choice of `link` (symlink, for files that should track the primary, e.g. shared certs) vs `copy` (independent, e.g. a tree-specific `.env`) is per-entry.

### 6.4 Hooks

- **FR-15** Lifecycle events: `pre_create`, `post_create`, `pre_remove`, `post_remove`, `pre_merge`, `post_merge`, `pre_sync`, `post_sync`. Each is an ordered list of shell commands.
- **FR-16** **CWD contract:** `post_create`/`setup` run in the new worktree root; merge/sync hooks run in the worktree root; `pre_create` runs in the primary. Documented and stable.
- **FR-17** **Environment contract:** every hook receives `YG_WORKTREE`, `YG_PRIMARY`, `YG_BRANCH`, `YG_TRUNK`, `YG_BASE` (the base branch/commit the worktree was created from; empty for `setup`/`remove`), `YG_REPO` (common dir), `YG_PROFILE`, `YG_EVENT`, `YG_AGENT_OWNED`, `YG_BRANCH_NEW` ("1" if the branch was newly created by this operation, "0" if it pre-existed — lets hooks gate on first-creation only). Hooks reference paths via these, never hardcoded. Hooks inherit the caller's full environment (including `PATH`); `yg` does not scrub or restrict it. This inheritance is part of the trust implication — `yg trust` means "I accept this command runs with my environment and privileges" (see §10.5).
- **FR-18** **Failure semantics:** a failing hook fails the operation with a non-zero exit and a clear report, but the worktree is **left in place by default** (`--keep-on-failure` default true) so it can be inspected; `--rollback-on-failure` opts into teardown. Provisioning that flaked should never silently destroy a tree.
- **FR-19** Hooks run via `sh -c` (POSIX) / appropriate shell on Windows. No timeout in v1 (documented limitation); `--hook-timeout` is a fast-follow. **Risk note:** a hung hook in agent/headless mode blocks the orchestrator indefinitely with no self-healing path. Chimera should treat worktree creation as a timed operation at the orchestration layer (kill the `yg` process after N seconds), and `yg` should document that it does not kill child processes on signal in v1. `--hook-timeout` should be elevated to v0.2 if agent workflows prove fragile.

### 6.5 Hook trust boundary (headline security feature)

- **FR-20** Hooks and provisioning commands defined in **project (committed) config are untrusted until explicitly approved.** Until trusted, `yg new` refuses to run them and tells the user to review.
- **FR-21** `yg trust` displays the exact hook/command set for review and records approval. `yg trust --revoke` removes it. Trust is stored locally per user (e.g. `~/.local/state/yggdrasil/trust.json`), never committed.
- **FR-22** A trust record is keyed by `(repo identity, profile, hash of the resolved hook+provisioning command set)`. The "resolved command set" is the exact, normalized string of every hook command and every provisioning entry (patterns + mode), after variable expansion but before evaluation. **Any change to the hooks invalidates trust** and forces re-approval — this defeats a contributor sneaking a malicious command into a previously approved config. Note: if a hook invokes an external script (e.g. `./scripts/setup.sh`), the hash covers the command string only, not the script's contents — the script can change without invalidating trust. This is an accepted limitation; mitigations (hashing referenced files, or documenting that script-content trust is out of scope for v1) are tracked in §14.
- **FR-23** **Local config (the user's own) is always trusted.** Only shared/committed config requires approval.
- **FR-24** **Non-interactive / agent policy:** in headless mode (`--json`, no TTY, or orchestrator-set env) `yg` is **fail-closed** — untrusted hooks abort rather than prompt. The orchestrator pre-authorizes via an explicit, auditable mechanism: `yg trust --from <signed-file>` or `YG_TRUST_FROM`, so Chimera grants trust deliberately, not implicitly. The signed-file format and signing mechanism (e.g. ed25519 signature over the command-set hash, verified against a pinned public key) are specified in a separate trust-protocol doc; v0.1 may ship with a simpler `--from <plaintext-hash-file>` as a placeholder, but the interface must not change when the signing backend lands.
- **FR-25** `yg trust --show` lists all trusted repos and the hashes approved, for audit.

### 6.6 Profiles & secret scoping

- **FR-26** Named profiles bundle hooks + provisioning + secret policy. Built-in intent: `human` (full local config) and `agent` (scoped/scrubbed). Selected via `--profile`, config default, or auto-detected from invocation context (e.g. orchestrator sets `YG_PROFILE=agent`).
- **FR-27** A profile can declare a **secret policy** for provisioned env files: `full` (copy/link as-is), `template` (copy a sanitized template instead of the live file), or `inject` (run a hook/command that mints scoped, short-lived credentials for that tree). Default for `agent` is never `full`. **The `inject` command is itself a hook** — it is subject to the same trust boundary (FR-20–25) as any other hook, not an exempt privileged operation. The `secret_command` receives `YG_WORKTREE`, `YG_PROFILE`, and `YG_BRANCH` in its env and must write credentials to a file path given by `YG_SECRET_OUTPUT` on stdout (a path inside the worktree); it must not print secrets to stderr or the event log. `yg` scrubs `YG_SECRET_OUTPUT` contents from event payloads.
- **FR-28** Profiles compose with the base config (a profile overrides specified keys, inherits the rest), with provenance visible via `yg config --profile agent`.

### 6.7 Integration surface (machine-first)

- **FR-29** Every read command supports `--json` with a documented, versioned schema. Exit codes are stable and enumerated.
- **FR-30** `yg` emits structured lifecycle events (worktree created/removed/provisioned/hook-failed) to a configurable sink: stdout JSONL, a file, or a command/socket — enough to publish onto the Chimera message bus. Event payloads include branch, path, profile, agent-owned, timing, and outcome.
- **FR-31** **Claude Code hook mode:** `yg cc-worktree-create` reads Claude Code's JSON payload on stdin and prints the absolute worktree path on stdout (fully replacing default behavior), applying our naming + provisioning. A matching `cc-worktree-remove` mirrors it.
- **FR-32** `yg new --print-path` / `--quiet` for clean capture by callers; never interleave human chrome into machine output.

### 6.8 Merge & sync (parity with existing tools, optional path)

- **FR-33** `yg merge [target]` integrates a worktree's branch back (squash/merge/ff configurable), gated by `pre_merge` hooks (tests/lint). `--delete` removes the worktree after a successful merge.
- **FR-34** `yg sync` updates a worktree's branch from trunk (rebase/merge configurable), gated by `pre_sync`/`post_sync`.
- **FR-35** These are explicitly thin convenience wrappers; complex flows defer to the user's own git / Chimera.

---

## 7. Non-functional requirements

- **NFR-1** Single statically linked binary, no runtime deps beyond a `git` on PATH.
- **NFR-2** Cold invocation overhead < 30 ms (it is called per-agent-spawn; must feel free). Native compiled binary, lazy config load. Measured as wall-clock from process start to exit for `yg ls --json` on a repo with ≤ 20 worktrees on the Bazzite/dev box.
- **NFR-3** Cross-platform: Linux (primary — Bazzite/dev box), macOS (supported), Windows via Git Bash (best-effort, with path conversion at the boundary like gtr does).
- **NFR-4** Deterministic, scriptable output; human and machine modes never mixed.
- **NFR-5** Safe by default: fail-closed on trust and path-safety violations; destructive ops require explicit flags.
- **NFR-6** Trust store and local config are per-user and never written into the repo.
- **NFR-7** Concurrency-safe: multiple `yg` invocations (e.g. two agents creating worktrees simultaneously) must not corrupt the trust store or produce conflicting path assignments. The trust store uses file locking; worktree path assignment is deterministic from the template (no random temp dirs).
- **NFR-8** Observable: `--verbose` prints the resolved hook commands, provisioning operations, and their results; `--dry-run` shows what would be provisioned/run without executing hooks or writing files. Both are essential for debugging in an autonomous workflow where a human isn't watching.

---

## 8. CLI surface (sketch)

| Command                                        | Purpose                                            |
| ---------------------------------------------- | -------------------------------------------------- |
| `yg new <branch> [base]`                       | Create + provision + hook a worktree               |
| `yg setup <branch>`                            | Re-provision/re-hook an existing tree (idempotent) |
| `yg list` / `yg ls`                            | Status dashboard (`--json`)                        |
| `yg cd <branch>` / `yg switch`                 | Move between trees (shell integration)             |
| `yg remove <branch>`                           | Tear down (with safety checks)                     |
| `yg prune`                                     | Reconcile stale/locked worktrees                   |
| `yg merge [target]`                            | Gated merge-back                                   |
| `yg sync`                                      | Update branch from trunk                           |
| `yg trust [--show\|--revoke\|--from <f>]`      | Manage hook trust                                  |
| `yg config [--json] [--profile p]`             | Show effective config + provenance                 |
| `yg cc-worktree-create` / `cc-worktree-remove` | Claude Code hook adapters                          |

---

## 9. Config schema (illustrative `.yggdrasil.toml`)

```toml
[general]
trunk = "main"                 # auto-detected if omitted
path_template = "../{repo}.{branch}"
merge_strategy = "squash"      # squash | merge | ff
sync_strategy  = "rebase"      # rebase | merge

[provision]
copy = [".env", ".env.*", "appsettings.Development.json"]
link = ["certs/dev.pem"]
# patterns are gitignore-style; absolute paths and `..` are rejected;
# symlinks are not followed on copy.

[hooks]
pre_create  = []
post_create = ["pnpm install"]
pre_remove  = []
pre_merge   = ["pnpm test", "pnpm lint"]
post_merge  = []

[events]
sink = "jsonl"                 # jsonl | file | command
target = "tcp://chimera-bus:4711"   # for command/socket sinks

# --- Profiles -------------------------------------------------
[profiles.human]
secret_policy = "full"

[profiles.agent]
secret_policy = "template"     # full | template | inject
# inject example:
# secret_policy = "inject"
# secret_command = "scripts/mint-scoped-token.sh"
post_create = ["pnpm install --prefer-offline"]
```

Local override (`~/.config/yggdrasil/<repo>.toml`, always trusted) can point at the real source `.env` path, set a personal default profile, etc.

---

## 10. Security model (consolidated)

The threat model is: a repository's committed config can execute code on anyone who creates a worktree, and an agent's worktree may receive credentials it should not hold.

1. **Untrusted-by-default hooks.** Committed hook/provision commands do not run until `yg trust` approves them; trust is hashed on the command set and invalidated on any change (FR-20–25). Mirrors gtr's trust step but extends invalidation to the full provisioning command set.
2. **Path-traversal containment.** Provisioning cannot read/write outside the worktree, cannot use absolute paths, cannot follow symlinks (FR-12). Closes the "symlink `.env` to exfiltrate" vector.
3. **Secret scoping.** Agent profiles never get `full` secrets by default; `template`/`inject` policies keep live credentials out of autonomous trees (FR-26–28). This is the gap none of the existing tools close.
4. **Fail-closed headless mode.** No silent trust prompts in orchestrator context; trust is granted explicitly and auditably (FR-24).
5. **Explicitly deferred:** hook sandboxing/timeouts and resource limits. v1 documents that a trusted hook runs with the user's full privileges — `yg trust` means "I would `bash` this." Sandboxing is a post-v1 investigation.

---

## 11. Implementation notes (Go vs Rust)

Both produce a single static binary with negligible startup cost; either is fine. The lean:

- **Go** — faster to ship, best-in-class CLI ergonomics (cobra/viper), trivial cross-compile (`GOOS`/`GOARCH`), and shelling out to `git` (the pragmatic choice all these tools make) is idiomatic. Recommended if the priority is getting `yg` into the Chimera loop quickly.
- **Rust** — stronger guarantees on the security-critical paths (the path-traversal and trust-hashing logic benefit from the type system), `clap` is excellent, and `gitoxide` offers pure-Rust git interop if we ever want to stop shelling out. Better if correctness of the safety code is the top priority and the steeper ramp is acceptable.

**Recommendation:** start in **Go** for v0.1 to validate the workflow and the Chimera integration; the design is language-agnostic and the security invariants (trust hashing, path checks) are small enough to port if a Rust rewrite ever earns its keep. Treat `git` as the source of truth and shell out either way.

Key decisions to lock before coding: trust-store format and location, event schema version, the `--json` schema contract, and the path template grammar.

---

## 12. Ecosystem integration (Chimera, Claude Code, tmux)

- **Chimera message bus.** `yg` publishes lifecycle events as JSONL (FR-30) so the orchestrator tracks tree state without scraping. Chimera owns the profile decision (sets `YG_PROFILE=agent`) and pre-authorizes trust (`YG_TRUST_FROM`) so headless creation never blocks on a prompt.
- **Claude Code.** `yg cc-worktree-create` lets `claude -w <name>` use our naming + provisioning while keeping Claude Code's native session/resume/cleanup handling.
- **tmux.** Shell integration + `yg ls --json` make it straightforward to drive a tmux layout (one pane per agent tree), consistent with the existing tmux-based orchestration.

---

## 13. Milestones

| Phase                       | Scope                                                                                                                             |
| --------------------------- | --------------------------------------------------------------------------------------------------------------------------------- |
| **v0.1 (MVP)**              | `new`, `ls`, `remove`, declarative `copy`/`link` with path safety, `post_create` hook, env contract. Single repo, single profile. |
| **v0.2 (Trust)**            | Trust boundary: untrusted-by-default committed config, `yg trust`, hash invalidation, fail-closed headless mode.                  |
| **v0.3 (Integration)**      | `--json` everywhere, stable exit codes, event emission to bus, `cc-worktree-create/remove`.                                       |
| **v0.4 (Profiles)**         | Profiles + secret scoping (`template`/`inject`), agent vs human defaults.                                                         |
| **v0.5 (Lifecycle polish)** | `setup`, `prune` (locked-entry reconciliation), `merge`/`sync`, shell integration, status dashboard fields.                       |
| **v1.0**                    | Cross-platform hardening, docs, hook-timeout, audit of trust + path-safety paths.                                                 |

---

## 14. Open questions & risks

- **Trust UX vs friction.** Re-approval on every hook change is correct but can annoy on actively edited config. Mitigation: scope trust to the _resolved command set_, and consider a `--trust-on-change-interactive` convenience for the primary author. Open.
- **Secret `inject` ergonomics.** Minting scoped short-lived creds per tree needs a credential source; the `secret_command` contract must be defined (what it receives, what it must output). Open.
- **Windows path handling.** Git Bash path conversion at the stdin/stdout boundary (the gtr/cygpath problem) is fiddly; scope as best-effort for v1.
- **Event schema churn.** Lock a versioned event schema early so Chimera consumers don't break.
- **Overlap with Claude Code native hooks.** If Claude Code's native worktree support grows, keep `yg`'s value in provisioning + trust + secret scoping, and stay a clean hook provider rather than competing on session management.
- **External-script trust gap.** FR-22 hashes the command string but not referenced script contents (see FR-22 note). A `post_create = ["./scripts/setup.sh"]` whose script is later modified to exfiltrate would not trigger re-approval. v0.2 should evaluate hashing file contents of local scripts referenced by hooks, or explicitly document this as accepted risk. Tied to §6.5.
- **`inject` credential lifecycle.** Scoped credentials minted by `secret_command` have no reaping mechanism — they persist in the worktree after `yg remove` unless the `post_remove` hook explicitly revokes them. The `inject` policy should define a `secret_revoke_command` or document that credential cleanup is the user's responsibility.

---

## 15. Success criteria

- A fresh `yg new <branch> --profile agent` produces a fully usable tree with no live production secrets, in one command.
- An agent orchestrated by Chimera can create, track, and reap worktrees entirely through the machine interface, with trust granted explicitly and no interactive prompt.
- Pulling a branch that modifies a committed hook does **not** execute that hook until re-approved.
- `yg` replaces the per-worktree manual setup step entirely for both human and agent workflows.
