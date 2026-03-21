#!/usr/bin/env bash
# Master smoke test runner. Executes all smoke checks in order and prints
# a summary table with color-coded results.
#
# REQUIRES: running docker-compose stack
# REQUIRES: NEWAPI_URL set (defaults to http://localhost:3000)
#
# Optional environment:
#   API_KEY       — needed for authenticated tests (model listing, chat, etc.)
#   ADMIN_TOKEN   — needed for admin API tests
#   BIFROST_URL   — internal Bifrost URL (default: http://localhost:8080)
#   CLEWDR_URL    — internal ClewdR URL (default: http://localhost:8484)
#   SERVER_IP     — public IP for isolation checks (default: localhost)
#
# Usage:
#   NEWAPI_URL=https://api.example.com API_KEY=sk-xxx ./run-all.sh
#   NEWAPI_URL=http://localhost:3000 API_KEY=sk-xxx SERVER_IP=203.0.113.5 ./run-all.sh

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

# ── Color codes ───────────────────────────────────────────
if [ -t 1 ]; then
    GREEN='\033[0;32m'
    RED='\033[0;31m'
    YELLOW='\033[0;33m'
    CYAN='\033[0;36m'
    BOLD='\033[1m'
    RESET='\033[0m'
else
    GREEN='' RED='' YELLOW='' CYAN='' BOLD='' RESET=''
fi

# ── Export variables so child scripts inherit them ────────
export NEWAPI_URL="${NEWAPI_URL:-http://localhost:3000}"
export BIFROST_URL="${BIFROST_URL:-http://localhost:8080}"
export CLEWDR_URL="${CLEWDR_URL:-http://localhost:8484}"
export API_KEY="${API_KEY:-}"
export ADMIN_TOKEN="${ADMIN_TOKEN:-}"
export SERVER_IP="${SERVER_IP:-localhost}"
export CONNECT_TIMEOUT="${CONNECT_TIMEOUT:-5}"

printf "${BOLD}========================================${RESET}\n"
printf "${BOLD}  Control Plane Smoke Test Suite${RESET}\n"
printf "${BOLD}========================================${RESET}\n"
printf "  NEWAPI_URL:  %s\n" "$NEWAPI_URL"
printf "  BIFROST_URL: %s\n" "$BIFROST_URL"
printf "  CLEWDR_URL:  %s\n" "$CLEWDR_URL"
printf "  SERVER_IP:   %s\n" "$SERVER_IP"
printf "  API_KEY:     %s\n" "$([ -n "$API_KEY" ] && echo 'set' || echo 'not set')"
printf "${BOLD}========================================${RESET}\n\n"

# ── Define test scripts in execution order ────────────────
TESTS=(
    "check-newapi.sh"
    "check-bifrost.sh"
    "check-clewdr-isolation.sh"
    "check-logical-models.sh"
    "check-correlation.sh"
    "check-fallback.sh"
)

# ── Run each test and collect results ─────────────────────
declare -a RESULTS=()
declare -a NAMES=()
TOTAL_PASS=0
TOTAL_FAIL=0
TOTAL_SKIP=0

for TEST_SCRIPT in "${TESTS[@]}"; do
    TEST_PATH="$SCRIPT_DIR/$TEST_SCRIPT"
    TEST_NAME="${TEST_SCRIPT%.sh}"

    if [ ! -f "$TEST_PATH" ]; then
        printf "${YELLOW}[SKIP]${RESET} %s (file not found)\n\n" "$TEST_NAME"
        RESULTS+=("SKIP")
        NAMES+=("$TEST_NAME")
        ((TOTAL_SKIP++)) || true
        continue
    fi

    echo ""
    if bash "$TEST_PATH"; then
        RESULTS+=("PASS")
        ((TOTAL_PASS++)) || true
    else
        RESULTS+=("FAIL")
        ((TOTAL_FAIL++)) || true
    fi
    NAMES+=("$TEST_NAME")
    echo ""
done

# ── Summary table ─────────────────────────────────────────
printf "\n${BOLD}========================================${RESET}\n"
printf "${BOLD}  Summary${RESET}\n"
printf "${BOLD}========================================${RESET}\n"
printf "  %-30s  %s\n" "Test" "Result"
printf "  %-30s  %s\n" "------------------------------" "------"

for i in "${!NAMES[@]}"; do
    case "${RESULTS[$i]}" in
        PASS)  COLOR="$GREEN" ;;
        FAIL)  COLOR="$RED" ;;
        SKIP)  COLOR="$YELLOW" ;;
        *)     COLOR="$RESET" ;;
    esac
    printf "  %-30s  ${COLOR}%s${RESET}\n" "${NAMES[$i]}" "${RESULTS[$i]}"
done

printf "  %-30s  %s\n" "------------------------------" "------"
printf "  ${GREEN}Passed: %d${RESET}  ${RED}Failed: %d${RESET}  ${YELLOW}Skipped: %d${RESET}\n" \
    "$TOTAL_PASS" "$TOTAL_FAIL" "$TOTAL_SKIP"
printf "${BOLD}========================================${RESET}\n"

if [ "$TOTAL_FAIL" -gt 0 ]; then
    printf "\n${RED}SMOKE TESTS FAILED${RESET} -- review failures above.\n"
    exit 1
else
    printf "\n${GREEN}All smoke tests passed.${RESET}\n"
    exit 0
fi
