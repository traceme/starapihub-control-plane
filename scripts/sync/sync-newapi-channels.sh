#!/usr/bin/env bash
# Sync New-API channels from the control plane's policy definitions.
#
# Creates or updates Bifrost-facing channels in New-API via its admin API.
# Implements the channel strategy from the control plane policies:
#   - bifrost-premium:  official providers only, priority 0 (highest)
#   - bifrost-standard: official preferred + unofficial fallback, priority 5
#   - bifrost-risky:    ClewdR primary, priority 10 (lowest)
#
# New-API admin API endpoints used:
#   POST /api/channel/       — create a new channel
#   GET  /api/channel/       — list existing channels (future: update support)
#   PUT  /api/channel/       — update an existing channel (requires id)
#
# REQUIRES: running New-API instance
# REQUIRES: ADMIN_TOKEN set in environment (obtain from New-API admin UI -> personal settings)
# REQUIRES: NEWAPI_URL set in environment (defaults to http://localhost:3000)
#
# Usage:
#   NEWAPI_URL=http://localhost:3000 ADMIN_TOKEN=<token> ./sync-newapi-channels.sh
#   NEWAPI_URL=http://localhost:3000 ADMIN_TOKEN=<token> DRY_RUN=true ./sync-newapi-channels.sh

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "$SCRIPT_DIR/../.." && pwd)"

# ── Color codes ───────────────────────────────────────────
if [ -t 1 ]; then
    GREEN='\033[0;32m'
    RED='\033[0;31m'
    YELLOW='\033[0;33m'
    CYAN='\033[0;36m'
    RESET='\033[0m'
else
    GREEN='' RED='' YELLOW='' CYAN='' RESET=''
fi

pass()  { printf "${GREEN}  [OK]${RESET}   %s\n" "$1"; }
fail()  { printf "${RED}  [FAIL]${RESET} %s\n" "$1"; }
info()  { printf "${CYAN}  [INFO]${RESET} %s\n" "$1"; }
warn()  { printf "${YELLOW}  [WARN]${RESET} %s\n" "$1"; }

# ── Required variables ───────────────────────────────────
NEWAPI_URL="${NEWAPI_URL:?Set NEWAPI_URL (e.g., http://localhost:3000)}"
ADMIN_TOKEN="${ADMIN_TOKEN:?Set ADMIN_TOKEN (obtain from New-API admin UI -> personal settings)}"
BIFROST_URL="${BIFROST_URL:-http://bifrost:8080}"
CHANNEL_KEY="${CHANNEL_KEY:-}"
DRY_RUN="${DRY_RUN:-false}"

echo "=== Sync New-API Channels ==="
info "New-API:     $NEWAPI_URL"
info "Bifrost URL: $BIFROST_URL (used as channel base_url)"
info "Channel key: ${CHANNEL_KEY:+(set)}"
info "Dry run:     $DRY_RUN"
echo ""

# ── Verify New-API is reachable ──────────────────────────
CODE=$(curl -s -o /dev/null -w "%{http_code}" --connect-timeout 5 \
    "$NEWAPI_URL/api/status" 2>/dev/null || echo "000")
if [ "$CODE" != "200" ]; then
    fail "New-API not reachable at $NEWAPI_URL/api/status (HTTP $CODE)"
    exit 1
fi
pass "New-API is reachable"

# ── Verify admin token ───────────────────────────────────
CODE=$(curl -s -o /dev/null -w "%{http_code}" --connect-timeout 5 \
    "$NEWAPI_URL/api/channel/?p=0&page_size=1" \
    -H "Authorization: Bearer $ADMIN_TOKEN" \
    2>/dev/null || echo "000")
if [ "$CODE" = "401" ] || [ "$CODE" = "403" ]; then
    fail "ADMIN_TOKEN is invalid or lacks permission (HTTP $CODE)"
    exit 1
fi
pass "Admin token is valid"
echo ""

# ── Channel creation helper ──────────────────────────────
CREATED=0
FAILED=0

