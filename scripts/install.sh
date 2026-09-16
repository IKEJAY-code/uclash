#!/bin/sh
# uclash installer — npm-style, per-user, no sudo.
#
# GitHub (primary):
#   curl -fsSL https://raw.githubusercontent.com/IKEJAY-code/uclash/main/scripts/install.sh | sh
# Behind CN networks (put a mirror in front of the raw URL):
#   curl -fsSL https://gh-proxy.com/https://raw.githubusercontent.com/IKEJAY-code/uclash/main/scripts/install.sh | sh
#
# Download behaviour:
#   * every attempt prints the exact URL it is trying
#   * the direct GitHub attempt is aborted when it stays slower than 100 KB/s
#     for 10 seconds (so a throttled connection falls back to mirrors instead
#     of looking stuck)
#   * UCLASH_GH_MIRROR=<prefix>[ <prefix>...] is tried FIRST
#   * otherwise: GitHub direct, then the built-in mirrors
#   * the downloaded binary is verified against the release SHA256SUMS
#     (or UCLASH_SHA256 when provided)
#
# Environment overrides:
#   UCLASH_REPO          owner/repo                (default: IKEJAY-code/uclash)
#   UCLASH_BIN_DIR       install dir               (default: ~/.local/bin)
#   UCLASH_VERSION       release tag or "latest"   (default: latest)
#   UCLASH_GH_MIRROR     space-separated mirror prefixes, tried first
#   UCLASH_SHA256        expected SHA-256 of the binary (skips SHA256SUMS fetch)
#   UCLASH_DOWNLOAD_URL  full URL of the binary    (skips host resolution)
#   UCLASH_LOCAL_FILE    install from a local file (skips downloading)
set -eu

REPO=${UCLASH_REPO:-IKEJAY-code/uclash}
VERSION=${UCLASH_VERSION:-latest}
BIN_DIR=${UCLASH_BIN_DIR:-$HOME/.local/bin}
EXPECTED_SHA=${UCLASH_SHA256:-}

DEFAULT_MIRRORS='https://gh-proxy.com https://ghfast.top https://ghproxy.net'
# GitHub direct aborts when it stays below 100 KB/s for 10s; user-provided
# mirrors or direct URLs only get the plain timeouts.
CURL_LIMITS='--connect-timeout 10 --speed-limit 100000 --speed-time 10 --max-time 600 --retry 1 --retry-delay 1 --retry-connrefused'
CURL_LIMITS_RELAXED='--connect-timeout 10 --max-time 600 --retry 1 --retry-delay 1 --retry-connrefused'

os=$(uname -s | tr '[:upper:]' '[:lower:]')
case "$os" in
  linux | darwin) ;;
  *) echo "uclash: unsupported OS: $os" >&2; exit 1 ;;
esac

arch=$(uname -m)
case "$arch" in
  x86_64 | amd64) arch=amd64 ;;
  aarch64 | arm64) arch=arm64 ;;
  *) echo "uclash: unsupported architecture: $arch" >&2; exit 1 ;;
esac

ASSET="uclash-$os-$arch"

fetch() {
  url=$1
  dest=$2
  if is_github_url "$url"; then
    limits=$CURL_LIMITS
  else
    limits=$CURL_LIMITS_RELAXED
  fi
  if [ -t 2 ]; then
    # shellcheck disable=SC2086
    curl -fL -# $limits -o "$dest" "$url"
  else
    # shellcheck disable=SC2086
    curl -fsSL $limits -o "$dest" "$url"
  fi
}

try_fetch() {
  label=$1
  url=$2
  dest=$3
  echo "uclash: trying $label" >&2
  echo "        $url" >&2
  if fetch "$url" "$dest"; then
    echo "uclash: downloaded $(wc -c < "$dest" | tr -d ' ') bytes via $label" >&2
    return 0
  fi
  echo "uclash: $label failed" >&2
  return 1
}

is_github_url() {
  case "$1" in
    https://github.com/* | https://raw.githubusercontent.com/* | https://api.github.com/*) return 0 ;;
    *) return 1 ;;
  esac
}

download_with_fallback() {
  url=$1
  dest=$2
  mirror=''

  # Explicitly configured mirrors come first: when a user sets
  # UCLASH_GH_MIRROR they have already decided which route they want.
  if [ -n "${UCLASH_GH_MIRROR:-}" ] && is_github_url "$url"; then
    for mirror in $UCLASH_GH_MIRROR; do
      [ -n "$mirror" ] || continue
      if try_fetch "mirror ${mirror%/}" "${mirror%/}/$url" "$dest"; then
        return 0
      fi
    done
  fi

  if is_github_url "$url"; then
    direct_label='GitHub direct'
  else
    direct_label='direct'
  fi
  if try_fetch "$direct_label" "$url" "$dest"; then
    return 0
  fi

  if is_github_url "$url"; then
    for mirror in $DEFAULT_MIRRORS; do
      if [ -n "${UCLASH_GH_MIRROR:-}" ]; then
        case " $UCLASH_GH_MIRROR " in
          *" $mirror "*) continue ;;
        esac
      fi
      if try_fetch "mirror ${mirror%/}" "${mirror%/}/$url" "$dest"; then
        return 0
      fi
    done
  fi
  return 1
}

