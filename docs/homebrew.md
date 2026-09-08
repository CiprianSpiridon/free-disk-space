# Homebrew tap for freedisk

freedisk is a macOS-only CLI. It is **not** in homebrew-core. Install from a
personal tap. Current stable formula: **0.2.0**.

The formula in this repo (`Formula/freedisk.rb`) is the template. The tap is a
**separate** GitHub repo that copies that file. This project does not ship
bottles; Homebrew builds from source with Go.

## Install (users)

```text
brew tap CiprianSpiridon/freedisk
brew install freedisk
```

HEAD (latest main):

```text
brew install --HEAD CiprianSpiridon/freedisk/freedisk
```

One-liner (taps implicitly):

```text
brew install CiprianSpiridon/freedisk/freedisk
```

Upgrade:

```text
brew update && brew upgrade freedisk
```

macOS only. Linux Homebrew will refuse the formula (`depends_on :macos`).
Scan never deletes; only `freedisk delete <id>` removes files.

## Create the tap (operators)

1. Create an **empty public** GitHub repo named
   `CiprianSpiridon/homebrew-freedisk`. Homebrew maps that to tap
   `CiprianSpiridon/freedisk`. Example:

   ```text
   gh repo create CiprianSpiridon/homebrew-freedisk --public \
     --description "Homebrew tap for freedisk"
   ```

2. Copy this repo's formula into the tap as `Formula/freedisk.rb`:

   ```text
   git clone git@github.com:CiprianSpiridon/homebrew-freedisk.git
   cd homebrew-freedisk
   mkdir -p Formula
   cp /path/to/free-disk-space/Formula/freedisk.rb Formula/freedisk.rb
   git add Formula/freedisk.rb
   git commit -m "Add freedisk formula (HEAD-only until v0.1.0)"
   git push origin HEAD
   ```

3. First publish path (no tag yet):

   ```text
   brew install --HEAD CiprianSpiridon/freedisk/freedisk
   ```

   That clones `https://github.com/CiprianSpiridon/free-disk-space.git` branch
   `main` and builds `./cmd/freedisk`.

## After tagging v0.1.0

Do this in **free-disk-space**, not the tap:

```text
git tag v0.1.0
git push origin v0.1.0
```

Pushing `v*` runs `.github/workflows/release.yml`, which attaches
`freedisk-darwin-arm64`, `freedisk-darwin-amd64`, and `checksums.txt` to a
GitHub Release. Those checksums are for direct binary downloads. The Homebrew
formula uses the **source tarball**, not those binaries.

Fill the formula `url` / `sha256` (in **both** this repo's template and the
tap copy):

```text
curl -sL "https://github.com/CiprianSpiridon/free-disk-space/archive/refs/tags/v0.1.0.tar.gz" | shasum -a 256
```

Then in `Formula/freedisk.rb`:

- Uncomment `url` and `sha256` and paste the digest.
- Remove the `livecheck` skip (GitHub tags can be livechecked after that).
- Leave `head` in place so `brew install --HEAD` still works.

Commit the tap formula update and push. Users can then
`brew install CiprianSpiridon/freedisk/freedisk` without `--HEAD`.

## Smoke-test the formula locally

From a **free-disk-space** checkout:

```text
ruby -c Formula/freedisk.rb
```

If Homebrew is installed:

```text
brew style ./Formula/freedisk.rb
```

After the tap exists (audit expects a formula name, not a path):

```text
brew audit --strict --formula CiprianSpiridon/freedisk/freedisk
```

HEAD-only formulas often warn about a missing stable URL until v0.1.0. That is
expected.

`brew install --build-from-source ./Formula/freedisk.rb` only works **after**
`url` and `sha256` are filled (Homebrew needs a stable URL). Until then:

```text
brew install --HEAD ./Formula/freedisk.rb
```

`--HEAD` clones the GitHub `head` URL (`main`), not your dirty working tree.
To exercise unpublished commits, push them to `main` first, or wait for the
stable tarball path above.

Confirm:

```text
freedisk version
freedisk help
```

`help` should mention that scan never deletes. `version` should match the
formula version (`HEAD` on a HEAD install; `0.2.0` on the current stable URL).

## Release checklist

- [ ] `go test ./...` passes
- [ ] `make build` produces `bin/freedisk` with Version from git describe or 0.2.0
- [ ] `ruby -c Formula/freedisk.rb` passes
- [ ] Tag `v0.2.0` and `git push --tags` (GitHub Release + darwin binaries)
- [ ] `shasum -a 256` of the **source** tarball (not the darwin binaries)
- [ ] Uncomment `url` / `sha256` in this repo's `Formula/freedisk.rb`
- [ ] Copy the same change into `CiprianSpiridon/homebrew-freedisk`
- [ ] `brew update && brew install CiprianSpiridon/freedisk/freedisk`
- [ ] `brew style ./Formula/freedisk.rb`
- [ ] After the tap exists: `brew audit --strict --formula CiprianSpiridon/freedisk/freedisk`
