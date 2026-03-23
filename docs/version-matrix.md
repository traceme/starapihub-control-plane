# Version Matrix

## Purpose

This file records which upstream versions were validated with which appliance/control-plane release.

Use it to answer:

- which upstream versions are currently approved
- which combinations were tested together
- whether an upgrade changed the appliance patch set

## Rules

- Update this file for every appliance release candidate and production release.
- Pin exact versions or image digests where practical.
- If an upstream patch exists, note whether the patch changed during validation.

## Matrix

| Appliance Version | Mode | New-API | Bifrost | ClewdR | Patch Count | Validation Status | Notes |
|------------------|------|---------|---------|--------|-------------|-------------------|-------|
| `0.1.0` | appliance | `v0.11.7-4-gd7c5e84f` | `helm-chart-v2.0.14` | `v0.12.23` | `1` | upgrade-check passed | Patch 001 (X-Request-ID propagation) active |
| `0.2.0` | upstream | `calciumion/new-api@sha256:648622b40a8275ed45eb988b0df3f0a548241b4eea5e2367ad377cd892d31b4a` | `maximhq/bifrost@sha256:0b67d35ef9c71d09c3ccc3eeb844f91e98b1cacd38b004e00c32921e26ad7350` | `clewdr@sha256:12d48e7e17877d0be8b8c6eedcd6bf90f335141921c4f5893848db6ab8f18ea5` | `0` | **PASS (17/17 applicable)** | 2 skips N/A in upstream mode (patched New-API image, Patch 001). Evidence: `artifacts/releases/0.2.0/20260323T113245Z/` |
| `0.2.0` | appliance | `starapihub/new-api:patched@sha256:75468e16023cb8778673ab5be3a12436e524a50b849667a2a76d273117b51956` | `maximhq/bifrost@sha256:0b67d35ef9c71d09c3ccc3eeb844f91e98b1cacd38b004e00c32921e26ad7350` | `clewdr@sha256:12d48e7e17877d0be8b8c6eedcd6bf90f335141921c4f5893848db6ab8f18ea5` | `1` | **PASS (20/20)** | Patch 001 (X-Request-ID propagation) active. Evidence: `artifacts/releases/0.2.0/20260323T161552Z/` |

## Validation Checklist Per Row

Each matrix row should correspond to a concrete validation run covering:

- deploy/bootstrap
- config sync
- drift detection
- smoke tests
- auditability checks
- any patched behavior

## Suggested Notes Format

Use short notes like:

- `No upstream patches`
- `New-API request-id patch retained unchanged`
- `Bifrost upgraded cleanly, patch removed because upstream now supports metadata exposure`
