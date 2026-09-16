#!/usr/bin/env bash
# Local dev checks: shell syntax + offline smoke tests of install.sh
# (local-file mode, file:// downloads, SHA-256 verification paths).
#   wsl -d Ubuntu-24.04 bash /mnt/c/Users/IkeJay/Projects/clash/scripts/smoke-install.sh
set -euo pipefail

SRC=$(cd "$(dirname "$0")/.." && pwd)
PASS=0
FAIL=0
ok()  { PASS=$((PASS + 1)); printf '  ok   %s\n' "$1"; }
bad() { FAIL=$((FAIL + 1)); printf '  FAIL %s\n' "$1"; }
check()  { local desc=$1; shift; if "$@" >/dev/null 2>&1; then ok "$desc"; else bad "$desc"; fi; }
ncheck() { local desc=$1; shift; if "$@" >/dev/null 2>&1; then bad "$desc"; else ok "$desc"; fi; }

echo "== syntax =="
sh -n "$SRC/scripts/install.sh" && ok "scripts/install.sh (sh -n)"
for f in wsl-test.sh wsl-run.sh release.sh smoke-install.sh; do
    bash -n "$SRC/scripts/$f" && ok "scripts/$f"
done

BIN=${BIN:-$HOME/uclash-test/bin/uclash}
if [ ! -x "$BIN" ]; then
    echo "skip behaviour tests: $BIN not found (run scripts/wsl-run.sh first)"
    printf 'passed: %d, failed: %d\n' "$PASS" "$FAIL"
    [ "$FAIL" -eq 0 ]
    exit
fi

TMP=$(mktemp -d /tmp/uclash-install-test.XXXXXX)
trap 'rm -rf "$TMP"' EXIT

FIXTURE_SHA=$(sha256sum "$BIN" | awk '{print $1}')
mkdir -p "$TMP/release"
cp "$BIN" "$TMP/release/uclash-linux-amd64"
printf '%s  uclash-linux-amd64\n' "$FIXTURE_SHA" > "$TMP/release/SHA256SUMS"

echo
echo "== local-file install =="
UCLASH_BIN_DIR="$TMP/bin-local" UCLASH_LOCAL_FILE="$BIN" \
    sh "$SRC/scripts/install.sh" >/dev/null 2>&1
check "binary installed" test -x "$TMP/bin-local/uclash"

echo
echo "== file:// download with SHA256SUMS verification =="
url="file://$TMP/release/uclash-linux-amd64"
out=$(UCLASH_BIN_DIR="$TMP/bin-verify" UCLASH_DOWNLOAD_URL="$url" \
    sh "$SRC/scripts/install.sh" 2>&1)
grep -Fq 'SHA-256 verified' <<< "$out" && ok "checksum verified (auto SHA256SUMS)" || bad "checksum verified (auto SHA256SUMS)"
check "binary installed" test -x "$TMP/bin-verify/uclash"

echo
echo "== explicit UCLASH_SHA256 =="
out=$(UCLASH_BIN_DIR="$TMP/bin-explicit" UCLASH_DOWNLOAD_URL="$url" UCLASH_SHA256="$FIXTURE_SHA" \
    sh "$SRC/scripts/install.sh" 2>&1)
grep -Fq 'SHA-256 verified' <<< "$out" && ok "checksum verified (explicit hash)" || bad "checksum verified (explicit hash)"

echo
echo "== bad explicit hash is refused =="
ncheck "install fails on mismatch" env UCLASH_BIN_DIR="$TMP/bin-badhash" \
    UCLASH_DOWNLOAD_URL="$url" \
    UCLASH_SHA256=0000000000000000000000000000000000000000000000000000000000000000 \
    sh "$SRC/scripts/install.sh"
check "no binary left behind" test ! -e "$TMP/bin-badhash/uclash"

echo
echo "== tampered SHA256SUMS is refused =="
mkdir -p "$TMP/tampered"
cp "$BIN" "$TMP/tampered/uclash-linux-amd64"
printf '%064d  uclash-linux-amd64\n' 0 > "$TMP/tampered/SHA256SUMS"
ncheck "install fails on tampered sums" env UCLASH_BIN_DIR="$TMP/bin-tampered" \
    UCLASH_DOWNLOAD_URL="file://$TMP/tampered/uclash-linux-amd64" \
    sh "$SRC/scripts/install.sh"
check "no binary left behind" test ! -e "$TMP/bin-tampered/uclash"

echo
echo "== missing SHA256SUMS warns but installs =="
mkdir -p "$TMP/nosums"
cp "$BIN" "$TMP/nosums/uclash-linux-amd64"
out=$(UCLASH_BIN_DIR="$TMP/bin-nosums" UCLASH_DOWNLOAD_URL="file://$TMP/nosums/uclash-linux-amd64" \
    sh "$SRC/scripts/install.sh" 2>&1)
grep -Fq 'skipping checksum verification' <<< "$out" && ok "warns when no SHA256SUMS" || bad "warns when no SHA256SUMS"
check "binary still installed" test -x "$TMP/bin-nosums/uclash"

printf '\npassed: %d, failed: %d\n' "$PASS" "$FAIL"
[ "$FAIL" -eq 0 ]
