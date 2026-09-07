# Agent notes for freedisk

CLI for reporting macOS disk consumers. **Scan never deletes.**

```text
go run ./cmd/freedisk help
go run ./cmd/freedisk scan --json
go run ./cmd/freedisk scan --quick --json
go run ./cmd/freedisk scan --dev --json
```

Scan duration on a full developer Mac: `--quick` 1-5 min (up to ~10),
`--dev` 3-10 min, full 5-15 min. Do not kill while stderr still prints
`freedisk: phase` / `still walking`. Prefer `--quick` first.

No overlay is required. Optional:

```text
freedisk catalog add PATH --scans quick,dev
freedisk catalog unassign PATH --from quick
freedisk scans disable artifacts
```

`freedisk skill install` writes the agent skill into every local CLI (Claude, Codex, Cursor, Grok, …).

`freedisk delete <id> [--yes]` is the only mutate path. **Never automatic.**
Agents must not run delete unless the human listed those finding ids.
There is no `delete --all`. Non-TTY requires `--yes`.
