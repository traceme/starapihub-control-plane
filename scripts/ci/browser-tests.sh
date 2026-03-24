#!/usr/bin/env bash
# CI wrapper for Playwright browser regression tests.
# Installs dependencies if needed, runs all browser tests, and produces
# JUnit XML + JSON reports in control-plane/artifacts/.
#
# REQUIRES: running docker-compose stack
#
# Optional environment:
#   DASHBOARD_URL   — dashboard base URL   (default: http://localhost:8090)
#   DASHBOARD_TOKEN — dashboard auth token  (default: test-token)
#   NEWAPI_URL      — New-API base URL      (default: http://localhost:3000)
#   ADMIN_TOKEN     — New-API admin token   (default: empty)
#
# Usage:
#   ./browser-tests.sh
#   DASHBOARD_URL=https://dash.example.com NEWAPI_URL=https://api.example.com ./browser-tests.sh

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
export ADMIN_TOKEN="${ADMIN_TOKEN:-}"

# ── Color codes ───────────────────────────────────────────
if [ -t 1 ]; then
    GREEN='\033[0;32m'
    RED='\033[0;31m'
    CYAN='\033[0;36m'
    BOLD='\033[1m'
    RESET='\033[0m'
else
    GREEN='' RED='' CYAN='' BOLD='' RESET=''
fi

# ── Banner ────────────────────────────────────────────────
printf "${BOLD}========================================${RESET}\n"
printf "${BOLD}  Browser Regression Tests${RESET}\n"
printf "${BOLD}========================================${RESET}\n"
printf "  DASHBOARD_URL:   %s\n" "$DASHBOARD_URL"
printf "  NEWAPI_URL:      %s\n" "$NEWAPI_URL"
printf "  DASHBOARD_TOKEN: %s\n" "$([ -n "$DASHBOARD_TOKEN" ] && echo 'set' || echo 'not set')"
printf "  ADMIN_TOKEN:     %s\n" "$([ -n "$ADMIN_TOKEN" ] && echo 'set' || echo 'not set')"
printf "${BOLD}========================================${RESET}\n\n"

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
