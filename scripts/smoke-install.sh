#!/usr/bin/env bash
# Local dev checks: shell syntax + a no-network smoke test of install.sh.
#   wsl -d Ubuntu-24.04 bash /mnt/c/Users/IkeJay/Projects/clash/scripts/smoke-install.sh
set -euo pipefail

SRC=$(cd "$(dirname "$0")/.." && pwd)

sh -n "$SRC/scripts/install.sh"
echo "syntax ok: scripts/install.sh (sh -n)"
for f in wsl-test.sh wsl-run.sh release.sh smoke-install.sh; do
  bash -n "$SRC/scripts/$f"
  echo "syntax ok: scripts/$f"
done

BIN=${BIN:-$HOME/uclash-test/bin/uclash}
if [ -x "$BIN" ]; then
  TMP=$(mktemp -d)
  trap 'rm -rf "$TMP"' EXIT
  UCLASH_BIN_DIR="$TMP/bin" UCLASH_LOCAL_FILE="$BIN" sh "$SRC/scripts/install.sh" >"$TMP/install.log"
  "$TMP/bin/uclash" version
  echo "install.sh smoke test ok (local-file mode)"
else
  echo "skip install.sh smoke test: $BIN not found (run scripts/wsl-run.sh first)"
fi
