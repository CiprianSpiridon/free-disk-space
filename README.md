# FreeDiskSpace

Repo: [github.com/CiprianSpiridon/free-disk-space](https://github.com/CiprianSpiridon/free-disk-space)

macOS disk-audit CLI that **agents** (Claude Code, Codex, Grok, Cursor, …)
can run themselves.

The bundled catalog is the **starting point**: known caches, SDKs, tmp,
Desktop/Downloads, generic work roots, and auto-discovered project
folders. `catalog add` is optional customization, not required for a
useful scan.

```bash
go run ./cmd/freedisk help
go run ./cmd/freedisk scan --quick --json
```

Scan never deletes. `freedisk delete <id>` is opt-in with explicit ids.

| File | Who it's for |
|---|---|
| **[RECIPE.md](RECIPE.md)** | **The spec.** Scan order, invariants, discovery markers, report shape. The CLI encodes this. |
| [catalog/macos-hotspots.yaml](catalog/macos-hotspots.yaml) | Paths, artifact names, APIs, thresholds the scanner loads |
| [findings.schema.json](findings.schema.json) | JSON contract for scan output |
| [research/mole-patterns.md](research/mole-patterns.md) | What to steal from [tw93/Mole](https://github.com/tw93/mole) (design, not code) |
| [NOTES.md](NOTES.md) | Research diary + snapshot of one Mac (2026-09-06). Not the runtime spec. |
| [skills/freedisk/](skills/freedisk/) | Agent skill (`SKILL.md`) for [skills.sh](https://skills.sh) indexing. `freedisk skill install` copies it into local CLIs. |

## Agent: run an audit now

```bash
go run ./cmd/freedisk scan --quick --json
```

`--quick` is volume + known catalog paths (named children, not parent
blobs) + home/Library depth-1 + **tmp children** (including
`/private/tmp/kensi-*` and other fat `/tmp` dirs). Markdown tables include
**id, full path, last used, and a reclaim command**. Progress goes to
stderr. Scan never deletes. `freedisk delete <id>` is the only mutate
path: explicit ids, confirmation, never `--all`.
