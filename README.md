# FreeDiskSpace

Repo: [github.com/CiprianSpiridon/free-disk-space](https://github.com/CiprianSpiridon/free-disk-space)

Research for a macOS disk-audit CLI that **agents** (Claude Code, Codex,
Grok, Cursor, …) can run themselves.

The bundled catalog is the **starting point**: known caches, SDKs, tmp,
Desktop/Downloads, generic work roots, and auto-discovered project
folders. `catalog add` is optional customization, not required for a
useful scan.

Nothing is deleted in this repo. The CLI is not built yet.

| File | Who it's for |
|---|---|
| **[RECIPE.md](RECIPE.md)** | **The spec.** Scan order, invariants, discovery markers, report shape. Follow this to audit a Mac today, or to implement the CLI. |
| [catalog/macos-hotspots.yaml](catalog/macos-hotspots.yaml) | Paths, artifact names, APIs, thresholds the scanner loads |
| [findings.schema.json](findings.schema.json) | JSON contract for scan output |
| [research/mole-patterns.md](research/mole-patterns.md) | What to steal from [tw93/Mole](https://github.com/tw93/mole) (design, not code) |
| [NOTES.md](NOTES.md) | Research diary + snapshot of one Mac (2026-09-06). Not the runtime spec. |

## Agent: run an audit now

1. Read `RECIPE.md` and the YAML catalog.
2. Execute phases 0–8. Do not skip Python / Ruby / Android because a
   default folder was empty.
3. Emit findings JSON (schema) + the human report in RECIPE §11.
4. Do not delete. Print reclaim commands only. **No delete is ever
   automatic** — not for caches, not for worktrees, not for unused
   runtimes.

## Later: CLI

`freedisk scan` encodes RECIPE.md and never mutates the disk.
`freedisk delete <id>` is the only delete path: explicit ids the human
named, with confirmation, never `--all`, never from an agent unless the
human asked for those ids.
