# Mole ([tw93/Mole](https://github.com/tw93/mole)) — patterns to steal

Read 2026-09-06 from `main` (GPL-3.0 CLI). **Learn the design. Do not copy
code.** Mole is a general Mac cleaner; we are an **agent-first developer
disk audit** that later becomes a CLI. Different product, same class of
mistakes.

## What Mole is

Terminal-first macOS toolkit. One binary `mo`:

| Command | Job |
|---|---|
| `mo clean` | Known-safe caches, logs, leftovers, some dev caches |
| `mo purge` | Project artifacts (`node_modules`, `target`, …) with confirm |
| `mo analyze` | Disk explorer TUI; optional `--json` |
| `mo uninstall` | App + related files, exact bundle evidence |
| `mo optimize` | Bounded maintenance (DNS, Finder caches, …) |
| `mo status` | Live health dashboard + `--json` / NDJSON |
| `mo installer` | Leftover DMG/PKG |
| `mo history` | Operation log (`~/Library/Logs/mole/operations.log`) |

Every destructive command has `--dry-run`. Whitelist at
`~/.config/mole/whitelist`. Purge roots at `~/.config/mole/purge_paths`.

Architecture: `mole` is a **router only**. Logic lives in `lib/clean/`,
`lib/core/`, `lib/uninstall/`. Analyze/status are Go (`cmd/analyze`,
`cmd/status`). Data lists (`app_protection_data.sh`) are separate from
matching logic.

Agent contract: `AGENTS.md` is the source of truth; `CLAUDE.md` is a
symlink so Claude and Codex get the same rules.

## Patterns we should take

### 1. Split the surface the way Mole does

Do **not** make one command that both scans and deletes everything.

Map onto our CLI:

| Ours (planned) | Mole analogue | Notes |
|---|---|---|
| `freedisk scan` | `mo analyze --json` + dry-run of clean/purge | Report only |
| `freedisk artifacts` / `purge` | `mo purge` | Rebuildable project dirs, confirm, age-aware |
| `freedisk caches` | `mo clean` (dev subset) | **Report only.** Propose owner commands; never run them. |
| `freedisk worktrees` | (Mole does not have this) | Our research gap Mole missed |
| `freedisk simulators` | part of `mo clean` (Xcode runtimes) | Join on `runtimeIdentifier` |
| `freedisk delete <id>` | selected items in clean/purge | Human-named ids only. Never automatic. |
| scan is always non-mutating | `--dry-run` / `MOLE_DRY_RUN=1` | We have no live-delete scan. |
| `--json` | `mo analyze --json`, `mo status --json`, auto-JSON when piped | Agents must not parse TUI |

### 2. One deletion sink

Mole: every remove goes through `mole_delete` / `validate_path_for_deletion`
(`lib/core/file_ops.sh`). Raw `rm -rf` needs `# SAFE: <reason>` on the
same line; CI greps for it.

We should: one `delete` path that checks absolute path, no `..` as a
component, no control chars, symlink-resolved ancestors, deny `/System`
`/usr` (except `/usr/local`) `/bin` `/sbin`, refuse empty-variable collapse
onto `/Users` or `$HOME`. Never delete from a **partial** timed-out scan.

### 3. Dry-run and real mode share one candidate plan

Mole insists the preview list is the same set delete would touch. Homebrew
`autoremove` is preview-first. Third-party owner commands (pnpm, uv, gh)
must expose mutated roots machine-readably.

We should: `scan` emits findings and never mutates. `delete` only
accepts ids the **human** listed from that scan (or a fresh scan) and
confirms each. No `--all`. No agent-initiated delete. No second hidden
matcher at delete time.

### 4. Classify by recovery contract, not folder name

Mole’s AGENTS.md: re-download cost is not an automatic veto, but mixed
state is. They **will** `go clean -modcache` and `uv cache prune` via
owner commands; they **will not** blanket-delete HuggingFace/Torch
models, Cargo `git`, `~/.m2`, Deno dir, AI tool sessions.

Matches our `risk`: `safe-cache` vs `ask` vs `keep`. HuggingFace and
Codex sessions stay `ask`. Claude/Grok worktrees are `leftover-worktree`
only with age + listing, never a silent `clean`.

### 5. Prefer owner commands over `rm`

Examples from `lib/clean/dev.sh`:

- `npm cache clean --force` then residual `_cacache`
- `pnpm store prune` only if no pnpm process (**fail closed if busy**)
- `uv cache prune` after `uv cache dir`
- `conda clean --yes --index-cache --tarballs --logfiles`
- `corepack cache clean` with `COREPACK_ENABLE_DOWNLOAD_PROMPT=0`

Ask the tool for its cache path (`npm config get cache`, `pnpm store
path`, `uv cache dir`). Do not assume `~/.npm`.

Skip with a **reason**: `◎ pnpm cache · skipped (pnpm busy)`.

### 6. Project purge is a separate, conservative walker

