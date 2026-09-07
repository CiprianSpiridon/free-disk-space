# RECIPE — macOS disk audit for agents

This is the spec the `freedisk` CLI encodes. Agents (Claude Code, Codex,
Grok, Cursor, …) run `freedisk scan --quick --json` (or `--dev` / full)
to answer “what is eating disk on this Mac?” and propose reclaim commands.

Do not invent a different scan order. The shell recipe below is the
fallback if the binary is not built.

Companion files:

| File | Role |
|---|---|
| **This file** | Procedure, invariants, finding contract, discovery rules |
| `catalog/macos-hotspots.yaml` | Path list, artifact names, APIs, thresholds |
| `findings.schema.json` | JSON shape every scan must emit |
| `research/mole-patterns.md` | Patterns from [tw93/Mole](https://github.com/tw93/mole) — steal design, not code |
| `NOTES.md` | Research diary + one-machine snapshot (not the runtime spec) |

OS: **macOS / APFS only** (first version). Linux is a different catalog.

---

## 1. What the agent / CLI must produce

A **report-only** scan. **No delete is ever automatic.** The CLI, and
any agent following this recipe, must never remove, trash, prune, or
uninstall anything unless a human named the exact finding ids and
confirmed. `scan` has no delete path. `delete` is opt-in, never default,
never `--all`, never implied by “safe-cache”.

Answer:

1. How full is the disk, really? (not `df /`)
2. What is using space, whole-disk then drilled
3. Which buckets are reclaimable vs keep
4. The exact reclaim command and risk for each finding

Emit:

- Human summary (markdown): fullness, top reclaimable, top keep
- Machine findings: JSON **object** matching `findings.schema.json` (host, volume, findings[])

One finding = one reclaimable or notable path. Do not dump every 4 KB
folder.

---

## 2. Invariants (violate these and the scan is wrong)

1. **No sudo.** Skip `EPERM` / TCC paths; list them as `unreadable`.
2. **Never walk** `/System`, `/usr` (except `/usr/local`), `/bin`,
   `/sbin`, `/private/var/vm`, Preboot, Recovery, `/dev`.
3. **Do not start with `du -sh /` or `du -sh ~`.** Too slow, wrong
   volume, thrash. Known paths first.
4. **Allocated size only.** `du -sk` or `st_blocks * 512`. Never
   `ls -l` / `st_size` for disk images (`Docker.raw` looked like 288 GB
   and used 85 GB).
5. **No symlink follow.** `os.walk(followlinks=False)`, no `du -L`.
6. **No glob of huge directories.** `du -sh dir/*` hits ARG_MAX on
   `~/Library/Preferences`. Use `du -d 1 dir`.
7. **Missing path = skip**, not fail.
8. **Ground truth for fullness:** `diskutil apfs list` container
   Capacity In Use / Not Allocated. `df /` is the sealed system
   snapshot (~17 GB) and will lie.
9. **No delete is ever automatic.** Not on scan, not on “safe-cache”,
    not on leftover worktrees, not because a tool is idle. The only
    delete path is `delete` with **explicit finding ids the human
    listed**, after they confirmed. Agents print reclaim commands; they
    do not run them unless the human said to run that id.
10. **Default action is report.** `risk: never` items are not reclaimable.
11. **Parallelize by tree, not by overlapping `du` of the same tree.**

---

## 3. Finding contract

```json
{
  "id": "npm-cache",
  "path": "/Users/you/.npm/_cacache",
  "bytes": 15032385536,
  "category": "npm-cache",
  "risk": "safe-cache",
  "last_used": "2026-08-15",
  "why": "npm package tarball cache",
  "reclaim": {
    "cmd": "npm cache clean --force",
    "dry_run": "du -sh ~/.npm/_cacache"
  }
}
```

`risk` enum:

| risk | Meaning | Agent / CLI |
|---|---|---|
| `safe-cache` | Recreated on next use | Propose command only |
| `rebuildable` | Recreated by install/build | Propose command only |
| `leftover-worktree` | Agent/git checkout whose session is gone | Propose after showing branch + age |
| `unused-runtime` | Simulator / AVD / toolchain never used or superseded | Propose after showing lastBooted / current |
| `ask` | Large, maybe still wanted | Propose only; extra confirmation if they later `delete` |
| `keep` | Source, photos, current tools | Hide from reclaim list |
| `never` | System, keychains | Do not list as reclaimable |

Propose ≠ execute. Printing `npm cache clean --force` is allowed.
Running it is not, until the human applies that finding id.

`bytes` is **allocated**. For sparse files also set `bytes_apparent`.

Thresholds (from the YAML; change there, not here):

- drill children if ≥ 1 GiB
- mention in human report if ≥ 50 MiB
- list individual artifact dirs if ≥ 5 MiB
- leftover-worktree if dir mtime ≥ 14 days (configurable)
- rebuildable artifacts (`node_modules`, Composer `vendor`, `target`, `.next`, venvs, …) always listed if ≥ 5 MiB; **untouched ≥ 30 days** (configurable `artifact_idle_days`) go in Reclaimable (high confidence) because they regenerate with install/build; **newer** ones go in Ask first. The human still picks ids — nothing is deleted automatically. Git-tracked paths stay keep.
- **tmp by default:** `/tmp`, `/var/tmp`, `$TMPDIR` (macOS per-user `/var/folders/…/T`). Size them in phase 1; always depth-1 children. **Never delete the tmp root.** Children ≥ 5 MiB are listed; idle ≥ 7 days (`tmp_idle_days`) go in Reclaimable (high confidence), newer in Ask first. Deduplicate `/tmp` → `/private/tmp`. EPERM children → `unreadable`.

---

## 4. Scan pipeline (encode this order)

### Phase 0 — Volume inventory (seconds)

```bash
df -h / /System/Volumes/Data
diskutil list
diskutil apfs list
diskutil info /System/Volumes/Data
tmutil listlocalsnapshots /
sysctl vm.swapusage
```

Record: container size, in-use, free, Data used, snapshot names, swap.
Ignore OS update snapshots (`com.apple.os.update-*`, `MSUPrepareUpdate`)
as reclaim targets.

If `df /` used% is low and Data used% is high, say so in the summary.
That is the APFS sealed-volume trap.

### Phase 1 — Known-path catalog (seconds–1 min)

For every `path` in `catalog/macos-hotspots.yaml` that exists, `du -sk`.
Do not walk unknown trees yet.

This phase is where Docker, Grok, Codex, npm, Xcode, Homebrew Android
SDK, uv, pnpm, and tmp roots (`/tmp`, `/var/tmp`, `$TMPDIR`) usually show up.
Tmp roots with `always_drill` also emit depth-1 children here (do not wait
for the 1 GiB drill phase).

### Phase 2 — Depth-1 of big roots

```bash
du -d 1 -h "$HOME" 2>/dev/null | sort -hr
# skip doing Library as part of that same wait — split:
du -d 1 -h "$HOME/Library" 2>/dev/null | sort -hr
du -d 1 -h /Applications 2>/dev/null | sort -hr
du -sh /opt/homebrew /opt/homebrew/Cellar /opt/homebrew/Caskroom 2>/dev/null
du -sh /Library/Developer /Library/Developer/CoreSimulator 2>/dev/null
```

Anything ≥ 1 GB becomes a Phase 3 drill. Home depth-1 should skip waiting
on `Library` if Library is a separate job.

### Phase 3 — Drill

For each ≥ 1 GB directory: `du -d 1 -h "$dir" | sort -hr`.
Stop when children are < ~500 MB or the child is a known file
(`Docker.raw`). Recurse once more on any child still ≥ 1 GB.

### Phase 4 — Pruned artifact walk (work roots only)

Walk configurable work roots. Bundled defaults (skip if missing):
`~/work`, `~/src`, `~/Projects`, `~/dev`, `~/code`, `~/GitHub`,
`~/gitlab`, `~/repos`, `~/Developer`, `~/workspace`, `~/go`, Desktop,
Documents, Downloads, plus `$HOME` depth-1 dirs that look like project
containers (`work_root_discover`: `.git` / `package.json` / `Cargo.toml`
or ≥2 child git repos). Overlay can add more; it is **not** required.
**Not** all of `$HOME`. **Not** `~/Library`.

Rules:

- `topdown=True`, `followlinks=False`, max depth 8
- If dir name is an artifact (YAML `artifacts`): record, **prune**
- If dir is `.git`: prune
- Also record **marker files** (see §5) even when the folder name is not
  `.venv` / `vendor`

Then size recorded paths (allocated) and group by kind.

### Phase 5 — Worktrees

See §6.

### Phase 6 — Simulator / emulator APIs

See §7. Do not only `du`. Ask `simctl` / `avdmanager`.

### Phase 7 — Package managers / toolchains

```bash
node -v; nvm ls 2>/dev/null
rustup toolchain list 2>/dev/null
python3 -V; pipx list 2>/dev/null; uv tool list 2>/dev/null
ruby -v; gem env 2>/dev/null
brew --prefix
xcode-select -p
docker system df 2>/dev/null || true   # engine may be down
```

Compare *current* toolchain to *installed* versions (nvm vs Homebrew
node is a common mismatch).

### Phase 8 — Classify and report

Every finding gets risk + reclaim cmd. Human summary lists reclaimable
total **without double-counting** (uv archive venvs live inside
`~/.cache/uv`; do not add them again).

---

## 5. Discovery rules (names lie; use markers)

The first research pass missed Python tool venvs, Homebrew Android SDK,
and leftover agent worktrees by trusting default paths and folder names.
The CLI must use **markers**.

### 5.1 JavaScript / Node

| Marker | What | Confirm |
|---|---|---|
| dir `node_modules` | deps | parent `package.json` |
| dir `.next` | Next.js cache | — |
| dir `.turbo`, `.parcel-cache` | bundler cache | — |
| dir `dist` / `build` / `out` | build output | not committed; not an app named `build` |
| `~/Library/pnpm`, `~/.npm/_cacache`, `~/.bun`, Yarn cache | global | — |

Prune `node_modules` so you do not count nested copies twice.

### 5.2 Python — `pyvenv.cfg`, not `.venv`

A venv may be named `.venv`, `headroom-ai`, `.browser-use-env`, or
`~/.cache/uv/archive-v0/<hash>`.

The directory that **contains** `pyvenv.cfg` **is** the environment.

Also size: `~/.cache/uv`, `~/Library/Caches/pip`, `~/.local/pipx`,
`~/.local/share/uv`, `~/.cache/huggingface`, `~/Library/Python`,
`~/.pyenv`, conda prefixes, `~/.virtualenvs`.

`env/` without `pyvenv.cfg` is a false positive. `site-packages` inside a
venv is already counted — do not add it again.

### 5.3 Ruby

`rbenv` / `rvm` / `asdf` / `chruby` if present (`~/.rbenv/versions` can
be huge). `gem env` for GEM PATHS. Marker: `Gemfile` → sibling
`vendor/bundle` and `.bundle`. CocoaPods: `Podfile` → sibling `Pods/`,
plus `~/Library/Caches/CocoaPods`. Skip Podfiles inside SwiftPM
checkouts (`/.build/checkouts/`, `Caches/kensi-spm/checkouts/`).

`vendor/` next to `composer.json` is PHP, not Ruby.

### 5.4 Rust

Dir `target` next to `Cargo.toml`. Global: `~/.cargo/registry`,
`~/.rustup/toolchains`. Extra toolchains vs `rustup show` default.

Worktree copies of `target/` are the expensive case (not shared).

### 5.5 PHP / Composer

Dir `vendor` **and** parent `composer.json` / `composer.lock`.
Skip `resources/views/vendor` (Laravel views).

### 5.6 Go / Java / Gradle / Swift

`~/go/pkg`, `~/Library/Caches/go-build`, `~/.gradle`, `~/.m2`,
`~/Library/Caches/org.swift.swiftpm`, dir `.build` (Swift).

### 5.7 Project copies

Directories whose names contain ` copy`, `-bk-`, `.bak`, `-backup` each
have their own `node_modules` / `.next` / `dist`. Flag as likely stale.

---

## 6. Worktrees (agent leftover checkouts)

Three products leave full working trees. Git objects may be shared;
**build artifacts are not.**

| Kind | Where | How to list |
|---|---|---|
| Git | `<repo>/.git/worktrees/` | `git -C <repo> worktree list --porcelain` |
| Claude Code | `<repo>/.claude/worktrees/<name>/` | walk work roots for that suffix |
| Grok | `~/.grok/worktrees/<project>/subagent-<id>/` | `du -d 1` |
| Ulpi | `<repo>/.ulpi/worktrees/` | walk |
| Codex | `~/.codex/worktrees/` if present | walk |

A `.git` **file** (not directory) means a linked worktree.

`prunable` in `git worktree list` = gitdir points at a deleted path;
safe to `git worktree prune`.

Age: `mtime` of the worktree directory. Idle > 14 days →
`leftover-worktree`. Still `ask` if mtime is recent.

Do not `du` every agent id in the human summary; size the parent
`.claude/worktrees` / grok project folder and list count.

---

## 7. Simulators and emulators

### 7.1 Apple (Xcode)

Two different buckets:

| Bucket | Path | API |
|---|---|---|
| Runtimes (OS images) | `/Library/Developer/CoreSimulator/Volumes` | `xcrun simctl list runtimes` |
| Devices (user data) | `~/Library/Developer/CoreSimulator/Devices` | `xcrun simctl list devices -j` |
| DeviceSupport (symbols) | `~/Library/Developer/Xcode/iOS DeviceSupport` | folder name = device + iOS |
| DerivedData | `~/Library/Developer/Xcode/DerivedData` | `safe-cache` |
| Xcode.app | `/Applications/Xcode.app` | `keep` if any iOS work |

`lastBootedAt` missing → unused device. Runtime with only never-booted
devices → unused-runtime.

Deleting devices does **not** delete the 16 GB iOS runtime.

Old DeviceSupport (iOS version no longer on the plugged-in phone) is
unused-runtime.

### 7.2 Android (do not stop at `~/Library/Android`)

Studio default is `~/Library/Android/sdk`. **Homebrew cask
`android-commandlinetools` installs to**

`/opt/homebrew/share/android-commandlinetools`

`ANDROID_HOME` may be unset even when that tree exists. `which emulator`
may fail; the binary is `$SDK/emulator/emulator`.

```bash
SDK="${ANDROID_SDK_ROOT:-${ANDROID_HOME:-/opt/homebrew/share/android-commandlinetools}}"
du -sh "$SDK" "$SDK/system-images" "$SDK/emulator" ~/.android/avd
avdmanager list avd
"$SDK/emulator/emulator" -list-avds
sdkmanager --sdk_root="$SDK" --list_installed
```

`system-images` with **zero AVDs** = unused-runtime (the image is
installed, no device uses it). Also probe Genymotion / BlueStacks / Nox
apps if present.

`~/Library/Caches/Google` is often **Chrome**, not Android Studio. Check
the child names.

---

## 8. Docker / VMs

```bash
du -sk ~/Library/Containers/com.docker.docker/Data/vms
ls -l ~/Library/Containers/com.docker.docker/Data/vms/0/data/Docker.raw
docker system df   # only if engine is up; do not start Docker just to look
```

Report **both** allocated (`du`) and apparent (`ls`) for `Docker.raw`,
`*.qcow2`, `*.raw`. Engine down → still report the file; do not fail.

Colima / Lima / OrbStack: `~/.colima`, `~/.lima`, `~/.orbstack`.

---

## 9. Whole-disk map (what “cover the disk” means)

Scan these. Skip the rest unless Phase 0 says full and 1–7 did not
explain it.

**Skip always:** `/System`, `/usr` except `/usr/local`, `/bin`, `/sbin`,
`/private/var/vm`, Preboot, Recovery, `/dev`.

**Must cover:**

```
$HOME                          # depth-1, then drill
$HOME/Library/{Containers,Caches,Application Support,Developer,Preferences,Logs,pnpm,Group Containers}
/Applications
/opt/homebrew  and/or  /usr/local
/Library/Developer
work roots (see Phase 4)
agent homes: ~/.grok ~/.codex ~/.claude ~/.ulpi ~/.cursor
language homes: ~/.npm ~/.nvm ~/.bun ~/.cargo ~/.rustup ~/.cache ~/.local
Android SDK homes (Studio + Homebrew share path)
Xcode DeviceSupport + CoreSimulator (user + system)
/tmp  /var/tmp  $TMPDIR            # children only; never the root
```

`~/Library/Preferences` is usually tiny; if it is gigabytes, drill with
`du -d 1` (never glob). Can be thousands of medium plists.

iCloud (`~/Library/Mobile Documents`, Desktop/Documents if iCloud Drive):
`du` is not “on this disk” if Optimize Mac Storage is on → `keep`, do
not promise reclaim.

---

## 10. Reclaim commands (print only)

These strings go on the finding as `reclaim.cmd`. **Never execute them
from scan, from an agent loop, or because risk is `safe-cache`.** The
human must run `freedisk delete <id>` (or paste the command themselves)
for each id they want.

Map category → command. The CLI stores this next to the finding.

| Category | Typical cmd |
|---|---|
| npm-cache | `npm cache clean --force` |
| pnpm-store | `pnpm store prune` |
| bun-cache | `bun pm cache rm` |
| yarn-cache | `yarn cache clean` |
| uv-cache | `uv cache clean` |
| pip-cache | `pip3 cache purge` |
| cargo-registry | delete `~/.cargo/registry` or `cargo cache -a` |
| rust extra toolchain | `rustup toolchain uninstall <name>` |
| nvm extra version | `nvm uninstall <ver>` |
| go-modcache | `go clean -modcache` |
| gradle-cache | `rm -rf ~/.gradle/caches` |
| homebrew-cache | `brew cleanup -s` |
| xcode-deriveddata | `rm -rf ~/Library/Developer/Xcode/DerivedData` |
| unused sim device | `xcrun simctl delete <udid>` |
| unused sim runtime | `xcrun simctl runtime delete <id>` |
| unused DeviceSupport | `rm -rf "~/Library/Developer/Xcode/iOS DeviceSupport/<old>"` |
| unused android image | `sdkmanager --sdk_root="$SDK" --uninstall 'system-images;...'` |
| rust `target` | `cargo clean` in that tree |
| node_modules | `rm -rf <path>` (rebuildable) |
| `.next` | `rm -rf <path>` |
| composer vendor | `rm -rf <path>` then `composer install` |
| leftover git worktree | `git worktree remove <path> --force` |
| leftover grok worktrees | `rm -rf ~/.grok/worktrees/<project>` after listing |
| leftover claude worktrees | `git worktree remove` + delete `.claude/worktrees/<name>` |
| docker (engine up) | `docker system prune -a --volumes` then compact in Docker Desktop |
| trash | `rm -rf ~/.Trash/*` |

Never auto-delete anything — not `safe-cache`, not `ask`, not `never`.
Never delete OS update snapshots. There is no `delete --all`.

---

## 11. Human report the agent must write

```markdown
## Disk
- Container: X GB in use / Y GB free (Z%)   # from diskutil apfs list
- Data volume: ...
- Note if df / disagrees

## Reclaimable (high confidence)
table: bucket, size, risk, command

## Ask first
table: bucket, size, why

## Keep / not reclaim
one line each (Photos, current Xcode, source trees)

## Not present
e.g. no conda, no Android AVDs, no rbenv — so the user knows we looked
```

“Not present” matters. Agents otherwise skip Python/Ruby/Android because
the default folder is empty.

---

## 12. Performance

The CLI must **not** shell out to `du` in a loop. `du` forks, walks, and
prints; doing that per catalog path thrashes the SSD. Size in-process:

- Allocated bytes: `lstat` / `st_blocks * 512` (same as `du -sk`, no
  subprocess). Apparent: `st_size` only for sparse files.
- Directory size: one walk (`ReadDir` / `getattrlistbulk` on Darwin),
  no symlink follow, prune artifact names.
- Depth-1: `ReadDir` + size each child, not a recursive walk of the
  parent and of every child.
- Parallelize **disjoint trees only**, bounded pool (about 4). Never
  two walkers on the same path.
- `diskutil apfs list` once per scan. `simctl` only in the sim phase.
  `git worktree list` / `git ls-files` once per repo, not per file.
- No `find ~ -name node_modules`. No `du -sh dir/*` (ARG_MAX).
- Cache by realpath so `/tmp` and `/private/tmp` are one walk.

Phase order still matters: known paths before any home walk. Split
`~/Library/Containers` (Docker.raw) and `~/Library/Caches` into
separate pool jobs.

---

## 13. CLI shape

Mole’s split is the right product shape (see `research/mole-patterns.md`).
We are narrower: developer reclaim for agents, not a consumer Mac cleaner.

```text
freedisk scan              # phases 0–8, JSON + markdown. Never deletes.
freedisk scan --quick      # volume + known + tmp children (e.g. /private/tmp/kensi-*). Never deletes.
freedisk why               # top reclaimable (report)
freedisk catalog / scans   # optional overlay; bundled catalog is the starting point
freedisk delete <id> [<id>…]  # ONLY mutate path. Explicit ids. Confirm each.
                             # No --all. No default. Agents must not call
                             # this unless the human named those ids.
freedisk history           # optional operation log
```

Flags: `--json` (and JSON when stdout is not a TTY), `--debug`.
There is no `--force` on scan. `delete` always confirms.

No network required. No sudo. Exit 0 on a complete report even if some
paths were unreadable.

Implementation notes stolen from Mole:

- Router binary vs modules; one deletion sink; path validator
- Dry-run (scan) and delete share the **same** candidate list
- Prefer `uv cache prune` / `pnpm store prune` over `rm` of the store
- Fail closed if the package manager process is running
- Timed-out scans must not become delete candidates
- Purge: configurable roots, artifact dirs are not project containers,
  vendor is Composer-only, last 7 days unselected by default
- Do not copy Mole’s Safari/Mail/uninstall-Photoshop surface

Implementation: load `catalog/macos-hotspots.yaml`; emit
`findings.schema.json`; fullness from `diskutil apfs list`.

---

## 14. Research fixtures (this machine, 2026-09-06)

Use these as regression checks, not as hardcoded sizes.

| If the scanner only looks at… | It will miss… |
|---|---|
| `df /` | Data volume 85% full, 141 GB free |
| `~/Library/Android/sdk` | Homebrew SDK 5.6 GB, unused API 35 image 3.8 GB + emulator 1.1 GB, **zero AVDs** |
| dirs named `.venv` | 32 `pyvenv.cfg` envs (pipx 1.6G, uv tools, `.browser-use-env`); uv cache 5.5G |
| `~/.rbenv` | (correctly empty here) but must still `gem env` + `vendor/bundle` |
| git objects only | Grok worktrees 41 GB of Rust `target/`; Claude `.claude/worktrees` 31 GB in KensiApp |
| `ls Docker.raw` | 288 GB apparent vs 85 GB allocated |
| folder name `vendor` | Laravel views vs Composer vs Rails mix |
| `which emulator` | binary at `$SDK/emulator/emulator`, not on PATH |

Largest reclaimable classes seen: leftover agent worktrees, Rust
`target/`, npm cache, `.next`, unused Apple watchOS runtime, unused
Android system-image, uv/pip caches, extra nvm/rustup toolchains.

---

## 15. How an agent should run this *today* (no CLI yet)

1. Read this file and `catalog/macos-hotspots.yaml`.
2. Execute phases 0 → 8 in order. Do not skip Python/Ruby/Android
   because the first default path was empty.
3. Write findings JSON + the human report in §11.
4. Do not delete anything. Do not run reclaim commands. Do not call
   `delete`. Print proposals only.
5. If you learn a new hotspot path, add it to the YAML catalog and a
   one-line lesson to this recipe — do not only mention it in chat.
