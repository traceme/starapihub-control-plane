# Integration Tests

End-to-end tests that build the `starapihub` CLI binary and run it against a live Docker Compose stack (New-API + Bifrost + Postgres + Redis).

## Prerequisites

- Docker and Docker Compose
- Go 1.22+
- The test compose stack running (see below)

## Running

### Start the test stack

```bash
cd tests/integration/compose
docker compose -f docker-compose.test.yml up -d
```

Wait for services to become healthy:

```bash
docker compose -f docker-compose.test.yml ps
```

### Run tests

From the `control-plane/` directory:

```bash
make test-integration
```

Or directly:

```bash
cd tests/integration
INTEGRATION=1 go test -v -timeout 300s ./...
```

The `INTEGRATION=1` environment variable gates the tests. Without it, `go test` skips all integration tests so that `make test-unit` remains fast and dependency-free.

### Tear down

```bash
make clean
```

Or:

```bash
cd tests/integration/compose
docker compose -f docker-compose.test.yml down -v --remove-orphans
```

## What the tests cover

| Test | What it verifies |
|------|-----------------|
| `TestHealth_ReportsServiceStatus` | `starapihub health` reaches live services |
| `TestHealth_JSONOutput` | `--output json` produces parseable JSON |
| `TestBootstrap_SeedsAdminAndSyncs` | `starapihub bootstrap` runs, audit log records `bootstrap_steps` |
| `TestSync_DryRunShowsActions` | `starapihub sync --dry-run` reports planned actions |
| `TestSync_DryRunJSON` | `--dry-run --output json` produces structured output |
| `TestSync_TargetNormalization` | Plural targets (`channels`) normalize to canonical names |
| `TestSync_UnknownTargetErrors` | Unknown targets produce a clear error |
| `TestSync_ApplyWithAuditLog` | `starapihub sync` writes to the JSONL audit log |
| `TestDiff_ProducesDriftReport` | `starapihub diff` reports drift |
| `TestDiff_JSONOutput` | `--output json` drift report is parseable |
| `TestDiff_TargetFilterWorks` | `--target channel` filters to one reconciler |
| `TestValidate_ValidFixtures` | `starapihub validate` accepts valid YAML registries |
| `TestPatch001_XRequestIDPropagation` | Incoming `X-Request-ID` is preserved by New-API (Patch 001) |

## Test fixtures

`fixtures/` contains minimal YAML registries (channels, providers, pricing) used by sync/diff tests. These are not production configs — they exist only to exercise the reconcilers against live services.

## Test architecture

Tests are in a separate Go module (`tests/integration/go.mod`) because they live outside the `dashboard/` module. Each test:

1. Builds the CLI binary from `../../dashboard/cmd/starapihub/`
2. Runs it as a subprocess against the live Docker Compose services
3. Asserts on exit code, stdout, and (for audit tests) written JSONL entries

This means the tests exercise the real CLI binary, not test doubles.
