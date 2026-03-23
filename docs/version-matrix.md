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

| Appliance Version | Control-Plane State | New-API | Bifrost | ClewdR | Patch Count | Validation Status | Notes |
|------------------|---------------------|---------|---------|--------|-------------|-------------------|-------|
| `v0.1.0` | Phase 8 | `v0.11.7-4-gd7c5e84f` | `helm-chart-v2.0.14` | `v0.12.23` | `1` | upgrade-check passed | Patch 001 (X-Request-ID propagation) active |

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
