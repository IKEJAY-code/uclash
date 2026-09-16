#!/usr/bin/env bash
# Copies built artifacts from the Windows project directory into ~/uclash-test
# and runs the integration test. Invoke from Windows with:
#
#   wsl -d Ubuntu-24.04 bash /mnt/c/Users/IkeJay/Projects/clash/scripts/wsl-run.sh
#
set -euo pipefail

SRC=$(cd "$(dirname "$0")/.." && pwd)
DST=$HOME/uclash-test

rm -rf "$DST"
mkdir -p "$DST/bin" "$DST/fixtures/ui" "$DST/fixtures/geodata"
cp "$SRC/dist/uclash-linux-amd64" "$DST/bin/uclash"
cp "$SRC/.dev/mihomo-linux-amd64" "$DST/fixtures/mihomo"
cp -r "$SRC/.dev/ui/." "$DST/fixtures/ui/"
cp "$SRC"/testdata/fixture-* "$DST/fixtures/"
cp -r "$SRC/.dev/geodata/." "$DST/fixtures/geodata/"
cp "$SRC/scripts/wsl-test.sh" "$DST/"
chmod +x "$DST/bin/uclash" "$DST/fixtures/mihomo" "$DST/wsl-test.sh"

# Optional: bundle a user-space Node so the npm installer path is exercised.
NODE_BIN=""
if ls "$SRC"/.dev/node-v*-linux-x64.tar.xz >/dev/null 2>&1; then
  rm -rf "$DST/node"
  mkdir -p "$DST/node"
  tar -xf "$SRC"/.dev/node-v*-linux-x64.tar.xz -C "$DST/node" --strip-components=1
  NODE_BIN="$DST/node/bin"
fi

exec env \
  PATH="${NODE_BIN:+$NODE_BIN:}$PATH" \
  BIN="$DST/bin/uclash" \
  CORE="$DST/fixtures/mihomo" \
  UI="$DST/fixtures/ui" \
  FIXTURE="$DST/fixtures/fixture-sub.yaml" \
  GEODATA="$DST/fixtures/geodata" \
  NPM_DIR="$SRC/npm" \
  bash "$DST/wsl-test.sh"
