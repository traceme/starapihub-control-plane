# Release Notes — StarAPIHub v0.2.0-appliance

**Released:** 2026-03-23
**Mode:** appliance (Patch 001 active)
**Validation:** 20/20 gates PASS, 0 failures, 0 skips

## What This Release Is

StarAPIHub v0.2.0-appliance is the first validated **appliance-mode** release. It proves the full control-plane toolchain works end-to-end with Patch 001 (X-Request-ID propagation) baked into a custom New-API image — all 20 validation gates pass with zero skips.

## Validated Topology

```
Client -> New-API (patched) -> Bifrost -> ClewdR (x3) / official providers
                ^
        StarAPIHub control plane (off hot path)
```

The patched New-API image includes Patch 001 (X-Request-ID propagation middleware). All other images are unmodified vendor pulls:

| Component | Image | Digest |
|-----------|-------|--------|
| New-API (patched) | `starapihub/new-api:patched` | `sha256:75468e16023cb8778673ab5be3a12436e524a50b849667a2a76d273117b51956` |
| Bifrost | `maximhq/bifrost:latest` | `sha256:0b67d35ef9c71d09c3ccc3eeb844f91e98b1cacd38b004e00c32921e26ad7350` |
| ClewdR (x3) | `clewdr:local` | `sha256:12d48e7e17877d0be8b8c6eedcd6bf90f335141921c4f5893848db6ab8f18ea5` |
| Dashboard | `starapihub/dashboard:local` | `sha256:83c1255b3d9b03beb6b54d343f65ac542f9f0f5ff9b933d631e85080f6eb2729` |

**CLI binary:** Go 1.26.1, built 2026-03-23T16:15:52Z

## Patch 001: X-Request-ID Propagation

The sole upstream patch modifies `new-api/middleware/request-id.go` to:
- **Preserve** caller-supplied `X-Request-ID` headers through to `X-Oneapi-Request-Id` response header
- **Auto-generate** a request ID when none is supplied (backward compatible)

This enables single-ID end-to-end request correlation across the full Client -> New-API -> Bifrost -> ClewdR chain.

## Operator Capabilities

Same 8 CLI subcommands as v0.2.0 upstream (validate, sync, diff, bootstrap, health, trace, cookie-status, upgrade-check).

## Validation Evidence

20 gates across 5 sections. Full evidence bundle at `artifacts/releases/0.2.0/20260323T161552Z/`.

| Section | Gates | Result |
|---------|-------|--------|
| Pre-release (VERSION, build, vet, unit tests, integration tests) | 5/5 | PASS |
| Image build (dashboard image, patched New-API image) | 2/2 | PASS |
| Live validation (health, validate, bootstrap, sync, diff, upgrade-check, smoke) | 7/7 | PASS |
| Auditability (trace, audit log, Patch 001 + backward compat) | 4/4 | PASS |
| Dashboard runtime (/api/version, /api/health) | 2/2 | PASS |

## Changes From v0.2.0 (upstream)

- Mode changed from upstream to appliance
- Patch count increased from 0 to 1 (Patch 001 active)
- New-API image switched from `calciumion/new-api:latest` to `starapihub/new-api:patched`
- 2 previously-skipped gates now PASS: patched New-API image build, Patch 001 verification
- Patch 001 backward compatibility verified as a separate gate
- Validation result: 20/20 PASS (vs 17/17 applicable in upstream mode)

---
*Evidence: `artifacts/releases/0.2.0/20260323T161552Z/`*
*Version matrix: `docs/version-matrix.md`*
