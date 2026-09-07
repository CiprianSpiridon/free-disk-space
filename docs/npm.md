# npm distribution (`free-disk-space`)

Publish root is the **`npm/`** directory so `go test` and the Go module stay
clean. The tarball is a wrapper: a `freedisk` bin shim plus `postinstall`
that downloads the darwin binary. It does not recompile Go. It does not
include `cmd/`, `internal/`, or other `.go` sources.

The package is **not** on npmjs.com until a human publishes it. Do not claim
it is already published.

## Package name

The registry package is **`free-disk-space`** (same as the GitHub repo).
The installed command is **`freedisk`**. The name `freedisk` on npmjs is
an unrelated `df -h` helper; we do not use `freedisk-cli`.

Keep `npm/package.json` `version` in sync with `internal/version.Version`
(currently `0.1.0`).

## Users (after GitHub Release **and** npm publish)

```bash
npm i -g free-disk-space
freedisk version
```

Requires:

- macOS (darwin `arm64` or `x64`)
- Node.js 18+
- GitHub Release binaries for this version

`package.json` sets `"os": ["darwin"]` and `"cpu": ["x64", "arm64"]`.
`postinstall` also refuses `linux` / `win32` with **macOS only**.

Until a `v0.1.0` GitHub Release exists, npm install will 404 and print:

```text
No GitHub release for this version. Use: go install github.com/CiprianSpiridon/free-disk-space/cmd/freedisk@latest
```

That `go install` line is the real path today. There is no npm package on
the registry yet, and there is no GitHub Release tarball yet.

Uninstall:

```bash
npm uninstall -g free-disk-space
```

Scan never deletes. Only `freedisk delete <id>` removes files, and only for
ids a human named. There is no `delete --all`. Non-TTY delete requires
`--yes`.

## What postinstall downloads

From
`https://github.com/CiprianSpiridon/free-disk-space/releases/download/v${version}/`:

| Node `process.arch` | Asset |
| --- | --- |
| `arm64` | `freedisk-darwin-arm64` |
| `x64` | `freedisk-darwin-amd64` |

These names match the release workflow (`freedisk-darwin-arm64` /
`freedisk-darwin-amd64`, plus `checksums.txt`). They are raw binaries, not
`.tar.gz`.

If `checksums.txt` (or `SHA256SUMS`) is on the release, the digest is
verified. The file is chmod `0755` into `vendor/freedisk` (next to this
package, i.e. `node_modules/free-disk-space/vendor/freedisk`).

Optional: `FREEDISK_RELEASE_BASE` overrides the download prefix (local
testing).

## Operators: publish checklist

Do this **after** `v0.1.0` is tagged and the GitHub Action has attached
darwin binaries. Do **not** `npm publish` before those assets exist.

1. `go test ./...`
2. Tag and push: `git tag v0.1.0 && git push origin v0.1.0`
3. Confirm the GitHub Release has:
   - `freedisk-darwin-arm64`
   - `freedisk-darwin-amd64`
   - `checksums.txt`
4. From repo root: `cd npm`
5. `npm pack` and inspect the tarball (must **not** contain Go sources)
6. `npm login` (npmjs.com account with permission to publish `free-disk-space`)
7. `npm publish --access public`
8. Smoke: `npm i -g free-disk-space && freedisk version` on a Mac (Node 18+)

Bump `npm/package.json` `version` whenever `internal/version.Version` bumps,
and publish a matching GitHub Release first.

## How to test locally (no publish)

From `npm/`:

```bash
npm test
npm pack
tar tzf free-disk-space-0.1.0.tgz
```

Expected entries (plus npm’s `package/` prefix):

- `package.json`
- `README.md`
- `LICENSE`
- `bin/freedisk`
- `scripts/install.js`

Must not appear: `cmd/`, `internal/`, `catalog/`, `*.go`, `vendor/freedisk`,
`scripts/install.test.js`.

`npm pack` does not run `postinstall`. A full download test needs the GitHub
Release (or `FREEDISK_RELEASE_BASE` pointing at a local HTTP tree with the
same asset names).

`npm install` inside `npm/` will run `postinstall` and fail with the
`go install` fallback until `v0.1.0` exists. That is expected.

## Layout

| Path | Role |
| --- | --- |
| `npm/package.json` | publish manifest (`free-disk-space` 0.1.0) |
| `npm/bin/freedisk` | shim; execs `vendor/freedisk` |
| `npm/scripts/install.js` | `postinstall` download + checksum |
| `npm/README.md` | registry listing |
| `npm/LICENSE` | MIT copy for the tarball |

Publish **only** from `npm/` (`cd npm && npm publish --access public`).
There is no root `package.json`, so an accidental publish from the repo
root will not pack the Go tree.
