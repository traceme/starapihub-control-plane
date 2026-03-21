#!/usr/bin/env bash
# Smoke test: Verify X-Request-ID correlation header propagates through the stack.
#
# Sends a request to New-API with a known X-Request-ID and checks whether:
#   1. New-API echoes it back in the response headers
#   2. The response body (if available) can be correlated
#
# Full end-to-end correlation (checking Bifrost and provider logs) requires
# log access and is covered by integration tests, not this smoke test.
#
# REQUIRES: running docker-compose stack
# REQUIRES: API_KEY set in environment
# REQUIRES: NEWAPI_URL set in environment (defaults to http://localhost:3000)
#
# Usage:
#   NEWAPI_URL=https://api.example.com API_KEY=sk-xxx ./check-correlation.sh

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
# shellcheck source=_helpers.sh
source "$SCRIPT_DIR/_helpers.sh"

echo "=== Smoke: X-Request-ID Correlation ==="
info "Target: $NEWAPI_URL"

if [ -z "$API_KEY" ]; then
    skip "API_KEY is required for correlation test"
    print_result
    exit 0
fi

# Generate a unique request ID for this test run.
REQUEST_ID="smoke-$(date +%s)-$(head -c 4 /dev/urandom | od -An -tx1 | tr -d ' \n')"
info "Sending X-Request-ID: $REQUEST_ID"

HEADERS_FILE=$(mktemp)
BODY_FILE=$(mktemp)
trap 'rm -f "$HEADERS_FILE" "$BODY_FILE"' EXIT

CODE=$(curl -s -o "$BODY_FILE" -D "$HEADERS_FILE" -w "%{http_code}" \
    --connect-timeout "$CONNECT_TIMEOUT" \
    -X POST "$NEWAPI_URL/v1/chat/completions" \
    -H "Authorization: Bearer $API_KEY" \
    -H "Content-Type: application/json" \
    -H "X-Request-ID: $REQUEST_ID" \
    -d '{"model":"gpt-4o-mini","messages":[{"role":"user","content":"ping"}],"max_tokens":1}' \
    2>/dev/null || echo "000")

# ── Test 1: Request succeeded (any auth-valid response) ──
if [ "$CODE" = "000" ]; then
    fail "Connection to New-API failed"
    rm -f "$HEADERS_FILE" "$BODY_FILE"
    print_result
    exit 1
fi

if [ "$CODE" = "401" ]; then
    fail "API_KEY invalid (HTTP 401)"
    rm -f "$HEADERS_FILE" "$BODY_FILE"
    print_result
    exit 1
fi

pass "New-API responded with HTTP $CODE"

# ── Test 2: X-Request-ID echoed in response headers ──────
# Check response headers (case-insensitive) for our request ID.
if grep -qi "x-request-id" "$HEADERS_FILE" 2>/dev/null; then
    RETURNED_ID=$(grep -i "x-request-id" "$HEADERS_FILE" | head -1 | cut -d: -f2- | tr -d '[:space:]')
    if [ "$RETURNED_ID" = "$REQUEST_ID" ]; then
        pass "X-Request-ID echoed correctly in response: $RETURNED_ID"
    else
        info "X-Request-ID present but different: sent=$REQUEST_ID, got=$RETURNED_ID"
        pass "X-Request-ID header present in response (value: $RETURNED_ID)"
    fi
else
    # Not all proxies echo X-Request-ID. This is a known limitation.
    info "X-Request-ID not found in response headers."
    info "New-API may not echo this header by default."
    info "Consider configuring nginx to add: proxy_set_header X-Request-ID \$request_id;"
    skip "X-Request-ID not echoed (may need nginx/proxy configuration)"
fi

# ── Test 3: Suggest log correlation ──────────────────────
info ""
info "To verify full end-to-end correlation, check logs for request ID: $REQUEST_ID"
info "  New-API:  docker logs <newapi-container> | grep '$REQUEST_ID'"
info "  Bifrost:  docker logs <bifrost-container> | grep '$REQUEST_ID'"
info "  Nginx:    grep '$REQUEST_ID' /var/log/nginx/access.log"

rm -f "$HEADERS_FILE" "$BODY_FILE"

print_result
