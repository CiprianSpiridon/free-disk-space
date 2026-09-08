# fdsk

macOS disk-audit CLI. **Scan never deletes.**

npm package: **`fdsk`**. Command: **`freedisk`**.

Thin wrapper: `postinstall` downloads `freedisk-darwin-arm64` or
`freedisk-darwin-amd64` from
[GitHub Releases](https://github.com/CiprianSpiridon/free-disk-space/releases).

```bash
npx fdsk version
npx fdsk scan --quick --json

npm i -g fdsk
freedisk version
```

macOS + Node 18+. Not `npx freedisk` (different package).

If the GitHub Release is missing:

```bash
go install github.com/CiprianSpiridon/free-disk-space/cmd/freedisk@latest
```

Scan never deletes. Only `freedisk delete <id>`. No `delete --all`.

```bash
npm uninstall -g fdsk
```

MIT. [CiprianSpiridon/free-disk-space](https://github.com/CiprianSpiridon/free-disk-space).
