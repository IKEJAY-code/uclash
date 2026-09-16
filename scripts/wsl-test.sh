#!/usr/bin/env bash
# End-to-end integration test for uclash.
#
# Runs inside Linux (e.g. WSL) with curl + python3 available. It needs no
# sudo, no systemd and no external network: the subscription is served from
# a local HTTP server and the core/dashboard come from local paths.
#
# Required:
#   BIN      path to a uclash linux binary
#   CORE     path to a mihomo linux binary   (passed to `init --core`)
#   UI       path to a metacubexd directory  (passed to `init --ui`)
#   FIXTURE  path to a valid Clash YAML subscription
# Optional:
#   GEODATA  directory with country.mmdb/geosite.dat/geoip.dat
set -uo pipefail

BIN=${BIN:?BIN is required}
CORE=${CORE:?CORE is required}
UI=${UI:?UI is required}
FIXTURE=${FIXTURE:?FIXTURE is required}
GEODATA=${GEODATA:-}

PASS=0
FAIL=0
ok()  { PASS=$((PASS + 1)); printf '  ok   %s\n' "$1"; }
bad() { FAIL=$((FAIL + 1)); printf '  FAIL %s\n' "$1"; }
check()  { local desc=$1; shift; if "$@" >/dev/null 2>&1; then ok "$desc"; else bad "$desc"; fi; }
ncheck() { local desc=$1; shift; if "$@" >/dev/null 2>&1; then bad "$desc"; else ok "$desc"; fi; }
section() { printf '\n== %s ==\n' "$1"; }

A=$(mktemp -d /tmp/uclash-a.XXXXXX)
B=$(mktemp -d /tmp/uclash-b.XXXXXX)
HTTP_PID=""
HTTP2_PID=""
SLEEP_PID=""
A_RUN() { UCLASH_HOME="$A/home" UCLASH_CONFIG_DIR="$A/config" "$BIN" "$@"; }
B_RUN() { UCLASH_HOME="$B/home" UCLASH_CONFIG_DIR="$B/config" "$BIN" "$@"; }

cleanup() {
  [ -n "$HTTP_PID" ] && kill "$HTTP_PID" 2>/dev/null
  [ -n "$HTTP2_PID" ] && kill "$HTTP2_PID" 2>/dev/null
  [ -n "$SLEEP_PID" ] && kill "$SLEEP_PID" 2>/dev/null
  A_RUN stop --quiet >/dev/null 2>&1
  B_RUN stop --quiet >/dev/null 2>&1
  rm -rf "$A" "$B"
}
trap cleanup EXIT

secret_of() { awk '$1 == "secret:" {print $2}' "$1/config/config.yaml"; }
mixed_of()  { awk '$1 == "mixed:" {print $2}' "$1/config/config.yaml"; }
ctrl_of()   { awk '$1 == "controller:" {print $2}' "$1/config/config.yaml"; }

api() {
  local port=${1:?} secret=${2:?} path=${3:?}
  curl -fsS -H "Authorization: Bearer $secret" "http://127.0.0.1:$port$path"
}
api_has()   { api "$1" "$2" "$3" | grep -q "$4"; }
ui_served() { curl -fsS "http://127.0.0.1:$1/ui/" | grep -qi '<html'; }
status_a()  { [ "$(A_RUN status --quiet)" = "$1" ]; }
status_b()  { [ "$(B_RUN status --quiet)" = "$1" ]; }
profile_list()   { A_RUN sub ls | grep -q "$1"; }
profile_absent() { ! A_RUN sub ls | grep -q "$1"; }
free_port() { python3 -c 'import socket; s=socket.socket(); s.bind(("127.0.0.1",0)); print(s.getsockname()[1]); s.close()'; }

