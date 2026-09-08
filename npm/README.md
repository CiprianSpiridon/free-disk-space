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

If the GitHub Release is missing:

```bash
go install github.com/CiprianSpiridon/free-disk-space/cmd/freedisk@latest
```

Scan never deletes. Only `freedisk delete <id>`.

```bash
npm uninstall -g fspace
```

MIT. [CiprianSpiridon/free-disk-space](https://github.com/CiprianSpiridon/free-disk-space).
