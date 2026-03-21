#!/usr/bin/env bash
# Smoke test: Verify New-API is healthy and responding on its public endpoint.
#
# REQUIRES: running docker-compose stack
# REQUIRES: NEWAPI_URL set in environment (defaults to http://localhost:3000)
#
# Usage:
#   NEWAPI_URL=https://api.example.com ./check-newapi.sh
#   NEWAPI_URL=https://api.example.com API_KEY=sk-xxx ./check-newapi.sh

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
# shellcheck source=_helpers.sh
source "$SCRIPT_DIR/_helpers.sh"

echo "=== Smoke: New-API Health ==="
info "Target: $NEWAPI_URL"

# ── Test 1: /api/status responds ──────────────────────────
CODE=$(http_code_of GET "$NEWAPI_URL/api/status")
if [ "$CODE" = "200" ]; then
    pass "/api/status returned 200"
else
    fail "/api/status returned $CODE (expected 200)"
fi

# ── Test 2: Root path responds (reverse proxy working) ────
CODE=$(http_code_of GET "$NEWAPI_URL/")
if [ "$CODE" = "200" ] || [ "$CODE" = "302" ] || [ "$CODE" = "301" ]; then
    pass "Root path returned $CODE"
else
    fail "Root path returned $CODE (expected 200, 301, or 302)"
fi

# ── Test 3: /v1/models responds (requires API_KEY) ───────
if [ -z "$API_KEY" ]; then
    skip "/v1/models check skipped (no API_KEY set)"
else
    CODE=$(http_code_of GET "$NEWAPI_URL/v1/models" \
        -H "Authorization: Bearer $API_KEY")
    if [ "$CODE" = "200" ]; then
        pass "/v1/models returned 200"
    elif [ "$CODE" = "401" ]; then
        fail "/v1/models returned 401 (API_KEY invalid or expired)"
    else
        fail "/v1/models returned $CODE (expected 200)"
    fi
fi

# ── Test 4: /v1/chat/completions accepts POST ────────────
if [ -z "$API_KEY" ]; then
    skip "Chat completions check skipped (no API_KEY set)"
else
    CODE=$(http_code_of POST "$NEWAPI_URL/v1/chat/completions" \
        -H "Authorization: Bearer $API_KEY" \
        -H "Content-Type: application/json" \
        -d '{"model":"gpt-4o-mini","messages":[{"role":"user","content":"ping"}],"max_tokens":1}')
    if [ "$CODE" = "200" ]; then
        pass "Chat completions returned 200"
    elif [ "$CODE" = "402" ] || [ "$CODE" = "429" ]; then
        pass "Chat completions returned $CODE (auth works, quota/rate limit hit)"
    elif [ "$CODE" = "401" ]; then
        fail "Chat completions returned 401 (API_KEY invalid)"
    else
        fail "Chat completions returned $CODE (expected 200, 402, or 429)"
    fi
fi

print_result
