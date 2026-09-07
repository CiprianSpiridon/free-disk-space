# freedisk-cli

macOS disk-audit CLI. **Scan never deletes.**

The command is `freedisk`. The npm name is `freedisk-cli` because `freedisk`
is already taken on npmjs.com.

This package is a thin wrapper. It does not compile Go. `postinstall`
downloads `freedisk-darwin-arm64` or `freedisk-darwin-amd64` from
[GitHub Releases](https://github.com/CiprianSpiridon/free-disk-space/releases).

```bash
npm i -g freedisk-cli
freedisk version
freedisk scan --quick
```

Requires macOS (Apple Silicon or Intel) and Node.js 18+. Linux and Windows
installs are refused.

If the GitHub Release for this version does not exist yet:

```bash
go install github.com/CiprianSpiridon/free-disk-space/cmd/freedisk@latest
```

Scan never deletes. The only mutate path is `freedisk delete <id>`. There is
no `delete --all`. Non-TTY delete requires `--yes`.

```bash
npm uninstall -g freedisk-cli
```

MIT. Source: [CiprianSpiridon/free-disk-space](https://github.com/CiprianSpiridon/free-disk-space).
