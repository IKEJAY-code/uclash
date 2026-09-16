#!/usr/bin/env node
// Downloads the uclash binary for the current platform.
// Invoked automatically as an npm postinstall hook; can also be run directly.
//
// Overrides:
//   UCLASH_HOST        gitee | github             (default: gitee)
//   UCLASH_REPO        owner/repo                 (default: IKEJAY-code/uclash)
//   UCLASH_VERSION     release tag or "latest"
//   UCLASH_GH_MIRROR   GitHub acceleration prefix (UCLASH_HOST=github only)
//   UCLASH_ASSET_URL   full URL to the binary     (skips host resolution)
//   UCLASH_LOCAL_FILE  install from a local file
//   UCLASH_FORCE=1     re-download even if the binary exists
"use strict";

const fs = require("fs");
const os = require("os");
const path = require("path");

const HOST = process.env.UCLASH_HOST || "gitee";
const REPO = process.env.UCLASH_REPO || "IKEJAY-code/uclash";
const VERSION = process.env.UCLASH_VERSION || "latest";
const MIRROR = process.env.UCLASH_GH_MIRROR || "";

function platform() {
  const p = os.platform();
  if (p !== "linux" && p !== "darwin") {
    throw new Error(`unsupported platform: ${p} (uclash targets linux/darwin)`);
  }
  const arch = os.arch() === "x64" ? "amd64" : os.arch() === "arm64" ? "arm64" : null;
  if (!arch) throw new Error(`unsupported architecture: ${os.arch()}`);
  return { p, arch };
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

async function resolveURL(asset) {
  if (process.env.UCLASH_ASSET_URL) return process.env.UCLASH_ASSET_URL;
  if (HOST === "github") {
    const base =
      VERSION === "latest"
        ? `https://github.com/${REPO}/releases/latest/download/${asset}`
        : `https://github.com/${REPO}/releases/download/${VERSION}/${asset}`;
    return MIRROR ? `${MIRROR.replace(/\/+$/, "")}/${base}` : base;
  }
  if (HOST !== "gitee") {
    throw new Error(`unknown UCLASH_HOST=${HOST} (want gitee or github)`);
  }
  if (VERSION !== "latest") {
    return `https://gitee.com/${REPO}/releases/download/${VERSION}/${asset}`;
  }
  const json = await request(`https://gitee.com/api/v5/repos/${REPO}/releases/latest`);
  const release = JSON.parse(json);
  const found = (release.assets || []).find((a) => a.name === asset);
  if (!found) throw new Error(`asset ${asset} not found in the latest Gitee release`);
  return found.browser_download_url;
}

(async () => {
  const { p, arch } = platform();
  const asset = `uclash-${p}-${arch}`;
  const binDir = path.join(__dirname, "bin");
  fs.mkdirSync(binDir, { recursive: true });
  const dest = path.join(binDir, "uclash");
  if (fs.existsSync(dest) && !process.env.UCLASH_FORCE) {
    console.log(`uclash: ${dest} already present (UCLASH_FORCE=1 to re-download)`);
    return;
  }
  if (process.env.UCLASH_LOCAL_FILE) {
    fs.copyFileSync(process.env.UCLASH_LOCAL_FILE, dest);
    fs.chmodSync(dest, 0o755);
    console.log(`uclash: installed from ${process.env.UCLASH_LOCAL_FILE} to ${dest}`);
    return;
  }
  const url = await resolveURL(asset);
  console.log(`uclash: downloading ${url}`);
  await request(url, { sink: dest });
  fs.chmodSync(dest, 0o755);
  console.log(`uclash: installed to ${dest}`);
  console.log("next: uclash init");
})().catch((err) => {
  console.error(`uclash install failed: ${err.message}`);
  console.error("hints: UCLASH_VERSION=v0.1.0 | UCLASH_HOST=github | UCLASH_LOCAL_FILE=/path/to/" + "uclash-<os>-<arch>");
  process.exit(1);
});
