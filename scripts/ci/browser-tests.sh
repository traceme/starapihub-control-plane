#!/usr/bin/env bash
# CI wrapper for Playwright browser regression tests.
# Installs dependencies if needed, runs all browser tests, and produces
# JUnit XML + JSON reports in control-plane/artifacts/.
#
# REQUIRES: running docker-compose stack
#
# Environment contract (all vars read by Playwright specs):
#
#   DASHBOARD_URL      — dashboard base URL        (default: http://localhost:8090)
#   DASHBOARD_TOKEN    — dashboard sessionStorage token (default: test-token)
#   NEWAPI_URL         — New-API base URL for admin access (default: http://localhost:3000)
#   GATEWAY_URL        — nginx ingress URL for smoke inference (default: NEWAPI_URL)
#                        Set this to the nginx port (e.g., http://localhost:80) so the
#                        smoke request generates an nginx access log entry for CI-07.
#   API_KEY            — New-API bearer token for smoke inference (REQUIRED for CI-05)
#   ADMIN_USERNAME     — New-API admin login user   (required for CI-08 real auth)
#   ADMIN_PASSWORD     — New-API admin login pass   (required for CI-08 real auth)
#   SMOKE_MODEL        — model name for smoke inference (default: cheap-chat)
#
# If API_KEY is unset, the global-setup smoke inference will fail fast.
# If ADMIN_USERNAME / ADMIN_PASSWORD are unset, New-API admin tests are skipped.
# If GATEWAY_URL is unset, smoke inference uses NEWAPI_URL (may bypass nginx → CI-07 fails).
#
# Usage:
#   GATEWAY_URL=http://localhost:80 API_KEY=sk-xxx ADMIN_USERNAME=admin ADMIN_PASSWORD=secret ./browser-tests.sh

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
TEST_DIR="$(cd "$SCRIPT_DIR/../../tests/browser" && pwd)"
ARTIFACTS_DIR="$(cd "$SCRIPT_DIR/../.." && pwd)/artifacts"

# ── Ensure artifacts directory exists ─────────────────────
mkdir -p "$ARTIFACTS_DIR"

# ── Auto-install dependencies if needed ───────────────────
if [ ! -d "$TEST_DIR/node_modules" ]; then
    echo "Installing Playwright dependencies..."
    (cd "$TEST_DIR" && npm ci)
    (cd "$TEST_DIR" && npx playwright install chromium)
fi

# ── Export environment variables with defaults ────────────
export DASHBOARD_URL="${DASHBOARD_URL:-http://localhost:8090}"
export DASHBOARD_TOKEN="${DASHBOARD_TOKEN:-test-token}"
export NEWAPI_URL="${NEWAPI_URL:-http://localhost:3000}"
export GATEWAY_URL="${GATEWAY_URL:-${NEWAPI_URL}}"
export API_KEY="${API_KEY:-}"
export ADMIN_USERNAME="${ADMIN_USERNAME:-}"
export ADMIN_PASSWORD="${ADMIN_PASSWORD:-}"
export SMOKE_MODEL="${SMOKE_MODEL:-cheap-chat}"

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

# ── Banner ────────────────────────────────────────────────
printf "${BOLD}========================================${RESET}\n"
printf "${BOLD}  Browser Regression Tests${RESET}\n"
printf "${BOLD}========================================${RESET}\n"
printf "  DASHBOARD_URL:    %s\n" "$DASHBOARD_URL"
printf "  NEWAPI_URL:       %s\n" "$NEWAPI_URL"
printf "  GATEWAY_URL:      %s\n" "$GATEWAY_URL"
printf "  DASHBOARD_TOKEN:  %s\n" "$([ -n "$DASHBOARD_TOKEN" ] && echo 'set' || echo 'not set')"
printf "  API_KEY:          %s\n" "$([ -n "$API_KEY" ] && echo 'set' || echo 'NOT SET — global setup will fail')"
printf "  ADMIN_USERNAME:   %s\n" "$([ -n "$ADMIN_USERNAME" ] && echo 'set' || echo 'not set — New-API tests will skip')"
printf "  ADMIN_PASSWORD:   %s\n" "$([ -n "$ADMIN_PASSWORD" ] && echo 'set' || echo 'not set — New-API tests will skip')"
printf "${BOLD}========================================${RESET}\n"

# ── Pre-flight warnings ──────────────────────────────────
if [ -z "$API_KEY" ]; then
    printf "\n${YELLOW}WARNING: API_KEY not set. Global setup smoke inference will fail.${RESET}\n"
    printf "${YELLOW}Set API_KEY to a valid New-API bearer token.${RESET}\n\n"
fi
if [ -z "$ADMIN_USERNAME" ] || [ -z "$ADMIN_PASSWORD" ]; then
    printf "\n${YELLOW}NOTE: ADMIN_USERNAME/ADMIN_PASSWORD not set. New-API admin tests will be skipped.${RESET}\n\n"
fi

# ── Run Playwright tests ─────────────────────────────────
cd "$TEST_DIR"
set +e
npx playwright test "$@"
EXIT_CODE=$?
set -e

# ── Report file locations ────────────────────────────────
printf "\n${BOLD}Reports:${RESET}\n"
if [ -f "$ARTIFACTS_DIR/browser-results.xml" ]; then
    printf "  ${CYAN}JUnit XML:${RESET} %s\n" "$ARTIFACTS_DIR/browser-results.xml"
else
    printf "  ${RED}JUnit XML:${RESET} not generated\n"
fi
if [ -f "$ARTIFACTS_DIR/browser-results.json" ]; then
    printf "  ${CYAN}JSON:${RESET}      %s\n" "$ARTIFACTS_DIR/browser-results.json"
else
    printf "  ${RED}JSON:${RESET}      not generated\n"
fi

# ── Summary ───────────────────────────────────────────────
printf "\n${BOLD}========================================${RESET}\n"
if [ "$EXIT_CODE" -eq 0 ]; then
    printf "${GREEN}All browser tests passed.${RESET}\n"
else
    printf "${RED}BROWSER TESTS FAILED${RESET} (exit code: %d)\n" "$EXIT_CODE"
fi
printf "${BOLD}========================================${RESET}\n"

exit $EXIT_CODE
