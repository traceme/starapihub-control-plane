# CI Guide — Reading and Acting on CI Results

## Workflows

| Workflow | Trigger | Runner | What it checks | Gates merge? |
|----------|---------|--------|----------------|-------------|
| **CI** (`ci.yml`) | Push/PR to `main` | GitHub-hosted (lint/test), self-hosted (browser) | Go vet, unit tests, CLI build, browser regression | **Lint + unit tests gate PRs.** Browser regression is post-merge only (requires live stack). |
| **Nightly** (`nightly.yml`) | 03:00 UTC daily + manual | Self-hosted | Health, drift, cookies, smoke inference, browser regression | No — detects environment breakage |
| **Release** (`release.yml`) | Manual dispatch | Self-hosted | Full RC validation + image build + tag + publish | No — produces versioned release artifacts |

## Environment Variables

### Secrets (sensitive — set in GitHub repository settings)

| Secret | Used by | Purpose |
|--------|---------|---------|
| `DASHBOARD_URL` | CI, Nightly, Release | Dashboard base URL (default: `http://localhost:8090`) |
| `DASHBOARD_TOKEN` | CI, Nightly, Release | Dashboard Bearer auth token (must match container env) |
| `NEWAPI_URL` | CI, Nightly, Release | New-API base URL for admin/API access (default: `http://localhost:3000`) |
| `GATEWAY_URL` | CI, Nightly, Release | Nginx ingress URL for smoke inference — **must route through nginx** so CI-07 gateway log assertions work (default: falls back to `NEWAPI_URL`) |
| `API_KEY` | CI, Nightly, Release | New-API bearer token for smoke inference (CI-05) |
| `ADMIN_USERNAME` | CI, Nightly, Release | New-API admin login user (CI-08) |
| `ADMIN_PASSWORD` | CI, Nightly, Release | New-API admin login password (CI-08) |
| `NEWAPI_ADMIN_TOKEN` | Release | New-API admin access_token for rc-validate |
| `ADMIN_TOKEN` | Nightly | New-API admin token for curl-based smoke scripts |
| `BIFROST_URL` | Nightly, Release | Bifrost URL (default: `http://localhost:8080`) |
| `CLEWDR_URL` | Nightly | ClewdR URL (default: `http://localhost:8484`) |

### Variables (non-sensitive — set in GitHub repository variables)

| Variable | Used by | Purpose |
|----------|---------|---------|
| `SMOKE_MODEL` | CI, Nightly, Release | Model name for smoke inference (default: `cheap-chat`) |

## URL Semantics

Two distinct URLs control where smoke inference traffic goes:

| Variable | Points to | Purpose |
|----------|-----------|---------|
| `NEWAPI_URL` | New-API direct (port 3000) | Admin API access, programmatic login, New-API admin page tests |
| `GATEWAY_URL` | Nginx ingress (port 80) | Smoke inference — request passes through nginx so an access log entry is written for CI-07 |

If `GATEWAY_URL` is unset, it falls back to `NEWAPI_URL`. This means CI-07 (dashboard logs assertion) will fail unless `GATEWAY_URL` explicitly points at the nginx port.

## Self-Hosted Runner Requirements

The browser regression and nightly jobs run on `self-hosted` runners because they need:

1. A running Docker Compose stack (New-API, Bifrost, ClewdR, nginx, dashboard)
2. Network access to all service ports (80, 3000, 8080, 8090)
3. Node.js 20+ (for Playwright)
4. Go 1.22+ (for CLI build)
5. Chromium (installed automatically by Playwright on first run)

## Reading CI Failures

### Lint + Unit Tests (PR gate — GitHub-hosted)

These run without any infrastructure. Failures mean:

- `go vet` found a code issue → fix the Go code
- `go test` failed → check test output, fix the test or the code

### Browser Regression (post-merge — self-hosted)

Failures here usually mean one of:

| Symptom | Likely cause | Fix |
|---------|-------------|-----|
| Global setup error: "API_KEY env var is required" | `API_KEY` secret not set | Add the secret |
| Global setup error: "Smoke inference failed with status 400" | `SMOKE_MODEL` doesn't have a Bifrost model_mapping | Set `SMOKE_MODEL` to a mapped model (e.g., `cheap-chat`) |
| Dashboard tests: 403 Forbidden | `DASHBOARD_TOKEN` doesn't match the running dashboard | Update the secret to match `DASHBOARD_TOKEN` env in the dashboard container |
| New-API tests: all skipped | `ADMIN_USERNAME`/`ADMIN_PASSWORD` not set | Add the secrets |
| New-API tests: redirect to /login | Login failed — wrong credentials or 2FA enabled | Check credentials, disable 2FA on CI admin user |
| Cookie test timeout (20s) | No ClewdR instances reporting cookies | Verify ClewdR containers are healthy |
| Logs test timeout (15s) | No log entries after smoke inference | Set `GATEWAY_URL` to nginx port (e.g., `http://localhost:80`) — request must go through nginx to generate a log entry |

### Nightly Failures

The nightly job runs health, drift, cookie, smoke, and browser checks. All steps now **fail the job** (no silent `|| true` on drift or cookies). If nightly fails but CI passed earlier, something changed in the running environment.

Check the step that failed:
- **Health check** → a service is down
- **Drift check** → config diverged from desired state
- **Cookie status** → ClewdR cookie count dropped below threshold
- **Smoke tests** → inference path broken
- **Browser tests** → dashboard or New-API admin issue

### Artifacts

**CI** uploads (30-day retention):
- `browser-results.xml` — JUnit XML
- `browser-results.json` — JSON detail

**Nightly** uploads (14-day retention):
- `nightly/health.log` — health check output
- `nightly/drift.log` — drift detection output
- `nightly/cookies.log` — cookie status output
- `nightly/smoke.log` — curl-based smoke test output
- `browser-results.xml` + `browser-results.json`

**Release** uploads (90-day retention):
- `releases/<version>/release-manifest.json` — version, mode, image digests, CLI sha256
- `releases/<version>/<timestamp>/` — full RC validation evidence
- `release-binaries/starapihub-<version>-<os>-<arch>` — stamped CLI binary
- `browser-results.xml` + `browser-results.json`