create_channel() {
    local name="$1"
    local models="$2"
    local mapping_json="$3"
    local priority="$4"

    local payload
    payload=$(cat <<EOF
{
    "name": "$name",
    "type": 1,
    "base_url": "$BIFROST_URL",
    "key": "$CHANNEL_KEY",
    "models": "$models",
    "model_mapping": "$mapping_json",
    "priority": $priority,
    "weight": 1
}
EOF
)

    info "Channel: $name"
    info "  Models: $models"
    info "  Priority: $priority"

    if [ "$DRY_RUN" = "true" ]; then
        warn "  DRY RUN: would POST to $NEWAPI_URL/api/channel/"
        echo "  Payload: $payload"
        return 0
    fi

    local response_file
    response_file=$(mktemp)

    local code
    code=$(curl -s -o "$response_file" -w "%{http_code}" \
        --connect-timeout 10 \
        -X POST "$NEWAPI_URL/api/channel/" \
        -H "Authorization: Bearer $ADMIN_TOKEN" \
        -H "Content-Type: application/json" \
        -d "$payload" \
        2>/dev/null || echo "000")

    if [ "$code" = "200" ] || [ "$code" = "201" ]; then
        pass "  Channel '$name' created (HTTP $code)"
        ((CREATED++)) || true
    else
        fail "  Channel '$name' failed (HTTP $code)"
        local body
        body=$(cat "$response_file" 2>/dev/null || echo "(no body)")
        info "  Response: $body"
        ((FAILED++)) || true
    fi

    rm -f "$response_file"
}

# ── Channel definitions ──────────────────────────────────
# These match the logical model registry in policies/logical-models.example.yaml.
# Model mapping JSON translates logical names to upstream model identifiers.

info "--- Creating channels ---"
echo ""

# Premium channel: official providers only
# Model mapping uses provider/model format required by Bifrost (e.g., clewdr-1/claude-sonnet-4-20250514)
create_channel "bifrost-premium" \
    "claude-sonnet,claude-opus,claude-haiku,gpt-4o,gpt-4o-mini" \
    "{\"claude-sonnet\":\"clewdr-1/claude-sonnet-4-20250514\",\"claude-opus\":\"clewdr-1/claude-opus-4-20250514\",\"claude-haiku\":\"claude-haiku-4-5-20251001\",\"gpt-4o\":\"gpt-4o\",\"gpt-4o-mini\":\"gpt-4o-mini\"}" \
    0

echo ""

# Standard channel: official preferred, unofficial fallback
create_channel "bifrost-standard" \
    "cheap-chat,fast-chat" \
    "{\"cheap-chat\":\"clewdr-1/claude-sonnet-4-20250514\",\"fast-chat\":\"claude-haiku-4-5-20251001\"}" \
    5

echo ""

# Risky channel: ClewdR primary
create_channel "bifrost-risky" \
    "lab-claude,lab-claude-opus" \
    "{\"lab-claude\":\"clewdr-1/claude-sonnet-4-20250514\",\"lab-claude-opus\":\"clewdr-1/claude-opus-4-20250514\"}" \
    10

# ── Summary ──────────────────────────────────────────────
echo ""
echo "=== Summary ==="
printf "  Created: %d\n" "$CREATED"
printf "  Failed:  %d\n" "$FAILED"
echo ""

if [ "$FAILED" -gt 0 ]; then
    fail "Some channels failed to create. Check errors above."
    echo ""
    warn "If channels already exist, New-API may reject duplicate names."
    warn "Use the admin UI or PUT /api/channel/{id} to update existing channels."
    exit 1
fi

pass "Channel sync complete."
echo ""
info "Next steps:"
info "  1. Verify channels in New-API admin UI ($NEWAPI_URL)"
info "  2. Set model pricing:  Admin UI -> Settings -> Operation -> Model Pricing"
info "  3. Configure user groups: Admin UI -> Users/Groups"
info "  4. Run smoke tests: bash scripts/smoke/run-all.sh"
