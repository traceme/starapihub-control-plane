# Release Checklist — StarAPIHub Commercial Appliance

Use this checklist for every release candidate and production release.

## Pre-release

- [ ] VERSION file updated
- [ ] Unit tests pass: `make test-unit`
- [ ] Integration tests pass: `make test-integration`
- [ ] `go vet` clean: `make lint`
- [ ] Build succeeds: `make build`

## Image build

- [ ] Dashboard image built: `make build-dashboard`
- [ ] Patched New-API image built (if using appliance mode): `make build-patched-newapi`
- [ ] Images tagged with version: `docker tag starapihub/dashboard:local starapihub/dashboard:$(cat VERSION)`

## Validation (against real stack)

- [ ] Fresh stack bootstraps cleanly: `starapihub bootstrap`
- [ ] Config sync works: `starapihub sync`
- [ ] Drift detection works: `starapihub diff`
- [ ] Health checks pass: `starapihub health`
- [ ] Validate passes: `starapihub validate`
- [ ] Upgrade check passes: `starapihub upgrade-check`
- [ ] Smoke tests pass: `scripts/smoke/run-all.sh`

## Auditability

- [ ] Trace command works: `starapihub trace <request-id>`
- [ ] Audit log records sync operations (check `~/.starapihub/audit.log`)
- [ ] If using patched image: verify X-Request-ID propagation

## Documentation

- [ ] `docs/version-matrix.md` updated with validated versions and image digests
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
