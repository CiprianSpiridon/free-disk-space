#!/usr/bin/env node
"use strict";

// Downloads the darwin binary from GitHub Releases into vendor/freedisk.
// Assets match the release workflow: freedisk-darwin-arm64, freedisk-darwin-amd64,
// checksums.txt (SHA256SUMS is a fallback name). Not a Go rebuild.

const crypto = require("node:crypto");
const fs = require("node:fs");
const path = require("node:path");

const pkg = require("../package.json");

const REPO = "CiprianSpiridon/free-disk-space";
const GO_INSTALL =
  "go install github.com/CiprianSpiridon/free-disk-space/cmd/freedisk@latest";
const NO_RELEASE_MESSAGE = `No GitHub release for this version. Use: ${GO_INSTALL}`;
const CHECKSUM_FILES = ["checksums.txt", "SHA256SUMS"];
const MACHO_MAGICS = new Set([
  0xfeedface, 0xfeedfacf, 0xcefaedfe, 0xcffaedfe, 0xcafebabe, 0xbebafeca,
]);

function defaultReleaseBase(version) {
  return `https://github.com/${REPO}/releases/download/v${version}`;
}

function assetName(arch) {
  if (arch === "arm64") return "freedisk-darwin-arm64";
  if (arch === "x64") return "freedisk-darwin-amd64";
  return null;
}

function isMachO(buf) {
  if (!buf || buf.length < 4) return false;
  return MACHO_MAGICS.has(buf.readUInt32BE(0));
}

function parseChecksums(text) {
  const out = new Map();
  if (!text) return out;
  for (const line of text.split(/\r?\n/)) {
    const trimmed = line.trim();
    if (!trimmed || trimmed.startsWith("#")) continue;
    const m = trimmed.match(/^([0-9a-fA-F]{64})\s+\*?(\S+)\s*$/);
    if (!m) continue;
    out.set(path.posix.basename(m[2]), m[1].toLowerCase());
  }
  return out;
}

function sha256(buf) {
  return crypto.createHash("sha256").update(buf).digest("hex");
}

async function httpGet(url, fetchImpl, asText) {
  if (typeof fetchImpl !== "function") {
    throw new Error("freedisk-cli requires Node.js 18+ (global fetch).");
  }
  const res = await fetchImpl(url, {
    redirect: "follow",
    headers: {
      "User-Agent": `freedisk-cli-npm/${pkg.version}`,
      Accept: asText ? "text/plain" : "application/octet-stream",
    },
    signal: AbortSignal.timeout(120000),
  });
  return res;
}

async function downloadChecksums(releaseBase, fetchImpl) {
  for (const name of CHECKSUM_FILES) {
    const url = `${releaseBase.replace(/\/$/, "")}/${name}`;
    const res = await httpGet(url, fetchImpl, true);
    if (res.status === 404) continue;
    if (!res.ok) {
      throw new Error(`failed to fetch ${name} (HTTP ${res.status}): ${url}`);
    }
    const text = await res.text();
    return { name, map: parseChecksums(text) };
  }
  return null;
}

async function install(opts = {}) {
  const platform = opts.platform ?? process.platform;
  const arch = opts.arch ?? process.arch;
  const version = opts.version ?? pkg.version;
  const destDir = opts.destDir ?? path.join(__dirname, "..", "vendor");
  const fetchImpl = opts.fetch ?? globalThis.fetch;
  const log = opts.log ?? ((msg) => console.error(msg));
  const releaseBase = (
    opts.releaseBase ??
    process.env.FREEDISK_RELEASE_BASE ??
    defaultReleaseBase(version)
  ).replace(/\/$/, "");

  if (platform !== "darwin") {
    throw new Error(`macOS only (refusing to install on ${platform}).`);
  }

  const asset = assetName(arch);
  if (!asset) {
    throw new Error(`unsupported architecture ${arch} (need arm64 or x64).`);
  }

  const url = `${releaseBase}/${asset}`;
  log(`freedisk-cli: downloading ${asset} (v${version})`);

  let res;
  try {
    res = await httpGet(url, fetchImpl, false);
  } catch (err) {
    throw new Error(`${err.message}\n${NO_RELEASE_MESSAGE}`);
  }

  if (res.status === 404) {
    throw new Error(NO_RELEASE_MESSAGE);
  }
  if (!res.ok) {
    throw new Error(`download failed HTTP ${res.status}: ${url}\n${NO_RELEASE_MESSAGE}`);
  }

  const buf = Buffer.from(await res.arrayBuffer());
  if (!isMachO(buf)) {
    throw new Error(
      `downloaded file is not a macOS binary (unexpected format).\n${NO_RELEASE_MESSAGE}`,
    );
  }

  const sums = await downloadChecksums(releaseBase, fetchImpl);
  if (sums) {
    const expected = sums.map.get(asset);
    if (!expected) {
      throw new Error(`${sums.name} has no entry for ${asset}`);
    }
    const got = sha256(buf);
    if (got !== expected) {
      throw new Error(
        `checksum mismatch for ${asset}: got ${got}, expected ${expected}`,
      );
    }
    log(`freedisk-cli: ${sums.name} ok`);
  } else {
    log("freedisk-cli: no checksums.txt/SHA256SUMS on the release; skipping digest check");
  }

  fs.mkdirSync(destDir, { recursive: true });
  const dest = path.join(destDir, "freedisk");
  const tmp = path.join(destDir, `.freedisk.${process.pid}.tmp`);
  try {
    fs.writeFileSync(tmp, buf);
    fs.chmodSync(tmp, 0o755);
    fs.renameSync(tmp, dest);
  } catch (err) {
    try {
      fs.unlinkSync(tmp);
    } catch {
      // ignore cleanup
    }
    throw err;
  }
  log(`freedisk-cli: installed ${dest}`);
  return dest;
}

async function main() {
  try {
    await install();
  } catch (err) {
    console.error(`freedisk-cli: ${err.message}`);
    process.exit(1);
  }
}

if (require.main === module) {
  main();
}

module.exports = {
  GO_INSTALL,
  NO_RELEASE_MESSAGE,
  assetName,
  defaultReleaseBase,
  install,
  isMachO,
  parseChecksums,
  sha256,
};
