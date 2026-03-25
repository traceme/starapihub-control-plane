# CI Guide — Reading and Acting on CI Results

## Workflows

| Workflow | Trigger | Runner | What it checks | Gates merge? |
|----------|---------|--------|----------------|-------------|
| **CI** (`ci.yml`) | Push/PR to `main` | GitHub-hosted (lint/test), self-hosted (browser) | Go vet, unit tests, CLI build, browser regression | **Lint + unit tests gate PRs.** Browser regression is post-merge only (requires live stack). |
| **Nightly** (`nightly.yml`) | 03:00 UTC daily + manual | Self-hosted | Health, drift, cookies, smoke inference, browser regression | No — detects environment breakage |
| **Release** (`release.yml`) | Manual dispatch | Self-hosted | Full RC validation + image build + registry push + tag | No — produces versioned release artifacts |

## Smoke Test Paths

There are **two distinct smoke paths** with different URLs and different purposes. They are not interchangeable.

### 1. Direct New-API Smoke (`run-all.sh`)

- **Script:** `control-plane/scripts/smoke/run-all.sh`
- **URL variable:** `NEWAPI_URL` (default: `http://localhost:3000`)
- **Traffic path:** `curl → New-API:3000 → Bifrost → ClewdR`
- **What it proves:** New-API accepts requests, Bifrost routes them, ClewdR responds, models resolve, request-IDs correlate, fallback works
- **What it does NOT prove:** nginx proxying works, nginx access logs are written
- **Used by:** Nightly workflow (step: "Smoke tests (curl-based)")

### 2. Gateway-Path Smoke (`global-setup.ts`)

- **Script:** `control-plane/tests/browser/global-setup.ts` (Playwright global setup)
- **URL variable:** `GATEWAY_URL` (default: falls back to `NEWAPI_URL`)
- **Traffic path:** `fetch → nginx:80 → New-API → Bifrost → ClewdR`
- **What it proves:** The full ingress path works, nginx writes an access log entry
- **Why it matters:** The dashboard Logs page (CI-07) reads from the nginx access log. If the smoke request bypasses nginx, no log entry exists, and CI-07 fails.
- **Used by:** CI workflow (post-merge browser regression), Nightly workflow (browser tests)

### URL Summary

| Variable | Default | Points to | Used by |
|----------|---------|-----------|---------|
| `NEWAPI_URL` | `http://localhost:3000` | New-API direct | `run-all.sh`, `newapi-admin.spec.ts`, `loginNewApi()` |
| `GATEWAY_URL` | Falls back to `NEWAPI_URL` | Nginx ingress (port 80) | `global-setup.ts` smoke inference |
| `DASHBOARD_URL` | `http://localhost:8090` | Dashboard | `dashboard.spec.ts` |
| `BIFROST_URL` | `http://localhost:8080` | Bifrost direct | `run-all.sh` health checks |
| `CLEWDR_URL` | `http://localhost:8484` | ClewdR direct | `run-all.sh` isolation checks |

## Secrets and Variables

### Secrets (sensitive — GitHub repository settings → Secrets)

| Secret | Used by | Purpose |
|--------|---------|---------|
| `DASHBOARD_TOKEN` | CI, Nightly, Release | Dashboard Bearer auth token (must match container env) |
| `API_KEY` | CI, Nightly, Release | New-API bearer token for smoke inference (CI-05) |
| `ADMIN_USERNAME` | CI, Nightly, Release | New-API admin login user (CI-08) |
| `ADMIN_PASSWORD` | CI, Nightly, Release | New-API admin login password (CI-08) |
| `NEWAPI_ADMIN_TOKEN` | Release | New-API admin access_token for rc-validate |
| `ADMIN_TOKEN` | Nightly | New-API admin token for curl-based smoke scripts |
| `REGISTRY_USERNAME` | Release | Container registry login |
| `REGISTRY_PASSWORD` | Release | Container registry password/token |
| `DASHBOARD_URL` | CI, Nightly, Release | Dashboard base URL |
| `NEWAPI_URL` | CI, Nightly, Release | New-API direct URL |
| `GATEWAY_URL` | CI, Nightly, Release | Nginx ingress URL for gateway-path smoke |
| `BIFROST_URL` | Nightly, Release | Bifrost direct URL |
| `CLEWDR_URL` | Nightly | ClewdR direct URL |

