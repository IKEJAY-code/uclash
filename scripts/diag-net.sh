#!/usr/bin/env bash
# Diagnose why proxy nodes are unreachable from this machine.
#
#   curl -fsSL https://raw.githubusercontent.com/IKEJAY-code/uclash/main/scripts/diag-net.sh | bash
#
# It reports, for every profile uclash knows about:
#   - the first node's host:port (no credentials)
#   - whether a raw TCP connection to that endpoint succeeds
#   - DNS resolution for the host
# plus a mihomo-level latency test.
set -uo pipefail

UCLASH=${UCLASH:-uclash}
DATA_DIR=${UCLASH_HOME:-$HOME/.local/share/uclash}
PROFILE_DIR="$DATA_DIR/profiles"

echo "== uclash =="
if command -v "$UCLASH" >/dev/null 2>&1; then
    "$UCLASH" version 2>/dev/null | head -1
    "$UCLASH" status 2>/dev/null | head -8
else
    echo "  uclash not found in PATH"
fi

echo
echo "== direct internet (should be 200) =="
curl -sS --max-time 8 -o /dev/null -w '  baidu:  %{http_code}\n' https://www.baidu.com || echo "  baidu:  FAILED"
curl -sS --max-time 8 -o /dev/null -w '  github: %{http_code}\n' https://github.com || echo "  github: FAILED"

echo
echo "== default node latency test (mihomo, first 12) =="
if command -v "$UCLASH" >/dev/null 2>&1; then
    "$UCLASH" node test --timeout 3000 2>&1 | head -13
fi

echo
echo "== node endpoint reachability (raw TCP, 5s; up to 3 per profile) =="
echo "   ports above 1024 are often blocked on campus/IDC networks; 443/80 usually work"
for f in "$PROFILE_DIR"/*.yaml; do
    [ -f "$f" ] || continue
    printf '  %s\n' "$(basename "$f" .yaml)"
    # only look inside the proxies block: "nameserver:" would otherwise match too
    endpoints=$(awk '/^proxies:/{f=1;next} f&&/^[a-z]/{exit} f' "$f" | awk '
        match($0, /(^|[ ,{])server: [^,} ]+/) { h = substr($0, RSTART, RLENGTH); sub(/^.*server: /, "", h) }
        match($0, /(^|[ ,{])port: [0-9]+/)     { p = substr($0, RSTART, RLENGTH); sub(/^.*port: /, "", p) }
        h != "" && p != "" { print h, p; h = ""; p = "" }
    ' | awk '!seen[$0]++' | head -3)
    if [ -z "$endpoints" ]; then
        printf '    (no node endpoints found)\n'
        continue
    fi
    while read -r host port; do
        printf '    %s:%s\n' "$host" "$port"
        resolved=$(getent hosts "$host" 2>/dev/null | head -1 | awk '{print $1}')
        if [ -n "$resolved" ]; then
            printf '      dns: %s\n' "$resolved"
        else
            printf '      dns: FAILED (cannot resolve)\n'
        fi
        if timeout 5 bash -c "cat < /dev/null > /dev/tcp/$host/$port" 2>/dev/null; then
            printf '      tcp: reachable\n'
        else
            printf '      tcp: BLOCKED or unreachable (5s timeout)\n'
        fi
    done <<EOF
$endpoints
EOF
done

echo
echo "== outbound high-port egress test (portquiz.net listens on every port) =="
echo "   same IP, different ports: tells 'port filtered' apart from 'host unreachable'"
for p in 443 20011 40001; do
    if timeout 5 bash -c "cat < /dev/null > /dev/tcp/portquiz.net/$p" 2>/dev/null; then
        printf '  portquiz.net:%-6s open\n' "$p"
    else
        printf '  portquiz.net:%-6s BLOCKED (or portquiz itself unreachable)\n' "$p"
    fi
done

echo
echo "interpretation:"
echo "  * tcp BLOCKED for one profile but reachable for another -> that endpoint/port is"
echo "    filtered by this network (ask the network admin, or use a provider/port that works)"
echo "  * dns FAILED -> the machine's resolver cannot resolve the node domain"
echo "  * node test shows latency but the panel does not -> hard-refresh the panel (Ctrl+F5)"
