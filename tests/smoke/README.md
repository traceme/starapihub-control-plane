# Smoke Tests

## Overview

Smoke tests verify that the deployed control plane stack is functioning correctly. They are lightweight checks designed to run in under 60 seconds and catch the most common deployment and configuration problems.

## Coverage

| Test Script | What It Verifies | PRD Reference |
|-------------|-----------------|---------------|
| `check-newapi.sh` | New-API health, /api/status, /v1/models, chat completions | 9.1 |
| `check-bifrost.sh` | Bifrost internal health, public isolation | 9.1 |
| `check-clewdr-isolation.sh` | ClewdR not reachable from public network | 9.1, 9.5 |
| `check-logical-models.sh` | Logical model names resolve through New-API | 9.2, 12 |
| `check-correlation.sh` | X-Request-ID propagates through the stack | 9.7 |
| `check-fallback.sh` | Fallback behavior when primary provider is down | 9.4, 14.8 |

## Prerequisites

- Docker Compose stack is running (`cd deploy && docker-compose up -d`)
- `curl` is installed on the test runner
- Environment variables are set (see below)

## Environment Variables

| Variable | Required | Default | Description |
|----------|----------|---------|-------------|
| `NEWAPI_URL` | For most tests | `http://localhost:3000` | New-API base URL |
| `API_KEY` | For auth tests | (none) | User API key for New-API |
| `BIFROST_URL` | For Bifrost tests | `http://localhost:8080` | Internal Bifrost URL |
| `CLEWDR_URL` | For ClewdR tests | `http://localhost:8484` | Internal ClewdR URL |
| `SERVER_IP` | For isolation tests | `localhost` | Public-facing IP of the host |
| `ADMIN_TOKEN` | For admin tests | (none) | New-API admin bearer token |
| `CONNECT_TIMEOUT` | No | `5` | HTTP connection timeout in seconds |

## Running Tests

### Full Suite

```bash
NEWAPI_URL=https://api.example.com \
API_KEY=sk-test-xxx \
SERVER_IP=203.0.113.5 \
  bash control-plane/scripts/smoke/run-all.sh
```

### Individual Tests

```bash
# New-API health only
NEWAPI_URL=https://api.example.com bash scripts/smoke/check-newapi.sh

# Bifrost (from a host on the core network)
BIFROST_URL=http://bifrost:8080 bash scripts/smoke/check-bifrost.sh

# ClewdR isolation (needs public IP)
SERVER_IP=203.0.113.5 bash scripts/smoke/check-clewdr-isolation.sh

# Logical model resolution
NEWAPI_URL=https://api.example.com API_KEY=sk-xxx bash scripts/smoke/check-logical-models.sh

# Correlation header
NEWAPI_URL=https://api.example.com API_KEY=sk-xxx bash scripts/smoke/check-correlation.sh

# Fallback (basic)
NEWAPI_URL=https://api.example.com API_KEY=sk-xxx bash scripts/smoke/check-fallback.sh

# Fallback (with primary disabled for failure drill)
NEWAPI_URL=https://api.example.com API_KEY=sk-xxx DISABLE_PRIMARY=true bash scripts/smoke/check-fallback.sh
```

## Output Format

Each test uses color-coded output:

- **[PASS]** (green) -- check succeeded
- **[FAIL]** (red) -- check failed, needs attention
- **[SKIP]** (yellow) -- check skipped due to missing prerequisites
- **[INFO]** (cyan) -- informational message

`run-all.sh` prints a summary table at the end and exits with code 1 if any test failed.

## When to Run

- After initial deployment
- After any configuration change
- After upgrading any upstream component (New-API, Bifrost, ClewdR)
- After running sync scripts
- As part of a failure drill
- On a regular schedule for production health monitoring (daily or weekly)

## Adding New Tests

1. Create a new script in `scripts/smoke/` following the naming convention `check-<thing>.sh`
2. Source `_helpers.sh` for color output and HTTP helpers
3. Use `pass`, `fail`, `skip`, and `info` functions
4. Exit 0 on success, 1 on failure (the `print_result` helper does this)
5. Add the script name to the `TESTS` array in `run-all.sh`
6. Update this README
