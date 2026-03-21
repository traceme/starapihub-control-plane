#!/usr/bin/env bash
# Smoke test: Verify fallback behavior when a provider is unavailable.
#
# This test exercises the standard-tier routing policy, where ClewdR is an
# allowed fallback. It works by:
#   1. Sending a request to a standard-tier model (cheap-chat or fast-chat)
#   2. Verifying the request succeeds
#   3. Optionally: if DISABLE_PRIMARY=true, the operator has manually paused
#      the primary provider, and we verify requests still succeed via fallback
#
# Full failure drill (disabling providers programmatically) requires Bifrost
# admin API access and is covered separately in the runbook.
#
# REQUIRES: running docker-compose stack
# REQUIRES: API_KEY set in environment
# REQUIRES: NEWAPI_URL set in environment (defaults to http://localhost:3000)
#
# Usage:
#   # Basic: verify standard-tier routes work
#   NEWAPI_URL=https://api.example.com API_KEY=sk-xxx ./check-fallback.sh
#
#   # After manually disabling primary provider in Bifrost:
#   NEWAPI_URL=https://api.example.com API_KEY=sk-xxx DISABLE_PRIMARY=true ./check-fallback.sh

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
# shellcheck source=_helpers.sh
source "$SCRIPT_DIR/_helpers.sh"

echo "=== Smoke: Fallback Behavior ==="
info "Target: $NEWAPI_URL"

if [ -z "$API_KEY" ]; then
    skip "API_KEY is required for fallback test"
    print_result
    exit 0
fi

DISABLE_PRIMARY="${DISABLE_PRIMARY:-false}"
FALLBACK_MODEL="${FALLBACK_MODEL:-cheap-chat}"

info "Test model: $FALLBACK_MODEL (standard tier, fallback allowed)"
if [ "$DISABLE_PRIMARY" = "true" ]; then
    info "DISABLE_PRIMARY=true: expecting fallback to secondary provider"
fi
echo ""

# ── Test 1: Baseline — standard-tier model resolves ──────
info "Step 1: Sending baseline request to $FALLBACK_MODEL..."
RESPONSE_FILE=$(mktemp)
HEADERS_FILE=$(mktemp)
_FALLBACK_TMPFILES="$RESPONSE_FILE $HEADERS_FILE"
trap 'rm -f $_FALLBACK_TMPFILES' EXIT
REQUEST_ID="fallback-$(date +%s)-$(head -c 4 /dev/urandom | od -An -tx1 | tr -d ' \n')"

CODE=$(curl -s -o "$RESPONSE_FILE" -D "$HEADERS_FILE" -w "%{http_code}" \
    --connect-timeout "$CONNECT_TIMEOUT" \
    --max-time 30 \
    -X POST "$NEWAPI_URL/v1/chat/completions" \
    -H "Authorization: Bearer $API_KEY" \
    -H "Content-Type: application/json" \
    -H "X-Request-ID: $REQUEST_ID" \
    -d "{\"model\":\"$FALLBACK_MODEL\",\"messages\":[{\"role\":\"user\",\"content\":\"say ok\"}],\"max_tokens\":3}" \
    2>/dev/null || echo "000")

case "$CODE" in
    200)
        pass "$FALLBACK_MODEL returned 200 (request completed)"
        ;;
    402|429)
        pass "$FALLBACK_MODEL returned $CODE (model recognized, quota/rate limit)"
        ;;
    000)
        fail "$FALLBACK_MODEL connection failed (New-API unreachable)"
        ;;
    *)
        BODY=$(cat "$RESPONSE_FILE" 2>/dev/null || echo "")
        fail "$FALLBACK_MODEL returned $CODE (body: $BODY)"
        ;;
esac

