# Config Sync

## Overview

The `starapihub` CLI is the primary tool for syncing desired state to upstream systems. It replaces the legacy shell scripts (`generate-config.sh`, `sync-newapi-channels.sh`) with typed Go reconciliation that supports idempotent CRUD, write-read-verify, dependency ordering, and dry-run.

The control plane is never on the request hot path. Config sync is an operator-driven activity that happens at deploy time, after policy changes, or during maintenance windows.

_For authoritative API shapes and struct definitions, see `docs/capability-audit.md`._

## CLI Commands

### `starapihub sync`

Reconciles desired-state YAML registries against live upstream systems.

```bash
# Sync everything (all 6 resource types in dependency order)
starapihub sync

# Sync only specific targets (plurals accepted)
starapihub sync --target channels,providers
starapihub sync --target routing-rule,pricing

# Preview changes without applying
starapihub sync --dry-run

# Delete resources not in desired state
starapihub sync --prune

# Stop on first error
starapihub sync --fail-fast

# JSON output
starapihub sync --output json
```

**Valid targets:** `channel`, `provider`, `config`, `routing-rule`, `pricing`, `cookie` (plurals like `channels`, `providers` are accepted and normalized).

**Dependency order:** cookie -> provider -> config -> routing-rule -> channel -> pricing

### `starapihub diff`

Detects drift between desired state and live systems. Classifies severity (informational, warning, blocking) for CI/cron use.

```bash
# Show all drift
starapihub diff

# Show only specific targets
starapihub diff --target channels,config

# Show only blocking drift
starapihub diff --severity blocking

# CI-friendly: treat warnings as exit 0
starapihub diff --exit-warn

# Write full JSON report to file
starapihub diff --report-file drift-report.json
```

**Exit codes:** 0=clean, 1=warning drift, 2=blocking drift.

### `starapihub bootstrap`

One command to go from zero to fully configured.

```bash
# Full bootstrap
starapihub bootstrap

# Skip admin seeding (already done)
starapihub bootstrap --skip-seed

# Skip sync (validate + health only)
starapihub bootstrap --skip-sync

# Preview mode
starapihub bootstrap --dry-run
```

See `docs/rollout-plan.md` for phased deployment guidance.

### `starapihub validate`

Validates YAML registries against JSON Schema before any sync operation.

```bash
starapihub validate
starapihub validate --config-dir /path/to/policies
```

### `starapihub health`

Checks health of all upstream services.

```bash
starapihub health
starapihub health --output json
```

## Sync Matrix

| Config Target | Reconciler | CLI Target | Sync Method |
|--------------|------------|------------|-------------|
| Bifrost providers | ProviderReconciler | `provider` | Live API: POST/PUT /api/providers |
| Bifrost config | ConfigReconciler | `config` | Live API: PUT /api/config (field-level merge) |
| Bifrost routing rules | RoutingRuleReconciler | `routing-rule` | Live API: CRUD /api/governance/routing-rules |
| New-API channels | ChannelReconciler | `channel` | Admin API: CRUD /api/channel/ (name-based matching) |
| New-API model pricing | PricingReconciler | `pricing` | Admin API: PUT /api/option/ (merge strategy) |
| ClewdR cookies | CookieReconciler | `cookie` | Admin API: POST /api/cookie (push-only) |
| Nginx config | File mount | N/A | Template in `config/nginx/` |
| TLS certificates | File mount | N/A | External tooling |

## Operator Workflows

### Workflow 1: Initial Deployment

```bash
# 1. Copy and fill env templates
cd control-plane/deploy/env
cp common.env.example common.env && $EDITOR common.env

# 2. Start the stack
cd control-plane/deploy && docker-compose up -d

# 3. Bootstrap (validates prereqs, waits for services, seeds admin, syncs config, verifies health)
starapihub bootstrap

# 4. Verify
starapihub health
starapihub diff
```

### Workflow 2: Add a New Logical Model

```bash
# 1. Edit the YAML registry
$EDITOR policies/logical-models.yaml

# 2. Validate
starapihub validate

# 3. Preview changes
starapihub sync --dry-run

# 4. Apply
starapihub sync

# 5. Verify
starapihub diff
```

### Workflow 3: Rotate ClewdR Cookies

```bash
# 1. Extract fresh cookies from Claude.ai (browser DevTools)
# 2. Push via CLI
starapihub sync --target cookie

# Or via ClewdR admin UI if preferred
```

### Workflow 4: Rotate Official Provider API Keys

```bash
# 1. Generate new key in provider dashboard
# 2. Update key in YAML registry
$EDITOR policies/provider-pools.yaml
# 3. Sync providers
starapihub sync --target provider
# 4. Verify
starapihub diff --target provider
```

### Workflow 5: Change Route Policy

```bash
# 1. Update policies
$EDITOR policies/logical-models.yaml
$EDITOR policies/route-policies.yaml
# 2. Validate and preview
starapihub validate && starapihub sync --dry-run
# 3. Apply
starapihub sync
# 4. Verify
starapihub diff
```

### Workflow 6: Upstream Version Upgrade

See `docs/upgrade-strategy-commercial-appliance.md`. Summary:

```bash
starapihub upgrade-check
# Follow upgrade workflow: bump versions, reapply patches, rebuild, sync, verify
```

## Drift Detection

Schedule regular drift checks:

```bash
# Cron: detect drift daily at 06:00
0 6 * * * starapihub diff --exit-warn --report-file /var/log/starapihub/drift-$(date +\%F).json
```

Exit codes enable CI gating: `--exit-warn` treats warnings as exit 0 (lenient mode).

## Audit Trail

Every `sync` and `bootstrap` operation is logged to `~/.starapihub/audit.log` (JSONL format). Override with `--audit-log <path>` or disable with `--no-audit`.

## Legacy Shell Scripts

The following scripts in `scripts/sync/` predate the Go CLI and are **deprecated**:

| Script | Replacement |
|--------|-------------|
| `generate-config.sh` | `starapihub sync --target provider,config,routing-rule` |
| `sync-newapi-channels.sh` | `starapihub sync --target channel` |

The shell scripts remain in the repo for reference but should not be used for production operations. The Go CLI provides:
- Write-read-verify (catches silent config rejection)
- Typed diffing (not text-based)
- Dependency ordering (the scripts don't enforce order)
- Audit logging
- Dry-run with accurate previews

## Network Context

Sync commands run from the operator's machine or CI runner and need access to:

| Target | Endpoint | Auth |
|--------|----------|------|
| New-API | `NEWAPI_URL` env var | `NEWAPI_ADMIN_TOKEN` env var |
| Bifrost | `BIFROST_URL` env var | None (internal network) |
| ClewdR | `CLEWDR_URLS` env var (comma-separated) | `CLEWDR_ADMIN_TOKEN` env var |
