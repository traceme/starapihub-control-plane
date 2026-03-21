#!/usr/bin/env bash
# Smoke test: Verify ClewdR is NOT reachable from the public network.
#
# ClewdR must be internal-only (PRD 9.1, 9.5). This is a negative test:
# success means the service is unreachable from the specified SERVER_IP.
#
# REQUIRES: running docker-compose stack
# REQUIRES: SERVER_IP set to the public-facing IP of the host
#
# Usage:
#   SERVER_IP=203.0.113.5 ./check-clewdr-isolation.sh
#   SERVER_IP=203.0.113.5 CLEWDR_PORTS="8484 18484 18485 18486" ./check-clewdr-isolation.sh

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
# shellcheck source=_helpers.sh
source "$SCRIPT_DIR/_helpers.sh"

echo "=== Smoke: ClewdR Network Isolation (negative test) ==="

# ClewdR ports to probe. Override with CLEWDR_PORTS env var if needed.
CLEWDR_PORTS="${CLEWDR_PORTS:-8484 18484 18485 18486}"

if [ "$SERVER_IP" = "localhost" ] || [ "$SERVER_IP" = "127.0.0.1" ]; then
    info "SERVER_IP is '$SERVER_IP'. For a meaningful isolation test,"
    info "set SERVER_IP to the public-facing IP of the deployment host."
    info "Proceeding anyway — testing localhost reachability."
fi

info "Probing ClewdR ports on $SERVER_IP: $CLEWDR_PORTS"

FOUND_EXPOSED=0
for PORT in $CLEWDR_PORTS; do
    # Try the root path
    CODE_ROOT=$(http_code_of GET "http://${SERVER_IP}:${PORT}/")
    # Also try the OpenAI-compatible endpoint ClewdR exposes
    CODE_V1=$(http_code_of GET "http://${SERVER_IP}:${PORT}/v1/models")

    if [ "$CODE_ROOT" = "000" ] && [ "$CODE_V1" = "000" ]; then
        pass "ClewdR port $PORT: not reachable (connection refused/timeout)"
    else
        fail "ClewdR port $PORT IS reachable from $SERVER_IP (root=$CODE_ROOT, /v1/models=$CODE_V1) -- SECURITY ISSUE"
        FOUND_EXPOSED=1
    fi
done

if [ "$FOUND_EXPOSED" -eq 1 ]; then
    info ""
    info "ClewdR is exposed publicly. To fix:"
    info "  1. Ensure docker-compose does NOT map ClewdR ports to the host."
    info "  2. Ensure ClewdR containers are on an internal-only Docker network."
    info "  3. If using a firewall, block ClewdR ports from external access."
fi

# ── Optional: verify ClewdR IS reachable internally ──────
# This confirms the service is actually running (not just absent).
info ""
info "Verifying ClewdR is alive internally (via CLEWDR_URL=$CLEWDR_URL)..."
CODE=$(http_code_of GET "$CLEWDR_URL/")
if [ "$CODE" != "000" ]; then
    pass "ClewdR responds internally at $CLEWDR_URL (HTTP $CODE)"
else
    skip "ClewdR not reachable internally at $CLEWDR_URL (may need docker exec)"
    info "Try: docker exec <newapi-container> wget -qO- http://clewdr-1:8484/"
fi

print_result