section "1. init with local core + local ui"
A_RUN init --core "$CORE" --ui "$UI" --no-shell >/dev/null
check "config file created"                  test -f "$A/config/config.yaml"
check "core installed and executable"        test -x "$A/home/bin/mihomo"
check "dashboard installed"                  test -f "$A/home/ui/index.html"
if [ -n "$GEODATA" ]; then
  cp "$GEODATA"/* "$A/home/" 2>/dev/null
  ok "geodata copied into data dir"
fi
A_MIXED=$(mixed_of "$A")
A_CTRL=$(ctrl_of "$A")
A_SECRET=$(secret_of "$A")
ports_distinct() { [ -n "$A_MIXED" ] && [ -n "$A_CTRL" ] && [ "$A_MIXED" != "$A_CTRL" ]; }
check "ports assigned (mixed=$A_MIXED controller=$A_CTRL)" ports_distinct
check "secret generated"                     test "${#A_SECRET}" -ge 16

section "2. subscription add from a local HTTP server"
python3 -m http.server 18080 --directory "$(dirname "$FIXTURE")" >/dev/null 2>&1 &
HTTP_PID=$!
sleep 1
A_RUN sub add "http://127.0.0.1:18080/$(basename "$FIXTURE")" --name local
check "profile stored"                       test -s "$A/home/profiles/local.yaml"
check "runtime config generated"             test -s "$A/home/config.yaml"
check "override: mixed-port present"         grep -q '^mixed-port:' "$A/home/config.yaml"
check "override: external-controller set"    grep -q '^external-controller:' "$A/home/config.yaml"
check "override: external-ui set"            grep -q '^external-ui:' "$A/home/config.yaml"
ncheck "hardened: tun block stripped"        grep -q '^tun:' "$A/home/config.yaml"
check "hardened: allow-lan false"            grep -q 'allow-lan: false' "$A/home/config.yaml"

section "2b. subscription URL hygiene"
sub_add_local_hint() { local out; out=$(A_RUN sub add "$FIXTURE" 2>&1); grep -q 'profile import' <<< "$out"; }
escaped_url="http://127.0.0.1:18080/$(basename "$FIXTURE")\?x\=1"
check "shell-escaped URL is accepted"        A_RUN sub add "$escaped_url" --name escaped
check "escaped profile stored"               profile_list escaped
check "local file gets a profile-import hint" sub_add_local_hint
ncheck "scheme-less argument is rejected"    A_RUN sub add example.com/sub
A_RUN sub rm escaped >/dev/null 2>&1

section "3. start / API / dashboard"
A_RUN start >/dev/null
check "status --quiet == running"            status_a running
check "API /version reachable"               api_has "$A_CTRL" "$A_SECRET" /version version
check "dashboard served at /ui/"             ui_served "$A_CTRL"
ncheck "API auth rejects wrong secret"       api_has "$A_CTRL" wrong /version version

section "4. mode switching"
A_RUN mode global >/dev/null
check "live mode is global"                  api_has "$A_CTRL" "$A_SECRET" /configs '"mode":"global"'
A_RUN mode rule >/dev/null
check "live mode back to rule"               api_has "$A_CTRL" "$A_SECRET" /configs '"mode":"rule"'

section "5. port changes"
NEW_MIXED=$(free_port)
A_RUN port set mixed "$NEW_MIXED" >/dev/null
check "live mixed-port changed"              api_has "$A_CTRL" "$A_SECRET" /configs "\"mixed-port\":$NEW_MIXED"
NEW_CTRL=$(free_port)
A_RUN port set controller "$NEW_CTRL" >/dev/null
check "core restarted on new controller"     status_a running
check "new controller port answers"          api_has "$NEW_CTRL" "$A_SECRET" /version version
A_CTRL=$NEW_CTRL
A_MIXED=$NEW_MIXED

section "6. terminal env"
env_on_has()  { A_RUN env on | grep -q "$1"; }
env_off_has() { A_RUN env off | grep -q "$1"; }
check "env on exports our port"              env_on_has "http://127.0.0.1:$A_MIXED"
check "env off is conditional"               env_off_has "unset http_proxy"
if (
  eval "$(A_RUN env on)"
  [ "${http_proxy:-}" = "http://127.0.0.1:$A_MIXED" ]
); then ok "eval env on sets http_proxy"; else bad "eval env on sets http_proxy"; fi
if (
  eval "$(A_RUN env on)"
  eval "$(A_RUN env off)"
  [ -z "${http_proxy:-}" ]
); then ok "eval env off unsets http_proxy"; else bad "eval env off unsets http_proxy"; fi

section "7. ui / log / profile import / sub while running"
ui_has_url()    { A_RUN ui --plain | grep -q "http://127.0.0.1:$A_CTRL/ui/"; }
log_nonempty()  { [ -n "$(A_RUN log -n 5)" ]; }
check "ui prints the dashboard URL"          ui_has_url
check "log has content"                      log_nonempty
IMPORTED=$(mktemp /tmp/imported.XXXXXX.yaml)
cp "$FIXTURE" "$IMPORTED"
check "profile import while running succeeds" A_RUN profile import "$IMPORTED" --name imported
check "imported profile listed"              profile_list imported
check "imported profile can be activated"    A_RUN profile use imported
check "imported profile is active"           profile_list '^\* imported'
check "sub add while running succeeds"       A_RUN sub add "http://127.0.0.1:18080/$(basename "$FIXTURE")" --name local2
check "core still answers after hot reload"  api_has "$A_CTRL" "$A_SECRET" /version version
check "sub update succeeds"                  A_RUN sub update local2
check "sub rm removes profile"               A_RUN sub rm local2
check "removed profile is gone"              profile_absent local2

section "7b. base64 node-link subscription + node commands"
check "base64 sub add succeeds"              A_RUN sub add "http://127.0.0.1:18080/fixture-base64.txt" --name b64
check "profile marked as converted"          A_RUN sub ls | grep -q 'sub(conv)'
check "converted profile has vmess node"     grep -q 'type: vmess' "$A/home/profiles/b64.yaml"
check "converted profile has reality node"   grep -q 'reality-opts:' "$A/home/profiles/b64.yaml"
check "reality public key preserved"         grep -q 'public-key:' "$A/home/profiles/b64.yaml"
check "vmess alterId present"                grep -q 'alterId: 0' "$A/home/profiles/b64.yaml"
check "converted profile has hysteria2 node" grep -q 'type: hysteria2' "$A/home/profiles/b64.yaml"
check "converted profile has groups"         grep -q 'name: AUTO' "$A/home/profiles/b64.yaml"
check "sub use b64"                          A_RUN sub use b64
check "core answers after converted reload"  api_has "$A_CTRL" "$A_SECRET" /version version
check "node ls shows group"                  A_RUN node ls | grep -q 'current:'
check "node ls PROXY lists nodes"            A_RUN node ls PROXY | grep -q 'B64-Trojan'
check "node use by name"                     A_RUN node use PROXY B64-Trojan
check "API reflects node switch"             api_has "$A_CTRL" "$A_SECRET" /proxies/PROXY '"now":"B64-Trojan"'
check "node use auto-picks group"            A_RUN node use B64-Hy2
check "API reflects auto-picked switch"      api_has "$A_CTRL" "$A_SECRET" /proxies/PROXY '"now":"B64-Hy2"'
check "node test runs"                       A_RUN node test PROXY --timeout 500
check "plain (non-base64) links also work"   A_RUN sub add "http://127.0.0.1:18080/fixture-links.txt" --name links
ncheck "node use rejects unknown group"      A_RUN node use NO_SUCH_GROUP x

section "7c. reload rollback when mihomo rejects a config"
check "invalid profile can be added"         A_RUN sub add "http://127.0.0.1:18080/fixture-invalid.yaml" --name bad
ncheck "activating invalid profile fails"    A_RUN sub use bad
check "active profile unchanged after reject" profile_list '^\* b64'
A_RUN sub rm bad >/dev/null 2>&1
check "core still healthy after rejection"   api_has "$A_CTRL" "$A_SECRET" /version version
check "recovered config still active"        api_has "$A_CTRL" "$A_SECRET" /proxies/PROXY '"now"'

section "8. second instance: multi-user isolation"
B_RUN init --core "$CORE" --ui "$UI" --no-shell >/dev/null
[ -n "$GEODATA" ] && cp "$GEODATA"/* "$B/home/" 2>/dev/null
B_RUN sub add "http://127.0.0.1:18080/$(basename "$FIXTURE")" --name other >/dev/null
B_RUN start >/dev/null
B_CTRL=$(ctrl_of "$B")
B_SECRET=$(secret_of "$B")
check "second instance uses different ports (A=$A_CTRL B=$B_CTRL)" test "$A_CTRL" != "$B_CTRL"
check "second API reachable"                 api_has "$B_CTRL" "$B_SECRET" /version version
check "first API still reachable"            api_has "$A_CTRL" "$A_SECRET" /version version
B_RUN stop --quiet >/dev/null
check "second instance stopped"              status_b stopped
check "first instance untouched by second stop" api_has "$A_CTRL" "$A_SECRET" /version version

section "9. stop safety: never kill a foreign process"
sleep 300 &
SLEEP_PID=$!
mkdir -p "$B/home/run"
printf '{"pid": %d, "started_at": "2026-01-01T00:00:00Z", "core": "%s", "config": "%s", "data_dir": "%s"}\n' \
  "$SLEEP_PID" "$B/home/bin/mihomo" "$B/home/config.yaml" "$B/home" > "$B/home/run/mihomo.pid"
ncheck "stop refused a foreign pid file"     B_RUN stop --quiet
check "foreign process still alive"          kill -0 "$SLEEP_PID"
kill "$SLEEP_PID" 2>/dev/null
SLEEP_PID=""

section "10. doctor + stop"
check "doctor exits 0"                       A_RUN doctor
A_RUN stop --quiet >/dev/null
check "first instance stopped"               status_a stopped

section "11. npm installer (asset served locally)"
if [ -n "${NPM_DIR:-}" ] && command -v node >/dev/null 2>&1; then
  NPMTMP=$(mktemp -d)
  cp -r "$NPM_DIR" "$NPMTMP/pkg"
  OS=$(uname -s | tr '[:upper:]' '[:lower:]')
  MACH=$(uname -m)
  case "$MACH" in x86_64) MACH=amd64 ;; aarch64 | arm64) MACH=arm64 ;; esac
  cp "$BIN" "$NPMTMP/uclash-$OS-$MACH"
  FIXTURE_SHA=$(sha256sum "$BIN" | awk '{print $1}')
  python3 -m http.server 18082 --directory "$NPMTMP" >/dev/null 2>&1 &
  HTTP2_PID=$!
  sleep 1
  UCLASH_ASSET_URL="http://127.0.0.1:18082/uclash-$OS-$MACH" \
  UCLASH_VERSION=v0.0.0-test UCLASH_SHA256="$FIXTURE_SHA" \
    node "$NPMTMP/pkg/install.js" >/dev/null
  check "npm install.js downloaded the binary" test -x "$NPMTMP/pkg/bin/uclash-bin"
  check "downloaded binary runs"             "$NPMTMP/pkg/bin/uclash-bin" version
  UCLASH_ASSET_URL="http://127.0.0.1:18082/uclash-$OS-$MACH" \
  UCLASH_VERSION=v0.0.0-test UCLASH_SHA256="$FIXTURE_SHA" \
    node "$NPMTMP/pkg/install.js" | grep -q 'already present' \
    && ok "npm installer is idempotent" || bad "npm installer is idempotent"
  ncheck "npm installer rejects a bad checksum" env \
    UCLASH_ASSET_URL="http://127.0.0.1:18082/uclash-$OS-$MACH" \
    UCLASH_VERSION=v0.0.0-test \
    UCLASH_SHA256=0000000000000000000000000000000000000000000000000000000000000000 \
    node "$NPMTMP/pkg/install.js"
  check "bad checksum removed the download"  test ! -e "$NPMTMP/pkg/bin/uclash-bin"
  kill "$HTTP2_PID" 2>/dev/null
  rm -rf "$NPMTMP"
else
  echo "(skipped: node not available in this environment)"
fi

printf '\n==============================\n'
printf 'passed: %d, failed: %d\n' "$PASS" "$FAIL"
[ "$FAIL" -eq 0 ]
