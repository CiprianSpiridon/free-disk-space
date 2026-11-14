# fspace

macOS disk-audit CLI. **Scan never deletes.**

npm: **`fspace`**. Command: **`freedisk`**.

```bash
npx fspace version
npx fspace scan --quick --json

npm i -g fspace
freedisk version
```

macOS + Node 18+. Not `npx freedisk` (different package).

Scans use five parallel directory workers by default. Tune with
`npx fspace scan --quick --workers=10 --json` (1-64 workers). Full scan:
`npx fspace scan --workers=5 --json`.

Reports include minimum APFS private-byte reclaim estimates, inventory-only
parents, and `catalog_advice` for agents. Agents may add useful recurring caches
to the per-user quick catalog or project/build paths to dev, then re-scan.
The bundled catalog is unchanged. Configuration changes never authorize deletion.

If the GitHub Release is missing:

```bash
go install github.com/CiprianSpiridon/free-disk-space/cmd/freedisk@latest
```

Scan never deletes. Only `freedisk delete <id>`.

```bash
npm uninstall -g fspace
```

MIT. [CiprianSpiridon/free-disk-space](https://github.com/CiprianSpiridon/free-disk-space).
