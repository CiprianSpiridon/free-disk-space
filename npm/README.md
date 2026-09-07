# free-disk-space

macOS disk-audit CLI. **Scan never deletes.**

The npm package is `free-disk-space` (same as the GitHub repo). The
command is `freedisk`.

This package is a thin wrapper. It does not compile Go. `postinstall`
downloads `freedisk-darwin-arm64` or `freedisk-darwin-amd64` from
[GitHub Releases](https://github.com/CiprianSpiridon/free-disk-space/releases).

```bash
npm i -g free-disk-space
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
npm uninstall -g free-disk-space
```

MIT. Source: [CiprianSpiridon/free-disk-space](https://github.com/CiprianSpiridon/free-disk-space).
