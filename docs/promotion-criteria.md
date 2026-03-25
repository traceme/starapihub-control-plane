# Promotion Criteria — StarAPIHub Releases

## Purpose

This document defines the minimum evidence set required to promote a release candidate to production. Promotion decisions are evidence-based — no gate may be satisfied by memory or verbal confirmation alone.

## Promotion Gates

A release candidate is **promotable** when ALL of the following are true:

| # | Gate | Evidence Source | How to Verify |
|---|------|----------------|---------------|
| 1 | Release workflow succeeded | GitHub Actions run (green) | `.github/workflows/release.yml` run page |
| 2 | `release-manifest.json` exists with pushed digests | Workflow artifact (90-day retention) | Download from release workflow run → `releases/<version>/release-manifest.json` |
| 3 | All validation gates passed | `release-manifest.json` → `validation` block | All four fields: `lint`, `unit_tests`, `rc_validate`, `browser_regression` = `"pass"` |
| 4 | Pushed image digests are recorded | `release-manifest.json` → `images` block | `dashboard.digest` and (appliance mode) `patched_newapi.digest` are real `sha256:` values |
| 5 | CLI binary sha256 recorded | `release-manifest.json` → `cli.sha256` | Non-empty hex string |
| 6 | Browser regression passed (9/9) | Workflow artifact: `browser-results.xml` | JUnit XML shows 9 passed, 0 failed |
| 7 | Nightly green streak (≥1 night) | GitHub Actions → `nightly.yml` history | At least one nightly run after the release workflow passed with no failures |
| 8 | Version matrix updated | `control-plane/docs/version-matrix.md` | Row exists for this version with validation status = PASS |
| 9 | Git tag exists | `git tag -l "v<version>"` | Tag points to the release commit |
| 10 | GitHub Release exists | `gh release view v<version>` | Release is published (not draft), CLI binary attached |

## Nightly Signal

Nightly runs (`nightly.yml`) are not a promotion gate per se — a release can be promoted after the first successful nightly. However:

- **≥3 consecutive green nightlies** = high confidence for production promotion
- **1 green nightly** = minimum for initial promotion
- **0 green nightlies since release** = do NOT promote; investigate first

Nightly failures after promotion do not automatically demote a release, but they should trigger investigation and may trigger rollback (see `rollback-runbook.md`).

## Promotion Decision Flow

```
Release workflow completes
  → Download release-manifest.json from workflow artifacts
  → Verify all validation gates = "pass"
  → Verify pushed digests are real sha256 values
  → Wait for ≥1 green nightly run
  → Update version-matrix.md with validation evidence
  → Update release-status.md with promoted version
  → PROMOTED
```

## What Is NOT a Promotion Gate

- Manual testing beyond what CI covers (CI is the truth source)
- Approval from a specific person (evidence replaces authority)
- Dashboard visual inspection (browser regression covers this)
- Load testing (out of scope for v1.5)

## Mode-Specific Notes

### Upstream Mode

- Only the dashboard image is promoted
- `patched_newapi.digest` in the manifest will read `"n/a (upstream mode)"`
- Operators pull upstream New-API images independently

### Appliance Mode

- Both dashboard and patched New-API images are promoted
- Both digests must be real `sha256:` values in the manifest
- The patched New-API image includes Patch 001 (X-Request-ID propagation)

## Evidence Locations

| Evidence | Type | Location | Retention |
|----------|------|----------|-----------|
| `release-manifest.json` | Workflow artifact | Release workflow run → Artifacts | 90 days |
| CLI binary | GitHub Release asset | `gh release download v<version>` | Permanent |
| Browser results | Workflow artifact | Release workflow run → Artifacts | 90 days |
| Pushed images | Container registry | `docker pull <registry>/<namespace>/<image>:<tag>` | Registry policy |
| Git tag | Git | `git tag -l "v<version>"` | Permanent |
| GitHub Release | GitHub | `gh release view v<version>` | Permanent |
| Nightly logs | Workflow artifact | Nightly workflow run → Artifacts | 14 days |

**Important:** `release-manifest.json` is a workflow artifact, NOT a GitHub Release asset. This is intentional — the manifest is build evidence for operators reviewing the pipeline, not a downloadable operator tool. The CLI binary is the only release asset attached to the GitHub Release.
