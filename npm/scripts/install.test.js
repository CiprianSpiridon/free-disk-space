"use strict";

const { test } = require("node:test");
const assert = require("node:assert/strict");
const crypto = require("node:crypto");
const fs = require("node:fs");
const os = require("node:os");
const path = require("node:path");

const {
  NO_RELEASE_MESSAGE,
  assetName,
  defaultReleaseBase,
  install,
  isMachO,
  parseChecksums,
  sha256,
} = require("./install.js");

function machoStub() {
  const buf = Buffer.alloc(32, 1);
  buf.writeUInt32BE(0xcffaedfe, 0);
  return buf;
}

function fakeFetch(routes) {
  return async (url) => {
    const key = String(url);
    const hit = routes[key];
    if (!hit) {
      return {
        ok: false,
        status: 404,
        text: async () => "",
        arrayBuffer: async () => new ArrayBuffer(0),
      };
    }
    return {
      ok: hit.status >= 200 && hit.status < 300,
      status: hit.status,
      text: async () => hit.text ?? "",
      arrayBuffer: async () => {
        const body = hit.body ?? Buffer.alloc(0);
        return body.buffer.slice(body.byteOffset, body.byteOffset + body.byteLength);
      },
    };
  };
}

test("assetName maps Node arch to release asset", () => {
  assert.equal(assetName("arm64"), "freedisk-darwin-arm64");
  assert.equal(assetName("x64"), "freedisk-darwin-amd64");
  assert.equal(assetName("ia32"), null);
});

test("defaultReleaseBase uses v-prefixed GitHub tag", () => {
  assert.equal(
    defaultReleaseBase("0.1.0"),
    "https://github.com/CiprianSpiridon/free-disk-space/releases/download/v0.1.0",
  );
});

test("parseChecksums accepts sha256sum and *binary lines", () => {
  const digest = "a".repeat(64);
  const text = [
    "# comment",
    `${digest}  freedisk-darwin-arm64`,
    `${digest.toUpperCase()} *dist/freedisk-darwin-amd64`,
    "",
  ].join("\n");
  const map = parseChecksums(text);
  assert.equal(map.get("freedisk-darwin-arm64"), digest);
  assert.equal(map.get("freedisk-darwin-amd64"), digest);
});

test("isMachO accepts 64-bit little-endian magic", () => {
  assert.equal(isMachO(machoStub()), true);
  assert.equal(isMachO(Buffer.from("<!DOCTYPE html>")), false);
  assert.equal(isMachO(Buffer.alloc(0)), false);
});

test("install refuses non-darwin platforms", async () => {
  await assert.rejects(
    () => install({ platform: "linux", arch: "x64", log: () => {} }),
    /macOS only/,
  );
  await assert.rejects(
    () => install({ platform: "win32", arch: "x64", log: () => {} }),
    /macOS only/,
  );
});

test("install 404 prints go install fallback", async () => {
  const destDir = fs.mkdtempSync(path.join(os.tmpdir(), "freedisk-npm-"));
  await assert.rejects(
    () =>
      install({
        platform: "darwin",
        arch: "arm64",
        destDir,
        releaseBase: "https://example.test/v0.1.0",
        fetch: fakeFetch({}),
        log: () => {},
      }),
    (err) => err.message === NO_RELEASE_MESSAGE,
  );
});

test("install verifies checksums.txt and writes vendor/freedisk", async () => {
  const destDir = fs.mkdtempSync(path.join(os.tmpdir(), "freedisk-npm-"));
  const body = machoStub();
  const digest = sha256(body);
  const base = "https://example.test/v0.1.0";
  const dest = await install({
    platform: "darwin",
    arch: "arm64",
    destDir,
    releaseBase: base,
    log: () => {},
    fetch: fakeFetch({
      [`${base}/freedisk-darwin-arm64`]: { status: 200, body },
      [`${base}/checksums.txt`]: {
        status: 200,
        text: `${digest}  freedisk-darwin-arm64\n`,
      },
    }),
  });
  assert.equal(dest, path.join(destDir, "freedisk"));
  const got = fs.readFileSync(dest);
  assert.equal(sha256(got), digest);
  assert.equal(fs.statSync(dest).mode & 0o111, 0o111);
});

test("install falls back to SHA256SUMS when checksums.txt is missing", async () => {
  const destDir = fs.mkdtempSync(path.join(os.tmpdir(), "freedisk-npm-"));
  const body = machoStub();
  const digest = crypto.createHash("sha256").update(body).digest("hex");
  const base = "https://example.test/v0.1.0";
  await install({
    platform: "darwin",
    arch: "x64",
    destDir,
    releaseBase: base,
    log: () => {},
    fetch: fakeFetch({
      [`${base}/freedisk-darwin-amd64`]: { status: 200, body },
      [`${base}/SHA256SUMS`]: {
        status: 200,
        text: `${digest}  freedisk-darwin-amd64\n`,
      },
    }),
  });
  assert.equal(fs.existsSync(path.join(destDir, "freedisk")), true);
});

test("install fails on checksum mismatch", async () => {
  const destDir = fs.mkdtempSync(path.join(os.tmpdir(), "freedisk-npm-"));
  const body = machoStub();
  const base = "https://example.test/v0.1.0";
  await assert.rejects(
    () =>
      install({
        platform: "darwin",
        arch: "arm64",
        destDir,
        releaseBase: base,
        log: () => {},
        fetch: fakeFetch({
          [`${base}/freedisk-darwin-arm64`]: { status: 200, body },
          [`${base}/checksums.txt`]: {
            status: 200,
            text: `${"b".repeat(64)}  freedisk-darwin-arm64\n`,
          },
        }),
      }),
    /checksum mismatch/,
  );
});
