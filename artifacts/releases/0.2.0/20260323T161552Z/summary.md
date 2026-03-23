# RC Validation Evidence — StarAPIHub v0.2.0

| Field | Value |
|-------|-------|
| Version | 0.2.0 |
| Mode | appliance |
| Timestamp | 20260323T161552Z |
| Result | **PASS** |
| Pass | 20 / 20 |
| Fail | 0 |
| Skip | 0 |
| Patch count | 1 |

## Checklist Gates

| Section | Gate | Result | Evidence | Notes |
|---------|------|--------|----------|-------|
| Pre-release | VERSION file | **PASS** | version-file.txt |  |
| Pre-release | CLI build | **PASS** | build.log |  |
| Pre-release | go vet | **PASS** | lint.log |  |
| Pre-release | unit tests | **PASS** | unit-tests.log |  |
| Pre-release | integration tests | **PASS** | integration-tests.log |  |
| Image build | dashboard image build | **PASS** | build-dashboard.log |  |
| Image build | patched New-API image | **PASS** | build-patched-newapi.log |  |
| Validation | health check | **PASS** | health.json |  |
| Validation | registry validate | **PASS** | validate.log |  |
| Validation | bootstrap | **PASS** | bootstrap.log |  |
| Validation | sync | **PASS** | sync.log |  |
| Validation | diff | **PASS** | diff.log |  |
| Validation | upgrade-check | **PASS** | upgrade-check.log |  |
| Validation | smoke tests | **PASS** | smoke-tests.log |  |
| Auditability | trace | **PASS** | trace.log |  |
| Auditability | audit log exists | **PASS** | audit-log-tail.txt |  |
| Auditability | Patch 001 | **PASS** | patch001-headers.txt |  |
| — | Patch 001 backward compat | **PASS** | patch001-backward-compat-headers.txt |  |
| Dashboard | dashboard /api/version | **PASS** | dashboard-version.json |  |
| Dashboard | dashboard /api/health | **PASS** | dashboard-health.json |  |

## Image Provenance

```
# Image provenance — v0.2.0 validated at 20260323T161552Z
# Mode: appliance

## Running containers (from docker inspect)

- cp-new-api: image=starapihub/new-api:patched digest=sha256:75468e16023cb8778673ab5be3a12436e524a50b849667a2a76d273117b51956
- cp-bifrost: image=maximhq/bifrost:latest digest=sha256:0b67d35ef9c71d09c3ccc3eeb844f91e98b1cacd38b004e00c32921e26ad7350
- cp-dashboard: image=starapihub/dashboard:local digest=sha256:83c1255b3d9b03beb6b54d343f65ac542f9f0f5ff9b933d631e85080f6eb2729
- cp-clewdr-1: image=clewdr:local digest=sha256:12d48e7e17877d0be8b8c6eedcd6bf90f335141921c4f5893848db6ab8f18ea5
- cp-clewdr-2: image=clewdr:local digest=sha256:12d48e7e17877d0be8b8c6eedcd6bf90f335141921c4f5893848db6ab8f18ea5
- cp-clewdr-3: image=clewdr:local digest=sha256:12d48e7e17877d0be8b8c6eedcd6bf90f335141921c4f5893848db6ab8f18ea5

## Local image digests (from docker image inspect)

- calciumion/new-api:latest: calciumion/new-api@sha256:648622b40a8275ed45eb988b0df3f0a548241b4eea5e2367ad377cd892d31b4a
- starapihub/new-api:patched: starapihub/new-api@sha256:e92f5b33799852c6fce988857b71501682c109c83a768be49380e9b85dce4470
- maximhq/bifrost:latest: maximhq/bifrost@sha256:0b67d35ef9c71d09c3ccc3eeb844f91e98b1cacd38b004e00c32921e26ad7350
- clewdr:local: clewdr@sha256:12d48e7e17877d0be8b8c6eedcd6bf90f335141921c4f5893848db6ab8f18ea5
- starapihub/dashboard:local: starapihub/dashboard@sha256:cc8b10bc5cf0c75eb41f6001cdc7ec8dfe2b3ed6fbe6f7928daf9f19137c5b3f

## CLI binary

{"build_date":"2026-03-23T16:15:52Z","go_version":"go1.26.1","mode":"appliance","version":"0.2.0"}
```

## Build Metadata

```json
{"build_date":"2026-03-23T16:15:52Z","go_version":"go1.26.1","mode":"appliance","version":"0.2.0"}
```
