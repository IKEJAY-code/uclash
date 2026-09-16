#!/usr/bin/env bash
# Build release assets for all platforms and (optionally) publish them as a
# Gitee release.
#
#   scripts/release.sh v0.1.0                # build dist/uclash-{linux,darwin}-{amd64,arm64}
#   GITEE_TOKEN=xxx scripts/release.sh v0.1.0 --publish
#
# Requires: go, curl. The Gitee token needs "releases" permission on the repo.
set -euo pipefail

TAG=${1:-}
PUBLISH=false
[ "${2:-}" = "--publish" ] && PUBLISH=true
if [ -z "$TAG" ]; then
  echo "usage: scripts/release.sh <tag> [--publish]   e.g. scripts/release.sh v0.1.0" >&2
  exit 1
fi

REPO=${UCLASH_REPO:-IKEJAY-code/uclash}
COMMIT=$(git rev-parse --short HEAD 2>/dev/null || echo none)
DATE=$(date -u +%Y-%m-%dT%H:%M:%SZ)
LDFLAGS="-s -w -X gitee.com/IKEJAY-code/uclash/internal/version.Version=$TAG -X gitee.com/IKEJAY-code/uclash/internal/version.Commit=$COMMIT -X gitee.com/IKEJAY-code/uclash/internal/version.Date=$DATE"

mkdir -p dist
for target in linux/amd64 linux/arm64 darwin/amd64 darwin/arm64; do
  GOOS=${target%/*} GOARCH=${target#*/} CGO_ENABLED=0 \
    go build -trimpath -ldflags "$LDFLAGS" -o "dist/uclash-${target%/*}-${target#*/}" ./cmd/uclash
  echo "built dist/uclash-${target%/*}-${target#*/}"
done

( cd dist && rm -f SHA256SUMS && sha256sum uclash-* > SHA256SUMS )
echo "wrote dist/SHA256SUMS"

if [ "$PUBLISH" != "true" ]; then
  echo
  echo "next: create a tag and push, then re-run with --publish"
  echo "  git tag $TAG && git push origin $TAG"
  exit 0
fi

: "${GITEE_TOKEN:?set GITEE_TOKEN (Gitee 私人令牌) to publish}"
API="https://gitee.com/api/v5/repos/$REPO/releases"

echo "creating Gitee release $TAG ..."
resp=$(curl -fsS -X POST "$API" \
  --data-urlencode "access_token=$GITEE_TOKEN" \
  --data-urlencode "tag_name=$TAG" \
  --data-urlencode "name=$TAG" \
  --data-urlencode "target_commitish=main" \
  --data-urlencode "body=uclash $TAG")
id=$(printf '%s' "$resp" | sed -n 's/.*"id":\([0-9]*\).*/\1/p' | head -n 1)
if [ -z "$id" ]; then
  echo "failed to create release: $resp" >&2
  exit 1
fi
echo "release id: $id"

for f in dist/uclash-* dist/SHA256SUMS; do
  echo "uploading $f ..."
  curl -fsS -X POST "$API/$id/attach_files?access_token=$GITEE_TOKEN" -F "file=@$f" >/dev/null
done

echo "done: https://gitee.com/$REPO/releases/tag/$TAG"
