# StarAPIHub v0.2.0 — Upstream Mode Release

**Date:** 2026-03-23
**Mode:** upstream (no upstream source patches)
**Patch count:** 0

## What was validated

17 of 19 release checklist gates passed. The 2 skipped gates are not applicable
in upstream mode (patched New-API image build, Patch 001 X-Request-ID
propagation). Zero failures.

### Validated capabilities

- **CLI tooling**: `starapihub version`, `health`, `validate`, `bootstrap`,
  `sync`, `diff`, `upgrade-check`, `trace` — all operational
- **Dashboard**: built from source, serves `/api/version` (public metadata)
  and `/api/health` (authenticated, checks all upstream services including
  ClewdR via internal Docker network)
- **Config sync**: channel and pricing resources synced to New-API via admin
  API with zero post-sync drift
- **Smoke tests**: New-API reachable, Bifrost reachable, ClewdR isolation
  confirmed (ports not exposed to host), logical model resolution available
- **Auditability**: X-Request-ID trace correlation, audit log persistence
- **Image provenance**: all container digests captured from running stack

### Validated topology

```
Host (CLI + rc-validate)
  -> New-API     (127.0.0.1:3000)   calciumion/new-api:latest
  -> Bifrost     (127.0.0.1:8080)   maximhq/bifrost:latest
  -> Dashboard   (127.0.0.1:8090)   starapihub/dashboard:local
  -> ClewdR x3   (internal only)    clewdr:local
```

ClewdR instances run on the internal provider-net Docker network. Health is
validated through the dashboard's `/api/health` endpoint, which reaches ClewdR
directly. ClewdR ports are not exposed to the host — the smoke isolation test
confirms this.

### Validated image digests

| Service | Image | Digest |
|---------|-------|--------|
| New-API | calciumion/new-api:latest | `sha256:648622b40a8275ed45eb988b0df3f0a548241b4eea5e2367ad377cd892d31b4a` |
| Bifrost | maximhq/bifrost:latest | `sha256:0b67d35ef9c71d09c3ccc3eeb844f91e98b1cacd38b004e00c32921e26ad7350` |
| Dashboard | starapihub/dashboard:local | `sha256:04abb001010d87915c08dd3c10b26f40945265a02ae2c88007721cb088b6f47c` |
| ClewdR | clewdr:local | `sha256:12d48e7e17877d0be8b8c6eedcd6bf90f335141921c4f5893848db6ab8f18ea5` |

## Known boundaries

- **Patch 001 (X-Request-ID propagation)** is appliance-mode only. It is not
  included in this release and was correctly skipped during validation.
- **Patched New-API image** is appliance-mode only. The upstream
  `calciumion/new-api:latest` image is used unmodified.
- **Bifrost admin API**: Bifrost does not expose a REST API for provider or
  routing-rule CRUD. Config sync targets New-API resources (channels, pricing)
  only. Bifrost provider configuration is managed via its config file.
- **ClewdR cookie admin**: requires direct network access to ClewdR instances.
  In the rc-validate topology, this is only available from the dashboard
  container (via provider-net), not from the host.

## Evidence

Full evidence bundle: `artifacts/releases/0.2.0/20260323T113245Z/`

Reproduce with:
```bash
cd deploy && docker compose -f docker-compose.yml -f docker-compose.rc-validate.yml \
  --env-file env/common.env --profile clewdr up -d --build
export STARAPIHUB_MODE=upstream
export NEWAPI_ADMIN_TOKEN=<admin-access-token>
export DASHBOARD_TOKEN=<dashboard-token>
cd .. && make rc-validate
```

## Next milestone

Appliance-mode validation (separate release):
- Patched New-API image build
- Patch 001 end-to-end proof (X-Request-ID propagation)
- Separate version-matrix row and evidence set
