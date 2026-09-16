#!/bin/sh
# uclash installer â€?npm-style, per-user, no sudo.
#
# Gitee (default):
#   curl -fsSL https://gitee.com/IKEJAY-code/uclash/raw/main/scripts/install.sh | sh
# GitHub mirror:
#   UCLASH_HOST=github sh install.sh
#
# Environment overrides:
#   UCLASH_HOST          gitee | github            (default: gitee)
#   UCLASH_REPO          owner/repo                (default: IKEJAY-code/uclash)
#   UCLASH_BIN_DIR       install dir               (default: ~/.local/bin)
#   UCLASH_VERSION       release tag or "latest"   (default: latest)
#   UCLASH_GH_MIRROR     GitHub acceleration prefix (UCLASH_HOST=github only)
#   UCLASH_DOWNLOAD_URL  full URL of the binary    (skips host resolution)
#   UCLASH_LOCAL_FILE    install from a local file (skips downloading)
set -eu

HOST=${UCLASH_HOST:-gitee}
REPO=${UCLASH_REPO:-IKEJAY-code/uclash}
VERSION=${UCLASH_VERSION:-latest}
MIRROR=${UCLASH_GH_MIRROR:-}
BIN_DIR=${UCLASH_BIN_DIR:-$HOME/.local/bin}

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

get_text() {
  if command -v curl >/dev/null 2>&1; then
    curl -fsSL "$1"
  elif command -v wget >/dev/null 2>&1; then
    wget -qO- "$1"
  else
    echo "uclash: need curl or wget" >&2
    return 1
  fi
}

get_file() {
  if command -v curl >/dev/null 2>&1; then
    curl -fsSL -o "$2" "$1"
  elif command -v wget >/dev/null 2>&1; then
    wget -qO "$2" "$1"
  else
    echo "uclash: need curl or wget" >&2
    return 1
  fi
}

gitee_latest_asset_url() {
  json=$(get_text "https://gitee.com/api/v5/repos/$REPO/releases/latest") || return 1
  printf '%s' "$json" |
    tr ',' '\n' |
    sed -n 's/.*"browser_download_url":"\([^"]*\)".*/\1/p' |
    grep "/$ASSET\$" |
    head -n 1
}

resolve_url() {
  if [ -n "${UCLASH_DOWNLOAD_URL:-}" ]; then
    printf '%s\n' "$UCLASH_DOWNLOAD_URL"
    return 0
  fi
  case "$HOST" in
    github)
      if [ "$VERSION" = "latest" ]; then
        printf 'https://github.com/%s/releases/latest/download/%s\n' "$REPO" "$ASSET"
      else
        printf 'https://github.com/%s/releases/download/%s/%s\n' "$REPO" "$VERSION" "$ASSET"
      fi
      ;;
    gitee)
      if [ "$VERSION" = "latest" ]; then
        if url=$(gitee_latest_asset_url) && [ -n "$url" ]; then
          printf '%s\n' "$url"
        else
          printf 'https://gitee.com/%s/releases/download/%s/%s\n' "$REPO" "$VERSION" "$ASSET"
        fi
      else
        printf 'https://gitee.com/%s/releases/download/%s/%s\n' "$REPO" "$VERSION" "$ASSET"
      fi
      ;;
    *)
      echo "uclash: unknown UCLASH_HOST=$HOST (want gitee or github)" >&2
      return 1
      ;;
  esac
}

mkdir -p "$BIN_DIR"
tmp=$(mktemp "${TMPDIR:-/tmp}/uclash.XXXXXX")
trap 'rm -f "$tmp"' EXIT

if [ -n "${UCLASH_LOCAL_FILE:-}" ]; then
  echo "uclash: installing from $UCLASH_LOCAL_FILE"
  cp "$UCLASH_LOCAL_FILE" "$tmp"
else
  url=$(resolve_url)
  if [ -n "$MIRROR" ] && [ "$HOST" = "github" ]; then
    url="${MIRROR%/}/$url"
  fi
  echo "uclash: downloading $url"
  if ! get_file "$url" "$tmp"; then
    cat >&2 <<EOF
uclash: download failed.

Common causes and fixes:
  * Gitee sometimes blocks scripted downloads (captcha/login).
    Download the asset in a browser and rerun with:
      UCLASH_LOCAL_FILE=/path/to/$ASSET sh install.sh
  * Pin a version instead of "latest":
      UCLASH_VERSION=v0.1.0 sh install.sh
  * Use the GitHub mirror instead:
      UCLASH_HOST=github UCLASH_GH_MIRROR=https://gh-proxy.com sh install.sh
  * Or point at a direct URL:
      UCLASH_DOWNLOAD_URL=<url> sh install.sh
EOF
    exit 1
  fi
  head -c 1 "$tmp" 2>/dev/null | grep -q '<' && {
    echo "uclash: downloaded an HTML page instead of the binary (captcha/login wall?)" >&2
    echo "uclash: see the fallbacks above (UCLASH_LOCAL_FILE / UCLASH_DOWNLOAD_URL / UCLASH_HOST=github)" >&2
    exit 1
  }
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
echo "  uclash start && proxyon     # start the core and proxy this shell"
echo
echo "behind GitHub access problems? use a mirror:"
echo "  UCLASH_GH_MIRROR=https://gh-proxy.com uclash init"
