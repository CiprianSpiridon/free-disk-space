# Agent notes for freedisk

CLI for reporting macOS disk consumers. **Scan never deletes.**

```text
go run ./cmd/freedisk help
go run ./cmd/freedisk scan --json
go run ./cmd/freedisk scan --quick --json
go run ./cmd/freedisk scan --dev --json
go run ./cmd/freedisk history
go run ./cmd/freedisk history show latest
```

Scan duration on a full developer Mac: `--quick` 1-5 min (up to ~10),
`--dev` 3-10 min, full 5-15 min. Do not kill while stderr still prints
`freedisk: phase` / `still walking`. Prefer `--quick` first.

## Agents MUST update the catalog on this Mac

You are allowed and expected to change the per-user overlay after a scan.
The bundled catalog is generic; this computer is not. Overlay is not delete.
Never edit catalog/macos-hotspots.yaml.

```text
freedisk catalog path
freedisk catalog add PATH --scans quick,dev --risk ask --category user
freedisk catalog unassign PATH --from quick
freedisk catalog disable PATH
freedisk scans disable artifacts
freedisk catalog list --json
```

Add when the user names a path, or home depth-1 / Ask-first showed a large
dir that should be sized every run. Unassign/disable when they do not use
that path. Re-scan after changes.

`freedisk skill install` writes the agent skill into every local CLI (Claude, Codex, Cursor, Grok, …).

`freedisk delete <id> [--yes]` is the only mutate path. **Never automatic.**
Agents must not run delete unless the human listed those finding ids.
There is no `delete --all`. Non-TTY requires `--yes`.
