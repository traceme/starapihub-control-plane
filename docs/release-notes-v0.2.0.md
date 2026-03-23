# Release Notes — StarAPIHub v0.2.0

**Released:** 2026-03-23
**Mode:** upstream (unpatched vendor images)
**Validation:** 17/17 applicable gates PASS, 0 failures, 2 skips (N/A in upstream mode)

## What This Release Is

StarAPIHub v0.2.0 is the first validated **upstream-mode** release. It proves the full control-plane toolchain works end-to-end against stock vendor container images — no source patches, no custom builds.

## Validated Topology

```
Client → New-API → Bifrost → ClewdR (×3) / official providers
                ↑
        StarAPIHub control plane (off hot path)
```

All upstream images are unmodified vendor pulls:

| Component | Image | Digest |
|-----------|-------|--------|
| New-API | `calciumion/new-api:latest` | `sha256:648622b40a8275ed45eb988b0df3f0a548241b4eea5e2367ad377cd892d31b4a` |
| Bifrost | `maximhq/bifrost:latest` | `sha256:0b67d35ef9c71d09c3ccc3eeb844f91e98b1cacd38b004e00c32921e26ad7350` |
| ClewdR (×3) | `clewdr:local` | `sha256:12d48e7e17877d0be8b8c6eedcd6bf90f335141921c4f5893848db6ab8f18ea5` |
| Dashboard | `starapihub/dashboard:local` | `sha256:04abb001010d87915c08dd3c10b26f40945265a02ae2c88007721cb088b6f47c` |

**CLI binary:** Go 1.26.1, built 2026-03-23T11:32:45Z

## Operator Capabilities

The `starapihub` CLI provides 8 subcommands validated in this release:

| Command | Capability |
|---------|-----------|
| `validate` | Check YAML registries against JSON Schema (models, channels, providers, routing, pricing) |
| `sync` | Idempotent reconciliation to all 3 upstreams (dry-run, fail-fast, target filtering) |
| `diff` | Drift detection with severity classification (text/JSON output, CI-friendly exit codes) |
| `bootstrap` | Fresh environment from zero to healthy in one command |
| `health` | Concurrent service health checks across the stack |
| `trace` | Cross-layer request tracing via docker logs with correlation IDs |
| `cookie-status` | ClewdR cookie inventory with threshold alerting |
| `upgrade-check` | 5-gate verification (deployment, sync, request-path, auditability, patch-intent) |

## Validation Evidence

19 gates across 5 sections. Full evidence bundle at `artifacts/releases/0.2.0/20260323T113245Z/`.

| Section | Gates | Result |
|---------|-------|--------|
| Pre-release (VERSION, build, vet, unit tests, integration tests) | 5/5 | PASS |
| Image build (dashboard image) | 1/1 | PASS |
| Live validation (health, validate, bootstrap, sync, diff, upgrade-check, smoke) | 7/7 | PASS |
| Auditability (trace, audit log) | 2/2 | PASS |
| Dashboard runtime (/api/version, /api/health) | 2/2 | PASS |

### Skipped Gates (N/A in upstream mode)

| Gate | Reason |
|------|--------|
| Patched New-API image build | No patches in upstream mode — stock vendor image used |
| Patch 001 (X-Request-ID propagation) | Patch not applied — upstream New-API does not include the request-ID middleware modification |

These are **not open gaps**. They are structurally inapplicable to upstream mode. Patch 001 and the patched New-API image are scoped to appliance-mode releases only.

## Known Boundary

**Patch 001** (X-Request-ID propagation in `new-api/middleware/request-id.go`) enables full single-ID end-to-end request correlation. This patch:

- Is **not part of this release** — v0.2.0 ships with unpatched upstream images
- Will be validated separately in an **appliance-mode release** with its own version-matrix row and evidence set
- Does not affect any functionality validated in this release; upstream-mode tracing uses the 3-tier correlation strategy (nginx-injected headers)

## Changes Since v0.1.0

- Upstream images updated to current vendor latest
- Full RC validation pipeline with 19-gate evidence bundle
- Integration tests added to pre-release gates
- Dashboard image build and runtime verification gates added
- Patch count reduced from 1 to 0 (upstream mode)
- Evidence artifacts structured as reproducible bundle

---
*Evidence: `artifacts/releases/0.2.0/20260323T113245Z/`*
*Version matrix: `docs/version-matrix.md`*