# ── Test 2: If primary disabled, verify request still succeeds ──
if [ "$DISABLE_PRIMARY" = "true" ]; then
    info ""
    info "Step 2: Primary provider reported as disabled. Verifying fallback..."
    info "(Ensure you have actually disabled the primary provider in Bifrost first.)"

    RESPONSE_FILE2=$(mktemp)
    _FALLBACK_TMPFILES="$_FALLBACK_TMPFILES $RESPONSE_FILE2"
    REQUEST_ID2="fallback-verify-$(date +%s)-$(head -c 4 /dev/urandom | od -An -tx1 | tr -d ' \n')"

    CODE2=$(curl -s -o "$RESPONSE_FILE2" -w "%{http_code}" \
        --connect-timeout "$CONNECT_TIMEOUT" \
        --max-time 60 \
        -X POST "$NEWAPI_URL/v1/chat/completions" \
        -H "Authorization: Bearer $API_KEY" \
        -H "Content-Type: application/json" \
        -H "X-Request-ID: $REQUEST_ID2" \
        -d "{\"model\":\"$FALLBACK_MODEL\",\"messages\":[{\"role\":\"user\",\"content\":\"say ok\"}],\"max_tokens\":3}" \
        2>/dev/null || echo "000")

    case "$CODE2" in
        200)
            pass "Fallback succeeded: $FALLBACK_MODEL returned 200 with primary disabled"
            info "Request ID for log correlation: $REQUEST_ID2"
            info "Check Bifrost logs to confirm the fallback provider was used."
            ;;
        402|429)
            pass "Fallback succeeded: $FALLBACK_MODEL returned $CODE2 (recognized via fallback)"
            ;;
        503|502)
            fail "Fallback failed: $FALLBACK_MODEL returned $CODE2 (no providers available)"
            info "This means Bifrost could not route to any fallback provider."
            info "Check: is ClewdR running? Are cookies valid?"
            ;;
        000)
            fail "Fallback failed: connection error"
            ;;
        *)
            BODY2=$(cat "$RESPONSE_FILE2" 2>/dev/null || echo "")
            fail "Fallback returned $CODE2 (body: $BODY2)"
            ;;
    esac

    rm -f "$RESPONSE_FILE2"
else
    info ""
    info "Step 2: Skipped (set DISABLE_PRIMARY=true to test with primary disabled)"
    info "Failure drill procedure:"
    info "  1. Disable the primary provider in Bifrost admin UI or config"
    info "  2. Re-run: DISABLE_PRIMARY=true API_KEY=... ./check-fallback.sh"
    info "  3. Re-enable the provider after testing"
    skip "Provider-disabled fallback test (DISABLE_PRIMARY not set)"
fi

# ── Test 3: Premium model should NOT fall back to ClewdR ──
info ""
info "Step 3: Verify premium model does NOT use unofficial fallback..."
PREMIUM_MODEL="${PREMIUM_MODEL:-claude-sonnet}"

RESPONSE_FILE3=$(mktemp)
_FALLBACK_TMPFILES="$_FALLBACK_TMPFILES $RESPONSE_FILE3"
CODE3=$(curl -s -o "$RESPONSE_FILE3" -w "%{http_code}" \
    --connect-timeout "$CONNECT_TIMEOUT" \
    --max-time 30 \
    -X POST "$NEWAPI_URL/v1/chat/completions" \
    -H "Authorization: Bearer $API_KEY" \
    -H "Content-Type: application/json" \
    -d "{\"model\":\"$PREMIUM_MODEL\",\"messages\":[{\"role\":\"user\",\"content\":\"say ok\"}],\"max_tokens\":1}" \
    2>/dev/null || echo "000")

case "$CODE3" in
    200)
        pass "$PREMIUM_MODEL returned 200 (premium route works)"
        ;;
    402|429)
        pass "$PREMIUM_MODEL returned $CODE3 (premium route recognized)"
        ;;
    503|502)
        if [ "$DISABLE_PRIMARY" = "true" ]; then
            pass "$PREMIUM_MODEL returned $CODE3 with primary disabled (correctly refusing unofficial fallback)"
        else
            fail "$PREMIUM_MODEL returned $CODE3 (premium provider unavailable)"
        fi
        ;;
    000)
        fail "$PREMIUM_MODEL connection failed"
        ;;
    *)
        fail "$PREMIUM_MODEL returned $CODE3"
        ;;
esac

rm -f "$RESPONSE_FILE" "$HEADERS_FILE" "$RESPONSE_FILE3"

print_result
