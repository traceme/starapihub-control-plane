# CI Guide — Reading and Acting on CI Results

## Workflows

| Workflow | Trigger | Runner | What it checks |
|----------|---------|--------|----------------|
| **CI** (`ci.yml`) | Push/PR to `main` | GitHub-hosted (lint/test), self-hosted (browser) | Go vet, unit tests, CLI build, browser regression |
| **Nightly** (`nightly.yml`) | 03:00 UTC daily + manual | Self-hosted | Health, drift, cookies, smoke inference, browser regression |
| **Release** (`release.yml`) | Manual dispatch | Self-hosted | Full RC validation + image build + tag + GitHub release |

## Environment Variables

All workflows that touch the live stack require these secrets in the repository:

| Secret | Used by | Purpose |
|--------|---------|---------|
| `DASHBOARD_URL` | CI, Nightly, Release | Dashboard base URL (default: `http://localhost:8090`) |
| `DASHBOARD_TOKEN` | CI, Nightly, Release | Dashboard Bearer auth token |
| `NEWAPI_URL` | CI, Nightly, Release | New-API base URL (default: `http://localhost:3000`) |
| `API_KEY` | CI, Nightly, Release | New-API bearer token for smoke inference (CI-05) |
| `ADMIN_USERNAME` | CI, Nightly, Release | New-API admin login user (CI-08) |
| `ADMIN_PASSWORD` | CI, Nightly, Release | New-API admin login password (CI-08) |
| `SMOKE_MODEL` | CI, Nightly, Release | Model for smoke inference (default: `cheap-chat`) |
| `NEWAPI_ADMIN_TOKEN` | Release | New-API admin access_token for rc-validate |
| `ADMIN_TOKEN` | Nightly | New-API admin token for smoke scripts |
| `BIFROST_URL` | Nightly, Release | Bifrost URL (default: `http://localhost:8080`) |
| `CLEWDR_URL` | Nightly | ClewdR URL (default: `http://localhost:8484`) |

## Self-Hosted Runner Requirements

The browser regression and nightly jobs run on `self-hosted` runners because they need:

1. A running Docker Compose stack (New-API, Bifrost, ClewdR, nginx, dashboard)
2. Network access to all service ports
3. Node.js 20+ (for Playwright)
4. Go 1.22+ (for CLI build)
5. Chromium (installed automatically by Playwright on first run)

## Reading CI Failures

### Lint + Unit Tests (GitHub-hosted)

These run without any infrastructure. Failures mean:

- `go vet` found a code issue → fix the Go code
- `go test` failed → check test output, fix the test or the code

### Browser Regression (self-hosted)

Failures here usually mean one of:

| Symptom | Likely cause | Fix |
|---------|-------------|-----|
| Global setup error: "API_KEY env var is required" | `API_KEY` secret not set | Add the secret |
| Global setup error: "Smoke inference failed with status 400" | `SMOKE_MODEL` doesn't have a Bifrost model_mapping | Set `SMOKE_MODEL` to a mapped model (e.g., `cheap-chat`) |
| Dashboard tests: 403 Forbidden | `DASHBOARD_TOKEN` doesn't match the running dashboard | Update the secret to match `DASHBOARD_TOKEN` env in the dashboard container |
| New-API tests: all skipped | `ADMIN_USERNAME`/`ADMIN_PASSWORD` not set | Add the secrets |
| New-API tests: redirect to /login | Login failed — wrong credentials or 2FA enabled | Check credentials, disable 2FA on CI admin user |
| Cookie test timeout (20s) | No ClewdR instances reporting cookies | Verify ClewdR containers are healthy |
| Logs test timeout (15s) | No log entries after smoke inference | Verify smoke inference went through nginx (not direct to port 3000) |

### Artifacts

Every browser test run uploads:

- `browser-results.xml` — JUnit XML (parseable by GitHub, GitLab, Jenkins)
- `browser-results.json` — JSON detail with per-test timing

Release runs also upload:

- `artifacts/releases/<version>/<timestamp>/` — full RC validation evidence

### Nightly Failures

The nightly job runs the same browser tests plus CLI health/drift/cookie checks. If the nightly fails but CI passed earlier, it means something changed in the running environment (service down, cookie expiry, drift introduced).

Check the step that failed:
- **Health check** → a service is down
- **Drift check** → config diverged from desired state
- **Cookie status** → ClewdR cookie count dropped
- **Smoke tests** → inference path broken
- **Browser tests** → dashboard or New-API admin issue