`lib/clean/project.sh` / `mo purge`:

- Configurable roots (`~/.config/mole/purge_paths`); else discover
  `~/Projects`, `~/GitHub`, `~/dev`, plus `$HOME/*` that look like
  project containers
- **Artifact dirs are not project containers.** A stray `~/node_modules`
  must not be treated as N projects (each package.json). Same for
  `vendor/`, `Pods/`
- Depth 1–6, timeout per root (default 60s); incomplete scan is **not**
  published as delete candidates
- Group by project; interactive multi-select
- Last **7 days** of file activity → unselected by default
- `vendor/`: only Composer (`composer.json` parent) is purgeable; Rails
  and Go vendor are **protected**; unknown vendor protected
- `bin/` only if .NET (`*.csproj` + Debug/Release)
- Physical containment: `pwd -P` both sides; lexical prefix is not
  enough if an ancestor is a symlink
- `fd` if present, else pruned `find`
- Filter nested artifacts so you don’t delete `dist` inside
  `node_modules`

This is the right shape for our Phase 4 + `freedisk artifacts`.

### 7. Simulator join is by JSON id, not display name

Mole bug #1505: `simctl runtime list` prints `iOS 26.4.1`; `simctl list
devices` groups under `iOS 26.4`. Name-join marks every point release as
orphan and offers `runtime delete` for a runtime devices still use.

Rule we already wanted, now confirmed: join on `runtimeIdentifier` from
`-j`. Recommend delete only if `state: Ready`, `deletable: true`, and
exactly one image for that id.

### 8. Timeouts, busy-process guards, no hang

- Wall-clock budget + inner checkpoints
- Timed-out producer must not feed a delete loop
- `pgrep` for gradle daemon / pnpm; unknown process state = skip
- Detect `uv`/`gh`/`conda` with a short timeout before running cleanup
- Spinners write to `/dev/tty`; JSON/piped output stays clean

### 9. Agent-facing I/O

- `--json` and auto-JSON when stdout is not a TTY (`mo status | jq`)
- Operation log + `mo history --json`
- `AGENTS.md` as the contract agents implement against
- Tests with `MOLE_TEST_NO_AUTH=1` so CI never hits sudo/osascript

We already have `findings.schema.json`. Agents should get `--json` as
the primary interface; TUI is optional later.

### 10. Whitelist is exact strings, not globs

`is_whitelisted` is exact path match after `~` expansion. Glob whitelist
was rejected as a security issue. Users can add arbitrary paths in the
file even if they are not in the inventory list.

Default safety rows (Playwright, Ollama, Finder metadata, …) still apply
when a custom file exists for those **hard** protections; inventory rows
are opt-in.

### 11. Measured value before a new cleanup target

Mole: a new target needs bytes on a real app version, an explicit
non-target sibling list, and proof every cleanup path is protected.
“It looks like a cache” is not enough. Tiny macOS UI caches (wallpaper
previews) stay because they blank the UI for a few MB.

Fits our research: `~/Library/Application Support/com.apple.wallpaper`
is `keep`; kache 48G is `ask` until we know the store contract.

## What we should not copy

| Mole | Why not us |
|---|---|
| Consumer cleaner (Safari/Mail/Chrome caches, uninstall Photoshop) | Out of scope; agents need **dev** reclaim |
| `mo status` live dashboard | Not a disk recipe |
| `mo optimize` Spotlight/DNS | Not reclaim |
| Interactive TUI as the only UI | Agents need JSON first |
| Uninstall-by-bundle leftover matching | Dangerous; we are not AppCleaner |
| GPL-3 source | Reimplement patterns, don’t paste |
| Homebrew `android-commandlinetools` share path | **Mole’s whitelist still uses `~/Library/Android` / `~/.android`** — we must keep our Homebrew SDK discovery |
| Agent worktrees | Mole does not scan `~/.grok/worktrees` or `.claude/worktrees` |
| `pyvenv.cfg` | Mole purge is name-based artifacts; we keep marker files |
| Docker.raw allocated vs apparent | Analyze TUI may show size; we must report both |

## Product filter (steal this question list)

Before we add a command or target:

1. Does it belong to scan / caches / artifacts / worktrees / simulators / delete / history?
2. Safe by default, previewable, testable without real sudo, explainable on one screen?
3. Can the user (or agent) see the exact paths before anything changes?
4. Is the data rebuildable, disposable, or backed by exact evidence?
5. Would this be better as a warning or “not supported”?

## Pointers in their tree

- Safety design: `docs/SECURITY_DESIGN.md`
- Agent rules: `AGENTS.md`
- Purge walker: `lib/clean/project.sh`, `bin/purge.sh`
- Dev caches: `lib/clean/dev.sh`
- Whitelist inventory: `lib/manage/whitelist.sh` (`get_all_cache_items`)
- Path validator: `lib/core/file_ops.sh`
- Analyze JSON: `cmd/analyze/json.go`
