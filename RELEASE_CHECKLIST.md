# Release Checklist — StarAPIHub Commercial Appliance

Use this checklist for every release candidate and production release.
Run `make rc-validate` to execute all gates automatically with evidence capture.

## Pre-release

- [ ] VERSION file updated
- [ ] CLI build succeeds: `make build`
- [ ] `go vet` clean: `make lint`
- [ ] Unit tests pass: `make test-unit`
- [ ] Integration tests pass: `make test-integration`

## Image build

- [ ] Dashboard image built: `make build-dashboard`
- [ ] Patched New-API image built (appliance mode only): `make build-patched-newapi`

## Validation (against real stack)

- [ ] Health checks pass: `starapihub health`
- [ ] Validate passes: `starapihub validate`
- [ ] Bootstrap succeeds: `starapihub bootstrap`
- [ ] Config sync works: `starapihub sync --dry-run`
- [ ] Drift detection works: `starapihub diff`
- [ ] Upgrade check passes: `starapihub upgrade-check`
- [ ] Smoke tests pass: `scripts/smoke/run-all.sh`

## Auditability

- [ ] Trace command works: `starapihub trace <request-id>`
- [ ] Audit log records operations (check `~/.starapihub/audit.log`)
- [ ] If appliance mode: verify X-Request-ID propagation (Patch 001)

## Documentation

- [ ] `docs/version-matrix.md` updated from evidence (not manually)
- [ ] `RELEASE_CHECKLIST.md` reviewed (this file)
- [ ] Release notes written

## Ship

- [ ] Git tag created: `git tag v$(cat VERSION)`
- [ ] Images pushed to registry (if applicable)
- [ ] Operator notified of new version

## Post-release

- [ ] Verify production deployment matches validated versions
- [ ] Run `starapihub health` against production
- [ ] Run `starapihub diff` to confirm zero drift

## Automation

`make rc-validate` runs all Pre-release, Image build, Validation, and Auditability
gates and writes evidence to `artifacts/releases/<version>/<timestamp>/summary.md`.

Required environment:
- `NEWAPI_ADMIN_TOKEN` — for sync/diff/bootstrap gates
- `STARAPIHUB_MODE` — `upstream` or `appliance`

Optional:
- `SKIP_IMAGES=1` — skip Docker image builds
- `SKIP_INTEGRATION=1` — skip integration tests
