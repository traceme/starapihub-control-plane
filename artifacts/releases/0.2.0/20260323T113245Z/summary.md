# RC Validation Evidence — StarAPIHub v0.2.0

| Field | Value |
|-------|-------|
| Version | 0.2.0 |
| Mode | upstream |
| Timestamp | 20260323T113245Z |
| Result | **PARTIAL** |
| Pass | 17 / 19 |
| Fail | 0 |
| Skip | 2 |
| Patch count | 0 |

## Checklist Gates

| Section | Gate | Result | Evidence | Notes |
|---------|------|--------|----------|-------|
| Pre-release | VERSION file | **PASS** | version-file.txt |  |
| Pre-release | CLI build | **PASS** | build.log |  |
| Pre-release | go vet | **PASS** | lint.log |  |
| Pre-release | unit tests | **PASS** | unit-tests.log |  |
| Pre-release | integration tests | **PASS** | integration-tests.log |  |
| Image build | dashboard image build | **PASS** | build-dashboard.log |  |
| Image build | patched New-API image | **SKIP** | — | mode is upstream |
| Validation | health check | **PASS** | health.json |  |
| Validation | registry validate | **PASS** | validate.log |  |
| Validation | bootstrap | **PASS** | bootstrap.log |  |
| Validation | sync | **PASS** | sync.log |  |
| Validation | diff | **PASS** | diff.log |  |
| Validation | upgrade-check | **PASS** | upgrade-check.log |  |
| Validation | smoke tests | **PASS** | smoke-tests.log |  |
| Auditability | trace | **PASS** | trace.log |  |
| Auditability | audit log exists | **PASS** | audit-log-tail.txt |  |
| Auditability | Patch 001 | **SKIP** | — | mode is upstream (no patch to verify) |
| Dashboard | dashboard /api/version | **PASS** | dashboard-version.json |  |
| Dashboard | dashboard /api/health | **PASS** | dashboard-health.json |  |

## Image Provenance

```
# Image provenance — v0.2.0 validated at 20260323T113245Z
# Mode: upstream

## Running containers (from docker inspect)

- cp-new-api: image=calciumion/new-api:latest digest=sha256:648622b40a8275ed45eb988b0df3f0a548241b4eea5e2367ad377cd892d31b4a
- cp-bifrost: image=maximhq/bifrost:latest digest=sha256:0b67d35ef9c71d09c3ccc3eeb844f91e98b1cacd38b004e00c32921e26ad7350
- cp-dashboard: image=starapihub/dashboard:local digest=sha256:04abb001010d87915c08dd3c10b26f40945265a02ae2c88007721cb088b6f47c
- cp-clewdr-1: image=clewdr:local digest=sha256:12d48e7e17877d0be8b8c6eedcd6bf90f335141921c4f5893848db6ab8f18ea5
- cp-clewdr-2: image=clewdr:local digest=sha256:12d48e7e17877d0be8b8c6eedcd6bf90f335141921c4f5893848db6ab8f18ea5
- cp-clewdr-3: image=clewdr:local digest=sha256:12d48e7e17877d0be8b8c6eedcd6bf90f335141921c4f5893848db6ab8f18ea5

## Local image digests (from docker image inspect)

- calciumion/new-api:latest: calciumion/new-api@sha256:648622b40a8275ed45eb988b0df3f0a548241b4eea5e2367ad377cd892d31b4a
- maximhq/bifrost:latest: maximhq/bifrost@sha256:0b67d35ef9c71d09c3ccc3eeb844f91e98b1cacd38b004e00c32921e26ad7350
- clewdr:local: clewdr@sha256:12d48e7e17877d0be8b8c6eedcd6bf90f335141921c4f5893848db6ab8f18ea5
- starapihub/dashboard:local: starapihub/dashboard@sha256:cf142e8c8cef42888011f902f9b33e393a9386a32c2b48e32522bd2dfde28d33

## CLI binary

{"build_date":"2026-03-23T11:32:45Z","go_version":"go1.26.1","mode":"upstream","version":"0.2.0"}
```

## Build Metadata

```json
{"build_date":"2026-03-23T11:32:45Z","go_version":"go1.26.1","mode":"upstream","version":"0.2.0"}
```
