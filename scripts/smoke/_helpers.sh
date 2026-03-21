#!/usr/bin/env bash
# Shared helper functions for smoke tests.
# Source this file; do not execute it directly.
#
# Provides:
#   - Color-coded output: pass, fail, skip, info
#   - Consistent variable defaults
#   - HTTP helper: http_code_of <method> <url> [extra curl args...]

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

# ── Counters (exported so run-all.sh can aggregate) ───────
_PASS=0
_FAIL=0
_SKIP=0

pass()  { printf "${GREEN}  [PASS]${RESET} %s\n" "$1"; ((_PASS++)) || true; }
fail()  { printf "${RED}  [FAIL]${RESET} %s\n" "$1"; ((_FAIL++)) || true; }
skip()  { printf "${YELLOW}  [SKIP]${RESET} %s\n" "$1"; ((_SKIP++)) || true; }
info()  { printf "${CYAN}  [INFO]${RESET} %s\n" "$1"; }

# ── Default variables ─────────────────────────────────────
NEWAPI_URL="${NEWAPI_URL:-http://localhost:3000}"
BIFROST_URL="${BIFROST_URL:-http://localhost:8080}"
CLEWDR_URL="${CLEWDR_URL:-http://localhost:8484}"
API_KEY="${API_KEY:-}"
ADMIN_TOKEN="${ADMIN_TOKEN:-}"
SERVER_IP="${SERVER_IP:-localhost}"
CONNECT_TIMEOUT="${CONNECT_TIMEOUT:-5}"

# ── HTTP helper ───────────────────────────────────────────
# Usage: http_code_of GET https://example.com/path
#        http_code_of POST https://example.com/path -H "Content-Type: application/json" -d '{}'
http_code_of() {
    local method="$1"; shift
    local url="$1"; shift
    curl -s -o /dev/null -w "%{http_code}" \
        -X "$method" \
        --connect-timeout "$CONNECT_TIMEOUT" \
        "$url" "$@" 2>/dev/null || echo "000"
}

# Usage: http_body GET https://example.com/path [extra curl args...]
http_body() {
    local method="$1"; shift
    local url="$1"; shift
    curl -s -X "$method" --connect-timeout "$CONNECT_TIMEOUT" "$url" "$@" 2>/dev/null || echo ""
}

# ── Summary helper (for individual scripts) ───────────────
print_result() {
    local script_name="${1:-$(basename "$0")}"
    if [ "$_FAIL" -gt 0 ]; then
        printf "\n${RED}%s: %d passed, %d failed, %d skipped${RESET}\n" "$script_name" "$_PASS" "$_FAIL" "$_SKIP"
        return 1
    else
        printf "\n${GREEN}%s: %d passed, %d failed, %d skipped${RESET}\n" "$script_name" "$_PASS" "$_FAIL" "$_SKIP"
        return 0
    fi
}
