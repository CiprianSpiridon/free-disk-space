# FreeDiskSpace — macOS disk audit notes

Research diary from the 2026-09-06 audit of one Mac. **Agents should
follow [RECIPE.md](RECIPE.md)**, not this file. This is the appendix:
why the recipe looks the way it does, and the snapshot we used as
fixtures.

This is **macOS / APFS specific**. Linux `df`/`du` intuition is wrong here.

---

## 1. Goal

Answer, quickly and safely:

1. How full is the disk, really? (APFS makes this non-obvious.)
2. What is using the space, from the whole disk down to named folders?
3. Which of that is **reclaimable** (caches, leftover worktrees, unused
   simulators, `node_modules` / `target` / `vendor` / `.next`, old
   toolchains) vs **keep** (source, photos, apps in active use)?
4. What exact command would reclaim each bucket, and how risky is it?

The CLI should **report only**. **No delete is ever automatic.** Cleanup
is a separate `delete` of explicit finding ids the human named and
confirmed — never `--all`, never implied by risk class.

---

## 2. macOS storage model (read this before any `du`)

A modern Mac is **not** one filesystem at `/`.

### 2.1 APFS container vs volumes

Physical disk (here: 1.0 TB SSD) holds an APFS **container**. The
container is split into volumes that **share the same free space**:

| Volume role | Typical mount | What it is | Touch? |
|---|---|---|---|
| System (sealed, snapshot-mounted) | `/` | Read-only OS (~17–20 GB) | Never |
| Data | `/System/Volumes/Data` (firmlinked into `/Users`, `/opt`, `/usr/local`, `/Library` user bits, `/Applications`) | Almost all user + app data | Yes, this is the scan target |
| Preboot | `/System/Volumes/Preboot` | Boot files | Never |
| Recovery | (often unmounted) | Recovery OS | Never |
| VM | `/System/Volumes/VM` | Swap / sleep image | Never (grows under memory pressure) |

**Consequence:** `df -h /` on a sealed system shows the **system snapshot**
(~17 GB used, 11%) even when the Mac is 85% full. Always also read:

```bash
df -h /
df -h /System/Volumes/Data
diskutil info disk3s5          # Data volume
diskutil apfs list             # container used vs free
```

On this machine (2026-09-06):

- Container: 994.7 GB, **842.5 GB used, 152 GB free (84.7%)**
- Data volume: **~742–796 GB used**
- `df` Available: **~141 GB**
- `/` (system snapshot): 17 GB — ignore this for "why am I full?"

### 2.2 Firmlinks

Paths like `/Users`, `/opt`, `/Applications`, `/Library` **look** like they
live on `/` but most of them are firmlinked onto the Data volume. Scanning
`/` recursively will either miss Data or double-count. **Scan Data-rooted
paths, not `/`.**

Practical roots to treat as the "whole disk" for a user:

```
/Users/<user>
/Applications
/opt/homebrew                  # Apple Silicon
/usr/local                     # Intel Homebrew, some tools
/Library                       # system-wide caches, Developer, CoreSimulator
/private/var                   # logs, tmp, folders — usually small, skip first
/System/Volumes/Data           # only as a last-resort whole-volume du
```

Never walk `/System`, `/usr` (except `/usr/local`), `/bin`, `/sbin`,
`/private/var/vm`, Preboot, Recovery.

### 2.3 Apparent size vs allocated size (sparse files, APFS clones)

macOS lies in both directions:

- **Sparse files:** `ls -lh` on Docker Desktop's VM showed **288 GB**.
  `du -sh` showed **85 GB**. The file is `Docker.raw`, a sparse disk
  image. The CLI **must use allocated size (`du` / `stat` blocks)**,
  never `st_size` from `lstat`.
- **APFS clones / copy-on-write:** `cp` and some "duplicate project"
  operations share blocks until one side writes. `du` can **over-count**
  clones if you sum children. `diskutil apfs list` container "in use" is
  the ground truth for "how full is the disk".
- **iCloud placeholders:** files in `~/Library/Mobile Documents` and
  Desktop/Documents (if iCloud Drive is on) may be evicted. `du` of those
  trees is not "on disk".

Use:

```bash
du -sh <path>                  # allocated 512-byte blocks, human
du -sk <path>                  # kilobytes, good for sorting
# Python: os.lstat(path).st_blocks * 512   — NOT st_size
```

### 2.4 Purgeable space and local snapshots

APFS can hold **purgeable** space (iCloud, sleep images, local TM
snapshots, OS update snapshots) that `df` may or may not count as free.

```bash
tmutil listlocalsnapshots /
tmutil listlocalsnapshotdates /
diskutil info /System/Volumes/Data | grep -E "Volume Used|Purgeable|Container Free"
```

On this machine, snapshots were **OS update snapshots only**
(`com.apple.os.update-*`, `MSUPrepareUpdate`), not Time Machine locals.
Do not delete OS update snapshots; they go away after the update.

### 2.5 Swap

```bash
sysctl vm.swapusage
```

