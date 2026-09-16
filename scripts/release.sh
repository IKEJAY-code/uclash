#!/usr/bin/env bash
# Build release assets for all platforms and (optionally) publish them as a
# GitHub release.
#
#   scripts/release.sh v0.1.0              # build dist/ + dist/SHA256SUMS
#   scripts/release.sh v0.1.0 --no-build   # reuse existing dist/ files
#   scripts/release.sh v0.1.0 --publish    # publish with the GitHub CLI (gh)
#
# Requires: go, sha256sum; the GitHub CLI (authenticated with `gh auth login`)
# for --publish. Pushing a v* tag also triggers .github/workflows/release.yml,
# which publishes automatically unless a release for that tag already exists.
set -euo pipefail

TAG=${1:-}
shift || true
PUBLISH=false
NOBUILD=false
for arg in "$@"; do
  case "$arg" in
    --publish) PUBLISH=true ;;
    --no-build) NOBUILD=true ;;
    *)
      echo "unknown option: $arg" >&2
      exit 2
      ;;
  esac
done
if [ -z "$TAG" ]; then
  echo "usage: scripts/release.sh <tag> [--no-build] [--publish]   e.g. scripts/release.sh v0.1.1" >&2
  exit 1
fi

REPO=${UCLASH_REPO:-IKEJAY-code/uclash}
COMMIT=$(git rev-parse --short HEAD 2>/dev/null || echo none)
DATE=$(date -u +%Y-%m-%dT%H:%M:%SZ)
LDFLAGS="-s -w -X gitee.com/IKEJAY-code/uclash/internal/version.Version=$TAG -X gitee.com/IKEJAY-code/uclash/internal/version.Commit=$COMMIT -X gitee.com/IKEJAY-code/uclash/internal/version.Date=$DATE"

if [ "$NOBUILD" != "true" ]; then
  mkdir -p dist
  for target in linux/amd64 linux/arm64 darwin/amd64 darwin/arm64; do
    GOOS=${target%/*} GOARCH=${target#*/} CGO_ENABLED=0 \
      go build -trimpath -ldflags "$LDFLAGS" -o "dist/uclash-${target%/*}-${target#*/}" ./cmd/uclash
    echo "built dist/uclash-${target%/*}-${target#*/}"
  done
  ( cd dist && rm -f SHA256SUMS && sha256sum uclash-* > SHA256SUMS )
  echo "wrote dist/SHA256SUMS"
else
  for f in dist/uclash-linux-amd64 dist/uclash-linux-arm64 dist/uclash-darwin-amd64 \
    dist/uclash-darwin-arm64 dist/SHA256SUMS; do
    [ -f "$f" ] || {
      echo "missing $f (drop --no-build to rebuild)" >&2
      exit 1
    }
  done
fi

if [ "$PUBLISH" != "true" ]; then
  echo
  echo "next:"
  echo "  git tag $TAG && git push origin $TAG          # triggers the release workflow"
  echo "  # or publish the built files directly:"
  echo "  gh release create $TAG dist/uclash-* dist/SHA256SUMS --repo $REPO --title $TAG"
  exit 0
fi

command -v gh >/dev/null 2>&1 || {
  echo "the GitHub CLI (gh) is required for --publish; install it or publish dist/ manually" >&2
  exit 1
}
gh release create "$TAG" dist/uclash-* dist/SHA256SUMS \
  --repo "$REPO" --title "$TAG" --generate-notes
echo "published: https://github.com/$REPO/releases/tag/$TAG"