sha256_file() {
  if command -v sha256sum >/dev/null 2>&1; then
    sha256sum "$1" | awk '{print $1}'
  elif command -v shasum >/dev/null 2>&1; then
    shasum -a 256 "$1" | awk '{print $1}'
  else
    return 1
  fi
}

sums_url_for() {
  case "$1" in
    */"$ASSET") printf '%s/SHA256SUMS\n' "${1%/$ASSET}" ;;
    *) return 1 ;;
  esac
}

resolve_url() {
  if [ -n "${UCLASH_DOWNLOAD_URL:-}" ]; then
    printf '%s\n' "$UCLASH_DOWNLOAD_URL"
    return 0
  fi
  if [ "$VERSION" = "latest" ]; then
    printf 'https://github.com/%s/releases/latest/download/%s\n' "$REPO" "$ASSET"
  else
    printf 'https://github.com/%s/releases/download/%s/%s\n' "$REPO" "$VERSION" "$ASSET"
  fi
}

mkdir -p "$BIN_DIR"
tmp=$(mktemp "${TMPDIR:-/tmp}/uclash.XXXXXX")
tmp_sums="${tmp}.SHA256SUMS"
trap 'rm -f "$tmp" "$tmp_sums"' EXIT

source_url=''
if [ -n "${UCLASH_LOCAL_FILE:-}" ]; then
  echo "uclash: installing from $UCLASH_LOCAL_FILE" >&2
  cp "$UCLASH_LOCAL_FILE" "$tmp"
else
  source_url=$(resolve_url)
  if ! download_with_fallback "$source_url" "$tmp"; then
    cat >&2 <<EOF

uclash: download failed on every route.

Fixes:
  * Put a mirror in front of the raw script URL, or pin one explicitly:
      UCLASH_GH_MIRROR=https://gh-proxy.com sh install.sh
  * Pin a release instead of "latest":
      UCLASH_VERSION=v0.1.0 sh install.sh
  * Install from a file downloaded in a browser:
      UCLASH_LOCAL_FILE=/path/to/$ASSET sh install.sh
  * Point at your own mirror:
      UCLASH_DOWNLOAD_URL=<url> sh install.sh
EOF
    exit 1
  fi
  head -c 1 "$tmp" 2>/dev/null | grep -q '<' && {
    echo "uclash: downloaded an HTML page instead of the binary" >&2
    echo "uclash: see the fallbacks above (UCLASH_LOCAL_FILE / UCLASH_DOWNLOAD_URL / mirrors)" >&2
    exit 1
  }
fi

actual_sha=''
actual_sha=$(sha256_file "$tmp") || true
if [ -n "$actual_sha" ]; then
  expected_sha=$EXPECTED_SHA
  if [ -z "$expected_sha" ] && [ -n "$source_url" ]; then
    if sums_url=$(sums_url_for "$source_url") && download_with_fallback "$sums_url" "$tmp_sums" >/dev/null 2>&1; then
      expected_sha=$(awk -v asset="$ASSET" '$2 == asset || $2 == "*" asset { print $1; exit }' "$tmp_sums")
    fi
  fi
  if [ -z "$expected_sha" ]; then
    echo "uclash: warning: no SHA256SUMS available for $VERSION; skipping checksum verification" >&2
    echo "        (pass UCLASH_SHA256=<hex> to verify explicitly)" >&2
  elif [ "$actual_sha" != "$expected_sha" ]; then
    echo "uclash: SHA-256 mismatch for $ASSET" >&2
    echo "  expected: $expected_sha" >&2
    echo "  actual:   $actual_sha" >&2
    echo "uclash: refusing to install a binary that does not match the release checksum" >&2
    exit 1
  else
    echo "uclash: SHA-256 verified ($actual_sha)" >&2
  fi
else
  echo "uclash: warning: no sha256sum/shasum available; skipping checksum verification" >&2
fi

chmod +x "$tmp"
mv "$tmp" "$BIN_DIR/uclash"
trap - EXIT

echo "uclash: installed $BIN_DIR/uclash"
case ":${PATH:-}:" in
  *":$BIN_DIR:"*) ;;
  *)
    echo "uclash: add $BIN_DIR to PATH, e.g.:"
    echo "  export PATH=\"$BIN_DIR:\$PATH\"   # add to ~/.bashrc or ~/.zshrc"
    ;;
esac
echo
echo "next steps:"
echo "  uclash init                 # fetch core + dashboard, pick ports"
echo "  uclash sub add <url>        # add your Clash/Mihomo subscription (base64 node links welcome)"
echo "  eval \"\$(uclash proxy on)\"   # start the core and proxy this shell (any shell)"
echo
echo "core/dashboard downloads can also use a mirror:"
echo "  uclash init --mirror https://gh-proxy.com"
