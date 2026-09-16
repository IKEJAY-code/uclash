#!/usr/bin/env node
// Downloads the uclash binary for the current platform.
// Invoked automatically as an npm postinstall hook; can also be run directly.
//
// The binary version defaults to this package's own version, so a pinned
// install never silently tracks a moving target. Downloads are verified
// against the SHA-256 table below, with GitHub mirror fallback.
//
// Overrides:
//   UCLASH_REPO        owner/repo                  (default: IKEJAY-code/uclash)
//   UCLASH_VERSION     release tag (default: v<package version>)
//   UCLASH_SHA256      override the expected checksum (required for
//                      versions not in the table below)
//   UCLASH_GH_MIRROR   space-separated mirror prefixes, tried before defaults
//   UCLASH_ASSET_URL   full URL of the binary      (skips host resolution)
//   UCLASH_LOCAL_FILE  install from a local file
//   UCLASH_FORCE=1     re-download even if the binary exists
"use strict";

const crypto = require("crypto");
const fs = require("fs");
const os = require("os");
const path = require("path");

const REPO = process.env.UCLASH_REPO || "IKEJAY-code/uclash";
const DEFAULT_MIRRORS = ["https://gh-proxy.com", "https://ghfast.top", "https://ghproxy.net"];
const MIRRORS = (process.env.UCLASH_GH_MIRROR ? process.env.UCLASH_GH_MIRROR.split(/\s+/) : [])
  .filter(Boolean)
  .concat(DEFAULT_MIRRORS);

// SHA-256 of the official release binaries (built by the release workflow),
// keyed by version and platform.
const SHA256 = {
  "v0.1.0": {
    "linux-amd64": "4fd1ac6d567534508a6fa3f13127102793675d601bd20d9faffb94d0da742c01",
    "linux-arm64": "0f19d9928865eff9c950f64bc674c03aca17a1b7697b172a4cafa1a09bc44f31",
    "darwin-amd64": "07283de7f8b12446b3bfc782e772ef9f33be87c4f1ac60b2a6391bba6033375e",
    "darwin-arm64": "aaeb327c2153b7441894a77838d5488e40b1680f22efb4891c36458441825b3b",
  },
  "v0.1.1": {
    "linux-amd64": "0f195aed53734c89c995c0d0c91cbb31c6cd2820c4707010838c346f1ed7c0ff",
    "linux-arm64": "fdf7ce26d8ccd85749d7f3d614588e62875d3108983b9ffa64b4faaed7e87c7b",
    "darwin-amd64": "6ba3cbeed9bff84b65995e7c5f2bf0b795a18152ee9b69d789aca6e6a9ccb71a",
    "darwin-arm64": "eff2e49468749416ddbe723d3249c61252eadad30c28a209474ea0c6a658f448",
  },
};

function packageVersion() {
  return require("./package.json").version;
}

function platform() {
  const p = os.platform();
  if (p !== "linux" && p !== "darwin") {
    throw new Error(`unsupported platform: ${p} (uclash targets linux/darwin)`);
  }
  const arch = os.arch() === "x64" ? "amd64" : os.arch() === "arm64" ? "arm64" : null;
  if (!arch) throw new Error(`unsupported architecture: ${os.arch()}`);
  return { p, arch, key: `${p}-${arch}` };
}

function request(url, { sink } = {}) {
  return new Promise((resolve, reject) => {
    const lib = url.startsWith("https:") ? require("https") : require("http");
    lib
      .get(url, { headers: { "User-Agent": "uclash-npm" } }, (res) => {
        if (res.statusCode >= 300 && res.statusCode < 400 && res.headers.location) {
          res.resume();
          const next = new URL(res.headers.location, url).toString();
          return request(next, { sink }).then(resolve, reject);
        }
        if (res.statusCode !== 200) {
          res.resume();
          return reject(new Error(`HTTP ${res.statusCode} for ${url}`));
        }
        if (!sink) {
          let body = "";
          res.setEncoding("utf8");
          res.on("data", (c) => {
            body += c;
            if (body.length > 4 << 20) {
              res.destroy();
              reject(new Error("response too large"));
            }
          });
          res.on("end", () => resolve(body));
          return;
        }
        const tmp = sink + ".tmp";
        const out = fs.createWriteStream(tmp);
        res.pipe(out);
        out.on("finish", () => {
          out.close(() => {
            fs.renameSync(tmp, sink);
            resolve("");
          });
        });
        out.on("error", reject);
      })
      .on("error", reject);
  });
}

function isGitHubURL(url) {
  return /^https:\/\/(github\.com|raw\.githubusercontent\.com|api\.github\.com)\//.test(url);
}

// download tries the URL directly, then through mirror prefixes for GitHub URLs.
async function download(url, dest) {
  try {
    return await request(url, { sink: dest });
  } catch (err) {
    if (!isGitHubURL(url)) throw err;
    for (const mirror of MIRRORS) {
      const proxied = `${mirror.replace(/\/+$/, "")}/${url}`;
      try {
        console.log(`uclash: retrying via ${mirror}`);
        return await request(proxied, { sink: dest });
      } catch (_) {
        // try the next mirror
      }
    }
    throw err;
  }
}

async function resolveURL(asset, version) {
  if (process.env.UCLASH_ASSET_URL) return process.env.UCLASH_ASSET_URL;
  return `https://github.com/${REPO}/releases/download/${version}/${asset}`;
}

function sha256(file) {
  return crypto.createHash("sha256").update(fs.readFileSync(file)).digest("hex");
}

(async () => {
  const { key } = platform();
  const asset = `uclash-${key}`;
  const version = process.env.UCLASH_VERSION || `v${packageVersion()}`;
  const expected = (process.env.UCLASH_SHA256 || SHA256[version]?.[key] || "").toLowerCase();
  if (!expected) {
    throw new Error(
      `no pinned SHA-256 for ${version}/${key}; set UCLASH_SHA256=<hex> to install explicitly`
    );
  }

  const binDir = path.join(__dirname, "bin");
  fs.mkdirSync(binDir, { recursive: true });
  const dest = path.join(binDir, "uclash-bin");
  if (fs.existsSync(dest) && !process.env.UCLASH_FORCE) {
    if (sha256(dest) === expected) {
      console.log(`uclash: ${dest} already present`);
      return;
    }
  }
  if (process.env.UCLASH_LOCAL_FILE) {
    fs.copyFileSync(process.env.UCLASH_LOCAL_FILE, dest);
    fs.chmodSync(dest, 0o755);
  } else {
    const url = await resolveURL(asset, version);
    console.log(`uclash: downloading ${url}`);
    await download(url, dest);
    fs.chmodSync(dest, 0o755);
  }
  const actual = sha256(dest);
  if (actual !== expected) {
    fs.rmSync(dest, { force: true });
    throw new Error(`SHA-256 mismatch for ${version}/${key}\n  expected: ${expected}\n  actual:   ${actual}`);
  }
  console.log(`uclash: installed ${version} (${key}) to ${dest}`);
  console.log("next: uclash init");
})().catch((err) => {
  console.error(`uclash install failed: ${err.message}`);
  console.error(
    "hints: UCLASH_VERSION=<tag> UCLASH_SHA256=<hex> | UCLASH_GH_MIRROR=<prefix> | " +
      "UCLASH_LOCAL_FILE=/path/to/uclash-<os>-<arch>"
  );
  process.exit(1);
});