### Variables (non-sensitive — GitHub repository settings → Variables)

| Variable | Used by | Purpose |
|----------|---------|---------|
| `SMOKE_MODEL` | CI, Nightly, Release | Model name for smoke inference (default: `cheap-chat`) |
| `IMAGE_REGISTRY` | Release | Container registry (default: `docker.io`) |
| `IMAGE_NAMESPACE` | Release | Image namespace (default: `starapihub`) |

## Self-Hosted Runner Requirements

1. A running Docker Compose stack (New-API, Bifrost, ClewdR, nginx, dashboard)
2. Network access to all service ports (80, 3000, 8080, 8090)
3. Node.js 20+ (for Playwright)
4. Go 1.22+ (for CLI build)
5. Docker CLI with push access to the configured registry (for release workflow)
6. Chromium (installed automatically by Playwright on first run)

## Reading CI Failures

### Lint + Unit Tests (PR gate — GitHub-hosted)

These run without infrastructure. Failures mean:
- `go vet` found a code issue → fix the Go code
- `go test` failed → check test output, fix the test or the code

### Browser Regression (post-merge — self-hosted)

| Symptom | Likely cause | Fix |
|---------|-------------|-----|
| Global setup: "API_KEY env var is required" | `API_KEY` secret not set | Add the secret |
| Global setup: "Smoke inference failed with status 400" | `SMOKE_MODEL` has no Bifrost model_mapping | Set `SMOKE_MODEL` to a mapped model (e.g., `cheap-chat`) |
| Dashboard tests: 403 Forbidden | `DASHBOARD_TOKEN` mismatch | Match secret to container env |
| New-API tests: all skipped | `ADMIN_USERNAME`/`ADMIN_PASSWORD` not set | Add the secrets |
| New-API tests: redirect to /login | Wrong credentials or 2FA enabled | Check creds, disable 2FA on CI admin |
| Cookie test timeout (20s) | No ClewdR instances reporting | Verify ClewdR containers are healthy |
| Logs test timeout (15s) | Smoke request bypassed nginx | Set `GATEWAY_URL` to `http://localhost:80` |

### Nightly Failures

All nightly steps fail the job — nothing is silently swallowed.

| Step | What broke | Likely cause |
|------|-----------|-------------|
| Health check | A service is down | Container crashed or OOM |
| Drift check | Config diverged from desired state | Manual change or upstream update |
| Cookie status | Cookie count dropped | Cookie expired or ClewdR restarted |
| Smoke tests (curl) | Direct New-API inference path broken | New-API, Bifrost, or ClewdR issue |
| Browser tests | Dashboard or admin regression | UI or auth change |

### Release Failures

| Step | Meaning |
|------|---------|
| Registry login failed | `REGISTRY_USERNAME`/`REGISTRY_PASSWORD` wrong or expired |
| Image push failed | Registry unreachable or quota exceeded |
| Tag push failed | Tag already exists or insufficient permissions |

## Artifacts

**CI** (30-day retention):
- `browser-results.xml` — JUnit XML
- `browser-results.json` — JSON detail

**Nightly** (14-day retention):
- `nightly/health.log`, `nightly/drift.log`, `nightly/cookies.log`, `nightly/smoke.log`
- `browser-results.xml` + `browser-results.json`

**Release** (90-day retention):
- `releases/<version>/release-manifest.json` — version, mode, pushed image digests, CLI sha256, workflow link
- `releases/<version>/<timestamp>/` — full RC validation evidence
- `release-binaries/starapihub-<version>-<os>-<arch>` — stamped CLI binary
- `browser-results.xml` + `browser-results.json`
