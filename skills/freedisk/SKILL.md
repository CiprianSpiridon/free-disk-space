---
name: freedisk
description: >
  Audit macOS disk usage with the freedisk CLI and reclaim space only by
  explicit finding ids. Use when the user asks what is eating disk, to scan
  tmp/node_modules/worktrees/simulators, to free disk space, or runs
  /freedisk. Scan never deletes.
---

# freedisk

macOS/APFS CLI. **Scan never deletes.** The only mutate path is
`freedisk delete <id> [<id>…]` with ids the human named.

Prefer JSON when piping or when stdout is not a TTY:

```bash
freedisk scan --quick --json
freedisk scan --dev --json
freedisk scan --json
freedisk why --json
```

## How long to wait

Do **not** kill a scan that is still printing `freedisk: phase …` or
`still walking …` on stderr. JSON/markdown is on stdout only.

| mode | typical (full developer Mac) | worst we budget |
| --- | --- | --- |
| `--quick` | 1–5 min | ~10 min if `$HOME` has a 100GB+ work dir |
| `--dev` | 3–10 min | artifacts + worktrees on top of quick |
| default / full | 5–15 min | drill capped at ~3 min, then simctl / Android / docker / brew |

Empty or small disks finish in tens of seconds. Start with `--quick`.
Run `--dev` when you need `node_modules` / `target` / leftover worktrees.
Run full only for simulators or `docker system df` / `brew autoremove`.

## Modes

- `--quick` — volume + known catalog paths + **home/Library depth-1** + **tmp children** (not cargo-target-only). Named hotspots (kache, Docker.raw, uv, Playwright) are listed even when they sit under a larger parent. `~/work*` is globbed so `~/work_cip` shows up. Catches `/private/tmp/kensi-*` and other fat `/tmp` dirs.
- `--dev` — leftover worktrees + project artifacts (`node_modules`, `target`, `.next`, venvs) + catalog paths tagged `dev`.
- default / `--mode=full` — all enabled phases.

No overlay is required for a first scan. The bundled catalog is generic.
**This Mac** can differ: extra work roots, a fat cache the YAML never heard
of, or a toolchain the user does not have. After a scan, you **may** change
the per-user overlay (`freedisk catalog path` → `~/.config/freedisk/catalog.yaml`).
Never edit the bundled `macos-hotspots.yaml`. Overlay is not delete.

When to **add** (user named a path, or home depth-1 / Ask-first showed a
large dir that should be sized every scan):

```bash
freedisk catalog add PATH --scans quick,dev --risk ask --category user
freedisk catalog add '/Users/you/work_*' --glob --scans dev
```

When to **stop scanning** a path on this machine (user does not use it, or
it is too expensive / not theirs):

```bash
freedisk catalog unassign PATH --from quick     # drop from one mode
freedisk catalog disable PATH                   # skip entirely
freedisk scans disable artifacts                # skip a whole phase
```

`catalog list [--json]` shows the merged view. Re-scan after overlay changes.
`CanCatalog` refuses `/`, `$HOME`, `/System`, tmp roots, etc. — same class
as delete denials.

## Report rules

- Fullness from `diskutil apfs list`, not `df /`.
- Bytes are allocated (`st_blocks*512`).
- Tmp **roots** (`/tmp`, `/private/tmp`, `/var/tmp`, `$TMPDIR`) are keep. **Children** ≥ 5 MiB are listed. Idle ≥ 7 days or ≥ 1 GiB → high-confidence reclaimable in the markdown report; JSON `risk` stays `ask`.
- Markdown tables always include **id**, **full path**, **last used**, and a **command**. If the catalog has no tool-specific reclaim, the command is `rm -rf PATH`. Never invent a path from a truncated id — use the path column or JSON `.path`.
- Print reclaim commands. Do not run them from scan.
- Progress is on stderr (`freedisk: phase …`); JSON stdout stays clean. `--quiet` silences progress.
- Full `freedisk scan` time-budgets the drill phase. It keeps partial children and still prints the report. It does not abort with `drill timed out`.
- Worktrees: grok/claude/codex/ulpi reclaim is `rm -rf PATH`; git linked worktrees use `git worktree remove --force PATH`. Newest iOS DeviceSupport is keep; older folders unused-runtime. Extra rustup/nvm versions unused-runtime. simctl runtimes include Volumes bytes. `--dev`/full also run `docker system df` and `brew autoremove --dry-run` when those tools exist.

## Delete (only if the human listed ids)

```bash
freedisk delete <id> [<id>…]          # TTY: type yes per id
freedisk delete <id> --yes            # required when non-TTY
```

Never `--all`. Never automatic. Agents must not delete unless the human named those ids.

Refused: `keep`/`never`, git-tracked, in-flight worktrees, busy npm/pnpm/yarn/cargo/uv/docker, tmp roots, `/System`, `~/.cargo` as a whole, `/`, `$HOME`, whole Downloads/Desktop/Documents/Library, `/Library/Developer`, `/opt/homebrew`, iCloud `Mobile Documents`.

Stale last-scan (>24h) cannot be deleted against. Re-scan first.

## If `freedisk` is missing

```bash
go install github.com/CiprianSpiridon/free-disk-space/cmd/freedisk@latest
# or from a checkout:
go install ./cmd/freedisk
ln -sfn "$(go env GOPATH)/bin/freedisk" "$HOME/bin/freedisk"
```

Install this skill into every local agent CLI:

```bash
freedisk skill install
```
