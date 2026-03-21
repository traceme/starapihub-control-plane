#!/usr/bin/env bash
# Smoke test: Verify that logical model names resolve through New-API.
#
# Sends a minimal chat-completions request for each logical model name defined
# in the control plane registry. A successful resolution means New-API accepted
# the model name and routed it (even if the downstream provider returns an error
# due to missing keys, the model name itself was recognized).
#
# REQUIRES: running docker-compose stack
# REQUIRES: API_KEY set in environment
# REQUIRES: NEWAPI_URL set in environment (defaults to http://localhost:3000)
#
# Usage:
#   NEWAPI_URL=https://api.example.com API_KEY=sk-xxx ./check-logical-models.sh
#   NEWAPI_URL=https://api.example.com API_KEY=sk-xxx MODELS="claude-sonnet cheap-chat" ./check-logical-models.sh

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
# shellcheck source=_helpers.sh
source "$SCRIPT_DIR/_helpers.sh"

echo "=== Smoke: Logical Model Resolution ==="
info "Target: $NEWAPI_URL"

if [ -z "$API_KEY" ]; then
    skip "API_KEY is required for logical model tests"
    print_result
    exit 0
fi

# Models to test. Override with MODELS env var, or test the full registry set.
DEFAULT_MODELS="claude-sonnet claude-opus claude-haiku gpt-4o gpt-4o-mini cheap-chat fast-chat lab-claude"
MODELS="${MODELS:-$DEFAULT_MODELS}"

info "Testing models: $MODELS"
echo ""

RESPONSE_FILE=""
trap 'rm -f "$RESPONSE_FILE"' EXIT

for MODEL in $MODELS; do
    # Send a minimal request — max_tokens=1 to minimize cost/latency.
    RESPONSE_FILE=$(mktemp)
    CODE=$(curl -s -o "$RESPONSE_FILE" -w "%{http_code}" \
        --connect-timeout "$CONNECT_TIMEOUT" \
        -X POST "$NEWAPI_URL/v1/chat/completions" \
        -H "Authorization: Bearer $API_KEY" \
        -H "Content-Type: application/json" \
        -d "{\"model\":\"$MODEL\",\"messages\":[{\"role\":\"user\",\"content\":\"hi\"}],\"max_tokens\":1}" \
        2>/dev/null || echo "000")

    case "$CODE" in
        200)
            pass "$MODEL -> 200 (routed and completed)"
            ;;
        402|429)
            # Auth worked, model was recognized, just hit quota/rate limit
            pass "$MODEL -> $CODE (model recognized, quota/rate limit)"
            ;;
        400)
            # Bad request could mean the model mapping produced an invalid
            # downstream model name — worth investigating but not a routing failure
            BODY=$(cat "$RESPONSE_FILE" 2>/dev/null || echo "")
            if echo "$BODY" | grep -qi "model"; then
                fail "$MODEL -> 400 (model mapping may be incorrect: $BODY)"
            else
                fail "$MODEL -> 400 (bad request: $BODY)"
            fi
            ;;
        404)
            fail "$MODEL -> 404 (model not found — channel/model mapping missing)"
            ;;
        401)
            fail "$MODEL -> 401 (API_KEY invalid)"
            ;;
        000)
            fail "$MODEL -> connection failed (New-API unreachable)"
            ;;
        *)
            BODY=$(cat "$RESPONSE_FILE" 2>/dev/null || echo "")
            fail "$MODEL -> $CODE (unexpected: $BODY)"
            ;;
    esac

    rm -f "$RESPONSE_FILE"
done

print_result