Here: 7 GB swap, ~6 GB used. Lives on the VM volume. Not a cleanup
target unless the machine is thrashing (then it's a RAM problem).

---

## 3. What NOT to do

| Don't | Why |
|---|---|
| `sudo du -sh /` | Walks the sealed system, firmlinks, VM; hours; permission noise; wrong answer |
| `find / -name node_modules` | Same; also descends into every `node_modules` (nested copies) |
| `du -sh ~` as the first command | Works, but on a 700 GB home it takes 5–10+ minutes and blocks everything else |
| Follow symlinks (`du -L`, `os.walk(followlinks=True)`) | Infinite loops, double-count |
| Trust `ls -lh` for VMs / disk images | Sparse files |
| Delete without classifying | `vendor/` is Composer **or** Laravel `resources/views/vendor` (tiny, not reclaimable) |
| `rm -rf` DerivedData / simulators while Xcode is open | Corrupt simulator runtime |
| Clean Docker while it's the working engine | Need `docker system df` first, engine running |

---

## 4. Scan pipeline (CLI should implement this order)

The winning strategy on this audit: **known paths first, then depth-1,
then drill, then a pruned artifact walk**. Never a full-disk `find`.

### Phase 0 — Volume inventory (seconds)

```bash
df -h /
df -h /System/Volumes/Data
diskutil list
diskutil apfs list
diskutil info /System/Volumes/Data
tmutil listlocalsnapshots /
sysctl vm.swapusage
```

Record: container size, in-use, free, Data used, snapshot list, swap.

### Phase 1 — Known-path catalog (seconds to ~1 min)

`du -sh` each path in `catalog/macos-hotspots.yaml` **if it exists**.
Do not walk unknown trees yet. This already found Docker (85 GB),
Grok (48 GB), Codex (27 GB), npm cache (20 GB), Xcode (25+30 GB),
pnpm (9 GB), LM Studio (11 GB) on this Mac.

### Phase 2 — Depth-1 of the big roots (1–5 min)

```bash
du -sh /Users/$USER/*/ /Users/$USER/.[^.]* 2>/dev/null | sort -hr
du -sh /Users/$USER/Library/*/ 2>/dev/null | sort -hr
du -sh /Applications/*.app 2>/dev/null | sort -hr
du -sh /opt/homebrew /opt/homebrew/Cellar /opt/homebrew/Caskroom 2>/dev/null
du -sh /Library/Developer /Library/Developer/CoreSimulator 2>/dev/null
```

Anything ≥ 1 GB gets a name and goes to Phase 3.

**Gotcha:** `du -sh ~/Library/*` is itself a long walk (Caches + Containers
are huge). Run Containers / Caches / Application Support as **separate
jobs**, not one `Library/*` that waits on Docker.raw.

### Phase 3 — Drill (minutes, parallel, per-bucket)

For each ≥1 GB directory: `du -sh $dir/* | sort -hr | head`.
Stop when children are < ~500 MB or are a known file (e.g. `Docker.raw`).

### Phase 4 — Pruned artifact walk (minutes)

Walk **work roots only** (`~/work*`, `~/src`, `~/Projects`,
`~/.grok/worktrees`, `~/.codex`, Desktop/Documents/Downloads), **not**
all of `$HOME`.

When you see a directory named `node_modules`, `target`, `vendor`,
`.venv`, `.next`, `dist`, `build`, `Pods`, etc.: **record it and prune**
(do not descend). Nested `node_modules` inside `node_modules` are
already counted.

Then size the recorded paths (allocated bytes) and group by kind.

On this Mac: 1,163 artifact dirs, **96.9 GB** in those names alone.

### Phase 5 — Worktree discovery

Git worktrees, Claude `.claude/worktrees`, Grok `~/.grok/worktrees`,
Ulpi `.ulpi/worktrees`. See §9. These were among the largest reclaimable
buckets here (Grok 42 GB, Claude/KensiApp 31 GB).

### Phase 6 — Simulator / emulator APIs

Do not only `du`. Ask the platform:

```bash
xcrun simctl list runtimes
xcrun simctl list devices -j
# Android (if SDK present)
emulator -list-avds
ls ~/.android/avd
```

A runtime that is installed but whose devices have `lastBootedAt` missing
is unused. On this Mac: **all 5 watchOS simulators have never been
booted**, and the watchOS 26.2 runtime is ~8.3 GB.

### Phase 7 — Package-manager / toolchain APIs

```bash
npm cache ls -l 2>/dev/null | tail
du -sh ~/.npm/_cacache
pnpm store path
pnpm store status
brew autoremove --dry-run
rustup toolchain list
nvm ls
uv cache dir
docker system df          # only if engine is up
```

### Phase 8 — Classify and report

Every item gets: path, allocated bytes, category, last-used (mtime of
dir or API), reclaim command, risk (`safe-cache` / `rebuildable` /
`ask` / `never`).

Print a reclaimable total that is **honest**: caches + leftover worktrees
+ unused runtimes + extra toolchains. Do not promise that deleting
`node_modules` in an active repo "frees" space you will immediately
reinstall.

---

## 5. Command cookbook

### 5.1 Disk / volume

```bash
df -h / /System/Volumes/Data
diskutil list
diskutil apfs list
diskutil info /                          # system snapshot, misleading
diskutil info /System/Volumes/Data       # the real volume
tmutil listlocalsnapshots /
tmutil thinlocalsnapshots / 10000000000 4   # last-resort, TM locals only
sysctl vm.swapusage
```

### 5.2 Sizing a path

```bash
du -sh <path>                  # one path
du -sh <path>/* 2>/dev/null | sort -hr | head -30
du -d 1 -h <path> 2>/dev/null | sort -hr | head -30   # GNU/BSD du depth
```

macOS `du` is BSD: `-d depth` works. `sort -hr` understands `1.2G`.

Redirect stderr: SIP and TCC will spam `Operation not permitted` on
Mail, Photos internals, some TCC-protected folders. That's fine.

### 5.3 Listing without sizing

```bash
ls -la ~
ls -d ~/.[^.]*
ls /Applications
```

Use this to **build the path list**, then `du` only what exists.
`ls -lh` file size is **not** allocated size.

### 5.4 Finding named artifact directories (pruned)

Do **not** use:

```bash
find ~ -name node_modules -type d        # descends into them, very slow
```

Do use a pruned walk (Python sketch we ran, ~14 s on work roots):

```python
ARTIFACTS = {
    "node_modules", "vendor", "target", ".venv", "venv",
    "__pycache__", ".next", "dist", "build", "Pods",
    "DerivedData", ".gradle", ".turbo", ".parcel-cache",
    "bower_components", ".tox", ".mypy_cache", ".pytest_cache",
    ".ruff_cache", "Carthage", ".nuxt", ".output",
}
SKIP_HIDDEN = True  # except the artifact names themselves
# os.walk(..., topdown=True, followlinks=False)
# if d in ARTIFACTS: record; do not put d in dirnames
# if d == ".git": prune
```

Then size with `st_blocks * 512` (or `du -sk` of the recorded list).

False positives to filter after:

- `vendor/` without `composer.json` / `composer.lock` nearby → likely
  Laravel views or a named folder, not Composer.
- `env/` without `pyvenv.cfg` → not a venv (we saw 8 `env` dirs, 17 KB total).
- `build/` / `dist/` / `out/` in an app that *ships* those (check git).
- `target/` that is not a Cargo project (`Cargo.toml` missing).

Confirm:

```bash
test -f "$dir/../Cargo.toml"          # rust target
test -f "$dir/../package.json"        # node_modules
test -f "$dir/../composer.json"       # php vendor
test -f "$dir/pyvenv.cfg"             # venv
test -f "$dir/../next.config.js" -o -f "$dir/../next.config.mjs" -o -f "$dir/../next.config.ts"
```

### 5.5 Git worktrees

```bash
git -C <repo> worktree list --porcelain
git -C <repo> worktree list
git worktree prune --dry-run          # drops stale gitdir pointers
```

A `.git` **file** (not directory) means this checkout is a linked worktree:

```
gitdir: /path/to/main/.git/worktrees/<name>
```

`git worktree list` on the main repo lists all of them. If `prunable`
appears, the directory is already gone and only the git metadata remains.

### 5.6 Xcode / iOS / watchOS

```bash
xcode-select -p
du -sh /Applications/Xcode.app
du -sh ~/Library/Developer
du -sh ~/Library/Developer/Xcode/DerivedData
du -sh ~/Library/Developer/Xcode/Archives
du -sh ~/Library/Developer/Xcode/iOS\ DeviceSupport
du -sh ~/Library/Developer/Xcode/watchOS\ DeviceSupport
du -sh ~/Library/Developer/CoreSimulator
du -sh /Library/Developer/CoreSimulator
du -sh /Library/Developer/CoreSimulator/Volumes
xcrun simctl list runtimes
xcrun simctl list devices -j
xcrun simctl delete unavailable
# xcrun simctl runtime delete <identifier>     # unused runtimes
```

DeviceSupport folders are named `iPhone16,1 18.4 (22E240)` — keep the
one matching the phone you still plug in; old iOS versions are reclaimable.

Simulator **runtimes** live as disk images under
`/Library/Developer/CoreSimulator/Volumes/` (iOS_23C54, watchOS_23S303).
Device **data** lives in `~/Library/Developer/CoreSimulator/Devices/`.

### 5.7 Android (absent on this Mac, still catalog)

```bash
du -sh ~/Library/Android/sdk ~/.android
du -sh ~/Library/Android/sdk/system-images
du -sh ~/.android/avd
emulator -list-avds
```

### 5.8 Docker Desktop

```bash
du -sh ~/Library/Containers/com.docker.docker
ls -lh ~/Library/Containers/com.docker.docker/Data/vms/0/data/Docker.raw
# allocated:
du -sh ~/Library/Containers/com.docker.docker/Data/vms
docker system df                 # needs the engine running
docker system prune -a --volumes --dry-run
```

If the engine is **not** running, `docker system df` fails. Still report
the `Docker.raw` allocated size. Starting Docker just to inspect it can
itself grow the image.

Colima / Lima / OrbStack:

```bash
du -sh ~/.colima ~/.lima ~/.orbstack
colima status
```

### 5.9 Homebrew

```bash
du -sh /opt/homebrew /opt/homebrew/Cellar /opt/homebrew/Caskroom
du -sh ~/Library/Caches/Homebrew
brew cleanup -s --dry-run
brew autoremove --dry-run
```

### 5.10 Node / JS

```bash
du -sh ~/.npm ~/.npm/_cacache ~/.nvm ~/.bun ~/Library/pnpm ~/Library/Caches/pnpm ~/Library/Caches/Yarn
du -sh ~/.nvm/versions/node/*
npm cache verify
# npm cache clean --force
pnpm store prune
bun pm cache rm
```

nvm: compare `nvm current` / `node -v` to installed versions. Extra
versions are reclaimable. On this Mac, `node -v` is **v25.8.1** (Homebrew)
while nvm still holds v18, v20, three v22s, and v23 (**2.4 GB**, unused).

### 5.11 Python

```bash
du -sh ~/.cache/uv ~/.local/share/uv ~/Library/Caches/pip
du -sh ~/.pyenv ~/.conda ~/miniconda3 ~/anaconda3 ~/.virtualenvs ~/.local/share/virtualenvs
du -sh ~/Library/Python
uv cache dir
uv cache clean --dry-run
pip3 cache dir
pip3 cache purge
```

Project-local: `.venv/`, `venv/`, `env/` with `pyvenv.cfg`; `__pycache__/`.

### 5.12 Rust

```bash
du -sh ~/.cargo ~/.cargo/registry ~/.cargo/git ~/.rustup ~/.rustup/toolchains
rustup toolchain list
rustup toolchain uninstall <name>
# per-project:
# cargo clean   in each tree that has target/
```

`target/` in leftover **worktrees** is the expensive copy. Cleaning the
main repo is not enough if Grok/Claude cloned the tree.

### 5.13 Go / Java / PHP / Ruby / SwiftPM

```bash
du -sh ~/go/pkg ~/Library/Caches/go-build
du -sh ~/.gradle ~/.m2
du -sh ~/.composer ~/Library/Caches/composer
du -sh ~/.gem ~/.bundle ~/Library/Caches/CocoaPods
du -sh ~/Library/Caches/org.swift.swiftpm ~/Library/Caches/kensi-spm
```

### 5.14 Local AI models

```bash
du -sh ~/.ollama ~/.lmstudio ~/.cache/huggingface ~/Library/Caches/huggingface
du -sh ~/.lmstudio/models ~/.ollama/models
```

---

## 6. Whole-disk map (every region of a Mac)

Scan these. Skip the rest unless Phase 0 says the container is full and
Phases 1–5 didn't explain it.

### 6.1 Always skip

- `/System` (sealed)
- `/usr` except `/usr/local`
- `/bin`, `/sbin`, `/private/var/vm`
- `/System/Volumes/Preboot`, `Update`, `VM`, `xarts`, `iSCPreboot`, `Hardware`
- `/dev`, `/net`, `/home` (autofs)

### 6.2 System-wide, user-relevant

| Path | What | This Mac |
|---|---|---|
| `/Applications` | Installed apps | Xcode 5.0G, Resolve 6.5G, iMovie 3.2G, Office, Docker.app 1.7G, … |
| `/Library/Developer` | Xcode CLT, CoreSimulator runtimes, DeviceKit | CoreSimulator **30G** (Volumes 24G) |
| `/Library/Developer/CoreSimulator/Volumes` | Mounted simulator OS images | iOS 26.2 16G, watchOS 26.2 8.3G |
| `/opt/homebrew` | Apple Silicon brew | **14G** (Cellar 5.3G) |
| `/usr/local` | Intel brew / leftover | (not a hotspot here) |
| `/private/var/folders` | per-user temp | usually small; check if desperate |
| `/private/var/log` | system logs | usually small |

Mounted **disk images** also show up in `df` (Kiro CLI, simulator
volumes, Metal toolchain cryptex). They are not extra used space on the
internal SSD beyond the backing file already counted.

### 6.3 Home top-level (this Mac, `du -sh ~/* ~/.[^.]*`)

| Path | Size | Notes |
|---|---|---|
| `~/Library` | **252G** | See 6.4 |
| `~/work_cip` | **148G** | Main work; includes Claude worktrees |
| `~/.grok` | **48G** | 42G worktrees + 4G sessions |
| `~/.codex` | **27G** | 22G sessions + sqlite |
| `~/.npm` | **20G** | 14G `_cacache` |
| `~/.cache` | **18G** | uv, huggingface, codex-runtimes, puppeteer |
| `~/.lmstudio` | **11G** | 8.3G models |
| `~/Pictures` | 9.9G | keep |
| `~/work_mumzworld` | 5.4G | |
| `~/.local` | 5.1G | uv, pipx, bin |
| `~/work` | 4.5G | |
| `~/.rustup` | 3.7G | 4 toolchains |
| `~/.bun` | 3.3G | install cache 3.0G |
| `~/.ulpi` | 3.2G | memory + logs |
| `~/Documents` | 3.0G | |
| `~/.nvm` | 2.4G | 6 unused Node versions |
| `~/.claude` | 1.7G | projects 1.3G |
| `~/.cargo` | 1.7G | registry 1.6G |
| `~/go` | 1.6G | pkg 1.5G |
| `~/Downloads` | 1.4G | |
| `~/.gradle` | 1.2G | |
| rest | <1G each | |

### 6.4 `~/Library` (the other half of the disk)

| Path | Size | Notes |
|---|---|---|
| `Containers` | **95G** | Docker 85G, CoreDevice 5.9G, Slack 1.3G |
| `Caches` | **78G** | **kache/store 48G**, Kensi 4.1G, Google 3.6G, Playwright 2.8G, Codex 2.8G |
| `Application Support` | **32G** | Claude Desktop **12G** (`vm_bundles` 10G), wallpaper 4.6G, Willow 2.9G |
| `Developer` | **25G** | Xcode DeviceSupport 15G, CoreSimulator devices 10G |
| `Preferences` | **11G** | not one fat file; `du dir/*` can hit ARG_MAX. Many `SnippetStoreMergeTests.*.plist` (~1.8M). Use `du -d 1`. |
| `pnpm` | **9.3G** | global store |
| `Group Containers` | 1.4G | VoicePrompt 872M, Office |
| `Logs` | 522M | |

### 6.5 Work roots (this Mac)

`~/work_cip` 148G, largest projects:

| Project | Size | Why it's big |
|---|---|---|
| VoicePrompt | **44G** | KensiApp **31G of leftover Claude worktrees** |
| holly_grail | 17G | `hgDB/target` 13.9G (Rust) |
| ulpi | 16G | multiple copies (`ulpi-v4`, `ulpi-v4 copy`, `ulpi-v4-bk-1`) + `.next` |
| ulpi-v6 | 12G | |
| secrets | 11G | node_modules 1.7G + Claude worktrees 7.5G |
| cc_vs_oc | 8.6G | `ulpi/target` 6.3G |
| browse, plugin-marketplace, ngrok-portless, albert, duncan, … | 2–5G | mostly node_modules + `.next` |

---

## 7. Known-path catalog

The CLI should ship a table of paths (see `catalog/macos-hotspots.yaml`).
For each: glob or absolute path, category, how to detect "unused",
reclaim command, risk.

Always test existence. Never fail the scan if a path is missing.

Homebrew Intel vs Apple Silicon: check both `/opt/homebrew` and
`/usr/local/Homebrew`.

---

## 8. Language / build artifacts (`node_modules` equivalents)

These are **rebuildable**. Deleting them does not delete source. Next
build/install recreates them.

### 8.1 Table (what we scanned)

| Ecosystem | Dir names | Global caches | This Mac (project-local) |
|---|---|---|---|
| Node / JS | `node_modules`, `.pnpm-store` | `~/.npm`, `~/Library/pnpm`, `~/.bun`, Yarn cache | **18.7G** in 508 `node_modules` |
| Next.js | `.next` | — | **13.1G** in 36 dirs |
| Turbo / Vite / Parcel | `.turbo`, `.parcel-cache` | — | 18.7M `.turbo` |
| JS build output | `dist`, `build`, `out`, `.nuxt`, `.output` | — | dist 1.9G, build 919M |
| PHP / Composer | `vendor` (next to `composer.json`) | `~/.composer`, `~/Library/Caches/composer` | **1.1G** in 24 `vendor` (some false positives) |
| Rust | `target` (next to `Cargo.toml`) | `~/.cargo/registry`, `~/.rustup` | **61.0G** in 11 `target` dirs |
| Python | **discover via `pyvenv.cfg`**, not the folder name. Also `__pycache__`, `.tox` | uv cache 5.5G, pip 803M, huggingface 3.5G, pipx 1.6G | **32 venvs / 4.5G** (most are uv cache + pipx, not project `.venv`) |
| CocoaPods | `Pods` | `~/Library/Caches/CocoaPods` 234M | 178M one tree |
| SwiftPM | `.build`, `~/Library/Caches/org.swift.swiftpm` | 1.9G + kensi-spm 1.8G | |
| Gradle / Android | `.gradle`, `app/build` | `~/.gradle` 1.2G | |
| Go | `vendor` (Go modules), `~/go/pkg` | `~/go/pkg` 1.5G, go-build 71M | |
| Xcode | `DerivedData`, `*.xcarchive`, `build/` | DerivedData only 340M here | ios `build/` 611M duncan |
| Ruby | `vendor/bundle`, `.bundle`, `Gemfile` | `~/.gem` 31M, Homebrew ruby 123M | **no `vendor/bundle` on this Mac**; 4 Gemfiles, gems tiny |
| Dart/Flutter | `.dart_tool`, `build/` | `~/.pub-cache` | absent |

**Headline:** on this machine, **Rust `target/` (61G) beat `node_modules`
(19G)**. A JS-only cleaner would have missed the biggest artifact class.
Grok leftover worktrees each carried a 5–7 GB `target/`.

### 8.2 Also treat as artifacts

- `.next` — reclaimable; `next build` / `next dev` recreates
- Xcode `iOS DeviceSupport/<old iOS>` — reclaimable if that iOS is gone
  from the phone (here: two 4.5G iOS 18.x folders while current is 26.4)
- Playwright / Puppeteer browsers: `~/Library/Caches/ms-playwright` 2.8G,
  `~/.cache/puppeteer` 2.2G
- HuggingFace / LM Studio / Ollama model blobs

### 8.3 Python — how to search (do not rely on `.venv` as a name)

The first pass only `du`'d known cache dirs and folders named `.venv` /
`venv` / `env`. That **under-counted**. Venvs are often named `headroom-ai`,
`aider-chat`, `.browser-use-env`, or uv `archive-v0/<hash>`.

**Correct discovery:**

```bash
# 1. Interpreters and version managers
which python3 pip3 uv poetry pipenv conda pyenv pipx
python3 -V
ls /opt/homebrew/Cellar | grep python
du -sh /opt/homebrew/Cellar/python@*

# 2. Global caches / tool envs (macOS + XDG)
du -sh ~/.cache/uv ~/.local/share/uv ~/.local/pipx \
  ~/Library/Caches/pip ~/.cache/pip ~/Library/Python \
  ~/.pyenv ~/.conda ~/miniconda3 ~/anaconda3 \
  ~/.virtualenvs ~/.local/share/virtualenvs \
  ~/Library/Caches/pypoetry ~/.cache/huggingface \
  ~/.browser-use-env ~/.cache/chroma ~/.cache/torch
pipx list
uv tool list

# 3. Marker walk (prune node_modules/target/.next/.git)
# Record the directory that CONTAINS pyvenv.cfg — that IS the venv.
# Also record: site-packages, __pycache__, .tox, .nox,
# poetry.lock, Pipfile, requirements.txt, conda-meta
```

**This Mac after the marker walk (32 `pyvenv.cfg`):**

| Env | Size | Kind |
|---|---|---|
| `~/.local/pipx/venvs/headroom-ai` | **1.6G** | pipx tool (headroom-ai) |
| `~/.local/share/uv/tools/aider-chat` | 693M | uv tool |
| `~/.browser-use-env` | 374M | ad-hoc venv on PATH |
| `~/.cache/uv/archive-v0/*` (many) | **~1.7G** | uv unpacked wheel cache (already inside the 5.5G uv cache) |
| `~/.local/share/uv/tools/mistral-vibe` | 98M | uv tool |
| `work_cip/davinci-resolve-mcp/.venv` | 71M | project venv |
| `work_cip/holly_grail/hgDB/sdks/python/.venv` | 64M | project venv |
| **All 32 venvs** | **4.5G** | of which ~3.3G is also counted in uv/pipx paths |

**Python totals on this Mac (do not sum — uv cache overlaps archive venvs):**

| Bucket | Size | Risk |
|---|---|---|
| `~/.cache/uv` | **5.5G** | safe-cache (`uv cache clean`) |
| `~/.cache/huggingface` | **3.5G** | ask (model blobs) |
| pipx `headroom-ai` | **1.6G** | ask (installed tool) |
| `~/Library/Caches/pip` | 803M | safe-cache (`pip3 cache purge`) |
| uv tools (aider + vibe) | 790M | ask |
| `~/.browser-use-env` | 374M | ask (on PATH) |
| Homebrew `python@3.11` + `3.13` | 72M+82M | unused-runtime if 3.14 is enough |
| `~/Library/Python/3.14` | 32M | user site-packages |
| project `.venv` (2) | ~135M | rebuildable |
| conda / pyenv / poetry / pipenv | **absent** | |
| `__pycache__` | ~7M | noise |

No conda, no pyenv, no poetry.lock, one Pipfile (turso research, no env).
Python disk here is **caches and tool venvs**, not dozens of project
`.venv`s. The CLI must still walk `pyvenv.cfg` because that is how you
find pipx/uv/ad-hoc envs.

**False positives:** a folder named `env` without `pyvenv.cfg` (we had 8,
17 KB). `site-packages` inside a venv is already counted in the venv size.

### 8.4 Ruby — how to search

```bash
# 1. Interpreters / version managers
which ruby gem bundle rbenv rvm asdf chruby pod
ruby -v
gem env                 # GEM PATHS, user dir
bundle env              # Bundler + RVM/rbenv/chruby installed?
du -sh ~/.rbenv ~/.rvm ~/.asdf ~/.chruby ~/.gem ~/.bundle
du -sh /opt/homebrew/Cellar/ruby /opt/homebrew/lib/ruby/gems
du -sh /Library/Ruby ~/Library/Caches/CocoaPods ~/.cocoapods

# 2. Marker walk
# Gemfile / Gemfile.lock → check sibling vendor/bundle and .bundle
# Podfile → sibling Pods/
# directory named vendor/bundle (Bundler's --path vendor/bundle)
```

**This Mac:**

| Item | Size | Notes |
|---|---|---|
| Ruby itself | system 2.6.10 (macOS) + Homebrew ruby **123M** | no rbenv/rvm/asdf/chruby |
| User gems `~/.gem` | **31M** (`ruby/2.6.0` 5.5M) | almost empty |
| System gems `/Library/Ruby/Gems/2.6.0` | 1.8M | |
| Homebrew gem dir | 0B | |
| `~/.bundle` (home) | 21M | bundler config/cache, not project gems |
| **`vendor/bundle`** | **0** | no Bundler-installed project gems |
| Gemfiles found | 4 | TelePrompter (iOS/.bundle 4K), chatwoot (Rails `vendor` 66M — mixed, not `vendor/bundle`), turso RN example, a uv-cache aider website |
| CocoaPods `Pods/` | 184M TelePrompter + 234M cache | the real Ruby-adjacent disk |
| Podfiles | 10 | 8 are GRDB.swift *test fixtures* inside SwiftPM checkouts — do not treat as apps |

Ruby is **not a disk problem on this machine**. The CLI still has to look:
many Rails apps hide gigabytes in `vendor/bundle` and rbenv in `~/.rbenv/versions`.
CocoaPods is Ruby; count `Pods/` + `~/Library/Caches/CocoaPods` with the
Ruby/iOS section, not as `node_modules`.

Chatwoot `vendor/` (66M) is a Rails tree (assets/gems mix) — confirm with
`Gemfile` in the parent, but do not assume it is Composer.

---

## 9. Worktrees (this was a top-3 reclaim bucket)

Three different products leave full working copies around.

### 9.1 Git worktrees

```
<repo>/.git/worktrees/<name>/
git worktree list
```

Linked checkouts. Source is shared (objects in main `.git`). **Build
artifacts are not shared** — each worktree can have its own
`node_modules` / `target`.

Also look for **prunable** entries (gitdir points at a deleted path).

### 9.2 Claude Code worktrees

```
<repo>/.claude/worktrees/<name>/
```

On this Mac: **22 repos** have a `.claude/worktrees` directory.

Biggest:

- `~/work_cip/VoicePrompt/KensiApp/.claude/worktrees` — **31G**
  (38 git worktrees listed)
- `~/work_cip/secrets/.claude/worktrees` — **7.5G** (24 agent-* trees)

These are leftover agent checkouts. If the session is done they are
safe to `git worktree remove` + delete the folder.

### 9.3 Grok worktrees

```
~/.grok/worktrees/<project>/subagent-<id>/
```

Here:

- `cc-vs-oc-ulpi` — **41G**, mostly 7 subagent trees each 5.5–7G of
  **Rust `target/`**
- `socialreply-socialreply` — 1.6G, many ~57M trees

These look like isolation worktrees that were never garbage-collected.

### 9.4 Ulpi worktrees

```
<repo>/.ulpi/worktrees/
```

Seen under `tunl` and `skills-engineering-autonomous`.

### 9.5 How the CLI should discover them

1. Known roots: `~/.grok/worktrees`, `~/.claude/worktrees` (rare),
   `~/.codex/worktrees` if present.
2. During the work-root walk, if you see `.claude/worktrees`,
   `.ulpi/worktrees`, or `.git/worktrees`, record the parent repo and
   `du -sh` the folder (do not list every agent id in the summary).
3. For each git repo found (`.git` dir or file), run
   `git worktree list --porcelain`. Flag: count > 1, or `prunable`.
4. Age: `mtime` of the worktree dir. Anything idle > N days is
   "likely leftover".

---

## 10. Emulators and simulators

### 10.1 Apple

| Kind | Where | This Mac |
|---|---|---|
| Xcode.app | `/Applications/Xcode.app` | 5.0G (keep if developing) |
| Simulator runtimes (system) | `/Library/Developer/CoreSimulator/Volumes` | iOS 26.2 **16G**, watchOS 26.2 **8.3G** |
| Simulator devices (user) | `~/Library/Developer/CoreSimulator/Devices` | **10G** |
| DeviceSupport (symbols for plugged-in devices) | `~/Library/Developer/Xcode/iOS DeviceSupport` | **15G**: 18.4, 18.5, 26.4 |
| DerivedData | `~/Library/Developer/Xcode/DerivedData` | 340M (unusually small) |
| Archives | `~/Library/Developer/Xcode/Archives` | empty / missing |
| CoreDevice | `~/Library/Containers/com.apple.CoreDevice.CoreDeviceService` | 5.9G |

Unused on this Mac:

- **watchOS 26.2 runtime + 5 never-booted watches** (~8–10G)
- DeviceSupport **iPhone16,1 18.4** and **18.5** (4.5G each) if the
  current phone is iPhone17,2 on 26.4

```bash
xcrun simctl delete <udid>                 # unused device
xcrun simctl runtime delete <identifier>   # unused runtime
rm -rf ~/Library/Developer/Xcode/iOS\ DeviceSupport/<old>
```

### 10.2 Android — first pass was wrong

The first pass only looked at `~/Library/Android` (missing) and
`~/.android` (8 KB of adb keys). **The SDK lives in Homebrew:**

`/opt/homebrew/share/android-commandlinetools` — **5.6 GB**

**How to search (CLI must do all of this):**

```bash
echo "ANDROID_HOME=$ANDROID_HOME ANDROID_SDK_ROOT=$ANDROID_SDK_ROOT ANDROID_AVD_HOME=$ANDROID_AVD_HOME"
which adb emulator avdmanager sdkmanager
# Homebrew casks do NOT put the SDK in ~/Library/Android/sdk
du -sh ~/Library/Android/sdk \
  /opt/homebrew/share/android-commandlinetools \
  /opt/homebrew/Caskroom/android-commandlinetools \
  /opt/homebrew/Caskroom/android-platform-tools \
  ~/.android ~/.android/avd \
  ~/Library/Application\ Support/Google/AndroidStudio* \
  /Applications/Android\ Studio*.app

SDK="${ANDROID_SDK_ROOT:-${ANDROID_HOME:-/opt/homebrew/share/android-commandlinetools}}"
sdkmanager --sdk_root="$SDK" --list_installed
avdmanager list avd
"$SDK/emulator/emulator" -list-avds   # often not on PATH
du -sh "$SDK"/system-images/*/*/*     # one folder per API/tag/ABI
```

Also probe Genymotion / BlueStacks / Nox if present (none here).

**This Mac:**

| Item | Size | Notes |
|---|---|---|
| Homebrew SDK root | **5.6G** | `/opt/homebrew/share/android-commandlinetools` |
| `system-images/android-35/google_apis/arm64-v8a` | **3.8G** | API 35 Google APIs image. **No AVD uses it.** |
| `emulator` package 36.4.10 | **1.1G** | binary exists; `which emulator` fails (not on PATH) |
| build-tools 34.0.0 + 35.0.0 | 374M | |
| platforms/android-35 | 130M | |
| cmdline-tools | 166M | |
| platform-tools (adb) cask | 38M | `/opt/homebrew/Caskroom/android-platform-tools` |
| `~/.android` | 8K | adbkey only — **no `avd/` directory** |
| AVDs | **0** | `avdmanager list avd` empty; `emulator -list-avds` empty |
| Android Studio.app | absent | Google Library dirs are **Chrome**, not Studio |
| Genymotion / BlueStacks | absent | |
| project `android/` trees | 13M browse-android, 4M TelePrompter | not emulators |

So: the **emulator runtime + one system image are installed**, but **no
virtual device was ever created**. That image is unused-runtime. Reclaim:

```bash
sdkmanager --sdk_root=/opt/homebrew/share/android-commandlinetools --uninstall \
  'system-images;android-35;google_apis;arm64-v8a' \
  emulator
# or: brew uninstall --cask android-commandlinetools
```

Uninstalling the cask would also drop cmdline-tools / platforms /
build-tools (~1.7G extra). Keep those if you still build APKs with
Gradle; the unused part is **system-images 3.8G + emulator 1.1G ≈ 4.9G**.

**CLI lesson:** `~/Library/Android/sdk` is the Android Studio default.
Homebrew `android-commandlinetools` installs to
`/opt/homebrew/share/android-commandlinetools`. `ANDROID_HOME` may be
unset even when the SDK exists. Always size that Homebrew path, and
treat `system-images` with **zero AVDs** as unused.

### 10.3 Other

- Docker: §5.8 (the big one here)
- Colima present but tiny (32K); cache 317M
- No OrbStack / Podman data

---

## 11. Package-manager caches and extra toolchains

Rebuildable, usually **safe-cache**.

| Bucket | Size | Reclaim |
|---|---|---|
| `~/.npm/_cacache` | 14G (npm total 20G) | `npm cache clean --force` |
| `~/Library/pnpm` | 9.3G | `pnpm store prune` |
| `~/.cache/uv` | 5.5G | `uv cache clean` |
| Homebrew prefix | 14G (Cellar 5.3G) | `brew autoremove`; `brew cleanup -s` |
| `~/.cache/huggingface` | 3.5G | delete unused model blobs |
| `~/.rustup` 4 toolchains | 3.7G | keep `stable`; drop `nightly`, `1.94.0`, `1.96.1` if unused |
| `~/.bun/install/cache` | 3.0G | `bun pm cache rm` |
| `~/.nvm` 6 versions, none is current `v25.8.1` | 2.4G | `nvm uninstall` each |
| `~/.cargo/registry` | 1.6G | `cargo cache -a` (needs cargo-cache) or delete registry |
| `~/go/pkg` | 1.5G | `go clean -modcache` |
| `~/.gradle` | 1.2G | `rm -rf ~/.gradle/caches` |
| pip | 803M | `pip3 cache purge` |
| CocoaPods cache | 234M | `pod cache clean --all` |

**LM Studio models** 8.3G and **Ollama** 610M are "ask" — they are
downloads, not caches, but unused models are reclaimable.

---

## 12. Containers / VMs

| Item | Apparent (`ls`) | Allocated (`du`) | Notes |
|---|---|---|---|
| Docker.raw | **288G** | **85G** | Sparse APFS file. Engine was not running, so no `docker system df`. |
| iOS simulator volume | 16G mounted | counted under `/Library/Developer/CoreSimulator` | |
| watchOS simulator volume | 8.3G | same | |
| Kiro CLI disk images | 1.7G × 2 mounted | backing files already in Applications / downloads | |

CLI rule: for files named `*.raw`, `*.qcow2`, `Docker.raw`, `*.dmg`,
report **both** apparent and allocated. Highlight sparse.

---

## 13. Apps, caches, Application Support

Not language artifacts, but they dominate `~/Library`.

Hot on this Mac:

| Path | Size | Class |
|---|---|---|
| `~/Library/Caches/kache/store` | **48G** | App cache (Kensi/kache). Highest single cache. Probe before delete. |
| `~/Library/Application Support/Claude` | 12G | **`vm_bundles` 10G**, Cache 771M, local-agent-mode-sessions 658M, claude-code-vm 236M. Ask. |
| `~/Library/Caches/com.kensi.mac` | 4.1G | App cache |
| `~/Library/Application Support/com.apple.wallpaper` | 4.6G | System, probably leave |
| `~/Library/Caches/Google` | 3.6G | Chrome etc. |
| `~/Library/Caches/ms-playwright` | 2.8G | safe-cache |
| `~/Library/Caches/com.openai.codex` | 2.8G | ask (may be session-related) |
| `~/.codex/sessions` | 22G | Session logs/history. Ask; huge. |
| `~/.grok/sessions` | 4.0G | Same |
| `~/Library/Caches/org.swift.swiftpm` + `kensi-spm` | 3.7G | SwiftPM cache |
| Playwright/Puppeteer/camoufox | ~5.7G combined | browser runtimes, safe-cache if unused |

`~/Library/Preferences` at **11G** is abnormal (prefs are usually tens of
MB). There is no single file >50M except Photoshop settings (~25M).
`du -sh ~/Library/Preferences/*` can fail with ARG_MAX. Many
`SnippetStoreMergeTests.*.plist` files sit at ~1.8M each. The CLI must
use `du -d 1`, never glob expansion, and must not ignore this directory
just because the name is "Preferences".

---

## 14. Classification

Every finding gets a risk:

| Risk | Meaning | Default CLI action |
|---|---|---|
| `safe-cache` | Recreated on next use (npm cache, uv, pip, brew cache, Playwright browsers, DerivedData) | Offer "clean" |
| `rebuildable` | Recreated by install/build (`node_modules`, `target`, `vendor`, `.next`, `Pods`) | Offer per-project or "all idle projects" |
| `leftover-worktree` | Agent/git worktree whose session is gone | Offer remove after listing branch + age |
| `unused-runtime` | Simulator/AVD/toolchain never used or superseded | Offer delete after showing lastBooted / current version |
| `ask` | Big, maybe still wanted (Docker VM, LM Studio models, Codex sessions, Claude app support, kache store) | Report only |
| `keep` | Source, Photos, current Xcode, current DeviceSupport, active app | Hide from "reclaim" unless `--all` |
| `never` | `/System`, keychains, Mail index unless user insists | Don't list as reclaimable |

---

## 15. This machine — reclaimable picture (2026-09-06)

Disk: **~141 GB free of 926 GB** (Data ~85% used). Not an emergency, but
a lot of easy wins.

### Highest-confidence reclaim (rebuildable / leftover / unused)

These do not delete source or current toolchains:

| Bucket | ~Size | Why |
|---|---|---|
| Grok leftover subagent worktrees (`cc-vs-oc-ulpi`) | **41G** | 7 copies of Rust `target/` |
| Claude worktrees in KensiApp | **31G** | 38 leftover agent checkouts |
| Rust `target/` in holly_grail + cc_vs_oc (main trees) | **20G** | `cargo clean` |
| npm cache | **14–20G** | `_cacache` |
| `.next` folders | **13G** | 36 Next.js caches, including backup copies |
| Claude worktrees in secrets | **7.5G** | |
| pnpm store | **9.3G** | prune unused |
| watchOS runtime + never-booted devices | **~8–10G** | never booted |
| Android system-image API 35 + emulator binary (no AVDs) | **~4.9G** | Homebrew SDK; unused-runtime |
| iOS DeviceSupport 18.4 + 18.5 | **9G** | phone is on 26.4 |
| uv cache | **5.5G** | |
| huggingface cache | **3.5G** | |
| bun cache | **3.0G** | |
| Playwright + Puppeteer | **5.0G** | |
| extra rustup toolchains (nightly 1.3G + 1.96.1 1.1G + 1.94.0 501M) | **~2.9G** | keep stable (797M) |
| extra nvm versions (all 6) | **2.4G** | current node is Homebrew 25 |
| Codex-runtimes in `~/.cache` | **3.5G** | |
| node_modules in **idle** projects (not all 18.7G) | variable | don't wipe active trees blindly |

**Rough high-confidence total: 150–180 GB** if leftover worktrees,
caches, unused simulators, and old DeviceSupport go away.

### Ask before touching

| Bucket | ~Size |
|---|---|
| Docker.raw allocated | 85G (sparse 288G) |
| `~/Library/Caches/kache/store` | 48G |
| `~/.codex/sessions` + sqlite | 22G+ |
| Claude Desktop `vm_bundles` | 10G of the 12G |
| LM Studio models | 8.3G |
| ulpi copies (`ulpi-v4` 2.5G, `copy` 2.3G, `bk-1` 1.7G) | plus hooks 3.0G |
| `~/Library/Preferences` | 11G, likely thousands of medium plists |

Android **emulators/AVDs:** none created. Homebrew SDK + unused API 35
system image + emulator binary: **~5.6G** (of which ~4.9G is unused-runtime).
Trash: empty.

---

## 16. Reclaim playbook (commands, not executed in this audit)

Dry-run / inspect first. The CLI should print these, not run them unless
`delete <id>` after the user named that id. Never automatic.

```bash
# Grok leftover worktrees
du -sh ~/.grok/worktrees/*
# then delete project folders whose sessions are done
# rm -rf ~/.grok/worktrees/cc-vs-oc-ulpi

# Claude leftover worktrees (per repo)
git -C ~/work_cip/VoicePrompt/KensiApp worktree list
# git worktree remove <path> --force
# rm -rf ~/work_cip/VoicePrompt/KensiApp/.claude/worktrees

# Rust build artifacts
cargo clean --manifest-path ~/work_cip/holly_grail/hgDB/Cargo.toml
cargo clean --manifest-path ~/work_cip/cc_vs_oc/ulpi/Cargo.toml

# JS caches
npm cache clean --force
pnpm store prune
bun pm cache rm
# optional: rm -rf project/.next   (rebuilds on next dev/build)

# Python
uv cache clean
pip3 cache purge

# Xcode / simulators
xcrun simctl delete unavailable
xcrun simctl runtime delete com.apple.CoreSimulator.SimRuntime.watchOS-26-2
rm -rf ~/Library/Developer/Xcode/iOS\ DeviceSupport/iPhone16,1\ 18.4*
rm -rf ~/Library/Developer/Xcode/iOS\ DeviceSupport/iPhone16,1\ 18.5*

# Toolchains
nvm uninstall 18.20.8 20.19.4 22.15.0 22.15.1 22.17.0 23.11.0
rustup toolchain uninstall nightly 1.94.0 1.96.1

# Docker (engine must be running; this shrinks Docker.raw only after compact)
docker system df
docker system prune -a --volumes
# Docker Desktop → Settings → Resources → "Reclaim disk space" / compact

# Homebrew
brew cleanup -s
brew autoremove
```

Do **not** auto-run `rm -rf ~/Library/Caches/kache` until we know whether
Kensi still needs that store (48G). Same for `~/.codex/sessions`.

---

## 17. CLI architecture (from this audit)

Suggested commands:

```text
freedisk scan              # Phases 0–8, JSON + human report
freedisk scan --quick      # Phase 0 + 1 only (known paths)
freedisk why               # top 20 reclaimable, ranked
freedisk worktrees         # git + claude + grok + ulpi
freedisk artifacts         # node_modules/target/vendor/.next grouped
freedisk simulators        # simctl + android
freedisk caches            # package managers + ~/Library/Caches
freedisk delete <id>       # ONLY mutate path; explicit id; confirm; never auto
```

Implementation notes:

1. **Parallelize by tree, not by file.** `du` of `~/Library/Containers`
   and `~/work_cip` can run at the same time; 6+ overlapping `du`s of the
   same tree just thrash the SSD (we did this by accident; home `du`
   took ~345s).
2. **Timeouts:** any single `du` of a known-huge path (Docker.raw parent,
   `~/Library/Caches`, `~/work_cip`) should be its own job.
3. **Allocated bytes only** (`st_blocks * 512`). Show apparent size as
   extra for sparse VM files.
4. **Don't follow symlinks. Don't cross into `/System`. Don't require
   sudo.** Skip `EPERM` / TCC paths and list them as "unreadable".
5. **Work roots are configurable:** default
   `~/work*`, `~/src`, `~/Projects`, `~/dev`, `~/code`,
   `~/.grok/worktrees`, Desktop, Documents, Downloads.
6. **Report format:** one finding = `{id, path, bytes, category, risk,
   last_used, reclaim: {cmd, dry_run}}`.
7. **Ground truth for "how full":** `diskutil apfs list` container
   Capacity In Use / Not Allocated, not `df /`.
8. **macOS only first.** Linux would be a different catalog (apt, systemd,
   `/var/lib/docker`).

---

## 18. Performance and correctness lessons

1. `du -sh ~` / `du -sh ~/Library/*` is the slowest useful command. Prefer
   a fixed catalog + depth-1 of home (skipping `Library`) + dedicated
   jobs for Library subtrees.
2. Pruned name-walk of work roots was **fast** (~14s) and found 1,163
   artifacts. Sizing those was ~80s. This is the right approach for
   `node_modules` / `target`.
3. `find ~ -name node_modules` without `-prune` is a trap.
4. `vendor` needs a `composer.json` check. We recorded
   `resources/views/vendor` (Laravel) as a false positive.
5. `env` without `pyvenv.cfg` is almost always a false positive.
6. Simulator **runtimes** (system Volumes) and simulator **devices**
   (user Library) are different buckets. Deleting devices does not
   delete the 16G iOS runtime.
7. Docker must be reported even when the daemon is down. Use `du` on
   `Docker.raw`.
8. Agent worktrees (Grok, Claude) duplicate **build artifacts**, not just
   git objects. `git worktree list` understates size until you `du`.
9. Project **copies** (`ulpi-v4 copy`, `ulpi-v4-bk-1`) each have their
   own `.next` and `dist`. Detect directories with ` copy`, `-bk-`,
   `.bak`, `-backup` in the name.
10. Current toolchain may not live where you think: Node 25 is Homebrew,
    nvm still has six older versions.
11. `du -sh dir/*` explodes on directories with tens of thousands of
    children (`~/Library/Preferences`). Use `du -d 1 dir`, not glob.

---

## 19. False positives / be careful

- `dist/` that is committed (browser extensions, some CLIs) — check git.
- `build/` inside iOS projects may be a local Xcode build (rebuildable)
  or a folder the user cares about.
- `~/Pictures` 9.9G — Photos library, keep.
- `~/Library/Application Support/com.apple.wallpaper` — system.
- OS update snapshots (`com.apple.os.update-*`) — not TM junk.
- `/Applications/Xcode.app` 5G — keep if any iOS work continues.
- `~/Library/Preferences` — do not wipe; drill first.
- kache 48G — looks like a cache (`Caches/kache/store`) but it is the
  largest single folder on the machine; confirm with the Kensi app
  before treating as `safe-cache`.

---

## 20. Open probes vs what the late drills closed

Closed after the first report:

- **Claude Desktop 12G:** `vm_bundles` **10G**, Cache 771M, local-agent-mode-sessions 658M, claude-code-vm 236M.
- **rustup per toolchain:** nightly 1.3G, 1.96.1 1.1G, stable 797M, 1.94.0 501M.
- **ulpi copies:** `ulpi-v4` 2.5G, `ulpi-v4 copy` 2.3G, `ulpi-v4-bk-1` 1.7G, plus hooks 3.0G.
- **Codex sessions:** `~/.codex/sessions/2026` **22G** (6 top-level entries); 2025 is 25M.
- **Grok sessions:** 4.0G across **172** session dirs.
- **Preferences 11G:** not one fat file. `du -sh ~/Library/Preferences/*` is unsafe (ARG_MAX). Largest named children are Photoshop settings (~25M) and many `SnippetStoreMergeTests.*.plist` (~1.8M). Still worth a counted `find | wc` in the CLI.
- **kache:** 48G is almost entirely `store/`; the rest is module caches and a 192M `index.db`.
- **phpactor** cache: `~/.cache/phpactor` **2.3G** (missed in the first “high-confidence” table).

Still open (CLI should auto-drill at ≥1 GB):

- `du -d 1 ~/Library/Caches/kache/store`
- `docker system df` + compact once the engine is up
- Age of each Grok/Claude worktree (`stat -f %Sm`)
- `brew autoremove --dry-run`
- CoreDevice container 5.9G
- Count of `SnippetStoreMergeTests.*.plist` in Preferences

---

## 21. Files produced by this audit

| File | What |
|---|---|
| `NOTES.md` | This document |
| `catalog/macos-hotspots.yaml` | Known paths + categories for the CLI |
| `.scan-artifacts.txt` | 1,163 artifact paths (kind, path) |
| `.scan-artifacts-sized.tsv` | Same with allocated bytes |

The `.scan-*` files are a snapshot of this machine, not part of the
tool's source. The YAML catalog is.
