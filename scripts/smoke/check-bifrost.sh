#!/usr/bin/env bash
# Smoke test: Verify Bifrost is healthy internally and NOT publicly exposed.
#
# REQUIRES: running docker-compose stack
# REQUIRES: BIFROST_URL set in environment (defaults to http://localhost:8080)
#
# This test has two modes:
#   1. Internal health check — run from a host/container on the core network.
#   2. Public isolation check — run from a host outside the core network.
#
# Usage:
#   BIFROST_URL=http://bifrost:8080 ./check-bifrost.sh                # internal
#   BIFROST_URL=http://bifrost:8080 SERVER_IP=203.0.113.5 ./check-bifrost.sh  # also checks public

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
# shellcheck source=_helpers.sh
source "$SCRIPT_DIR/_helpers.sh"

echo "=== Smoke: Bifrost Health and Isolation ==="
info "Internal URL: $BIFROST_URL"
info "Public IP for isolation check: $SERVER_IP"

# ── Test 1: Internal health endpoint ─────────────────────
CODE=$(http_code_of GET "$BIFROST_URL/health")
if [ "$CODE" = "200" ]; then
    pass "Bifrost /health returned 200 (internal)"
elif [ "$CODE" = "000" ]; then
    info "Bifrost not reachable at $BIFROST_URL"
    info "If running from Docker host, Bifrost may be internal-only."
    info "Try: docker exec <newapi-container> wget -qO- http://bifrost:8080/health"
    skip "Bifrost internal health check (not reachable from this host)"
else
    fail "Bifrost /health returned $CODE (expected 200)"
fi

# ── Test 2: Public isolation — Bifrost must NOT be reachable from outside ──
# Only meaningful if SERVER_IP is set to a public/external IP.
if [ "$SERVER_IP" = "localhost" ] || [ "$SERVER_IP" = "127.0.0.1" ]; then
    skip "Public isolation check skipped (SERVER_IP is localhost; set to public IP to test)"
else
    BIFROST_PORTS=(8080 18080 28080)
    for PORT in "${BIFROST_PORTS[@]}"; do
        CODE=$(http_code_of GET "http://${SERVER_IP}:${PORT}/health")
        if [ "$CODE" = "000" ]; then
            pass "Bifrost port $PORT not reachable from $SERVER_IP (good)"
        else
            fail "Bifrost port $PORT IS reachable from $SERVER_IP (HTTP $CODE) -- SECURITY ISSUE"
        fi
    done
fi

print_result
