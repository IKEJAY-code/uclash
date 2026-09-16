#!/usr/bin/env node
// Entry point shipped inside the package. The real binary is downloaded by
// install.js (postinstall) into bin/uclash-bin; this shim forwards to it so
// npm can link the command before postinstall runs.
"use strict";

const { spawnSync } = require("child_process");
const path = require("path");

const binary = path.join(__dirname, "bin", "uclash-bin");
const result = spawnSync(binary, process.argv.slice(2), { stdio: "inherit" });

if (result.error) {
  if (result.error.code === "ENOENT") {
    console.error("uclash: the platform binary is missing; reinstall the package:");
    console.error("  npm i -g uclash-cli --force");
  } else {
    console.error(`uclash: failed to start the binary: ${result.error.message}`);
  }
  process.exit(1);
}
process.exit(result.status === null ? 1 : result.status);
