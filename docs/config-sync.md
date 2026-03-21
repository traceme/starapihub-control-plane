# Config Generation and Sync Strategy

## Overview

The control plane generates configuration that must be loaded into upstream systems. Since we cannot modify upstream source code, syncing happens through four mechanisms:

1. **File mounting** — generate config files, mount them into containers at deploy time
2. **Admin API** — call upstream admin endpoints to create/update resources
3. **Admin UI** — manual steps in upstream web dashboards (when no API exists)
4. **Environment variables** — set at deploy time via compose env files

The control plane is never on the request hot path. Config sync is an operator-driven activity that happens at deploy time, after policy changes, or during maintenance windows.

_For authoritative API shapes and struct definitions, see `docs/capability-audit.md`._

## Sync Matrix

| Config Target | Sync Method | Automation Level | Script |
|--------------|-------------|-----------------|--------|
| Bifrost providers (official) | Mount `config.json` or Web UI | Semi-automated | `scripts/sync/generate-config.sh` |
| Bifrost routing rules | Mount `config.json` or Web UI | Semi-automated | `scripts/sync/generate-config.sh` |
| Bifrost providers (ClewdR) | Bifrost Web UI or config.json | Semi-automated | `scripts/sync/generate-config.sh` |
| New-API channels | Admin API `POST /api/channel/` | Fully automatable | `scripts/sync/sync-newapi-channels.sh` |
| New-API model mapping | Admin API (part of channel config) | Fully automatable | `scripts/sync/sync-newapi-channels.sh` |
| New-API model pricing | Admin API `PUT /api/option/` with key=ModelRatio | Fully automatable | Phase 3 sync engine (see capability-audit.md -- System Options) |
| New-API model visibility | Admin API (channel group assignment) | Partially automatable | `scripts/sync/sync-newapi-channels.sh` |
| ClewdR cookies | ClewdR Admin API `POST /api/cookie` per instance | Fully automatable | Phase 3 sync engine (see capability-audit.md -- Cookie Management) |
| ClewdR config options | ClewdR Admin API or env vars | Semi-automated | Environment files |
| Nginx config | File mount | Fully automated | Template in `config/nginx/` |
| TLS certificates | File mount or cert-manager | Automated | External tooling |

## Operator Workflows

### Workflow 1: Initial Deployment

This is the full bootstrap sequence for a fresh stack.

```
Step  Action                          Method              Reference
----  ------                          ------              ---------
1     Copy and fill env templates     Manual edit         deploy/env/*.env.example
2     Generate Bifrost config.json    Script              scripts/sync/generate-config.sh
3     Review nginx config             Manual review       config/nginx/nginx.conf
4     Start the stack                 docker-compose up   deploy/docker-compose.yml
5     Wait for health (30-60s)        docker-compose ps   —
6     Create New-API admin account    API POST /api/setup  See capability-audit.md -- setup.go PostSetup
7     Sync New-API channels           Script or manual    scripts/sync/sync-newapi-channels.sh
8     Set model pricing               API PUT /api/option/ key=ModelRatio (see capability-audit.md)
9     Configure ClewdR cookies        API POST /api/cookie per instance (see capability-audit.md)
10    Run smoke tests                 Script              scripts/smoke/run-all.sh
```

Steps 1-5 are deploy-time. Steps 6-9 are post-deploy configuration. Step 10 is verification.

### Workflow 2: Add a New Logical Model

When the operator wants to expose a new model to clients.

```
Step  Action                                  Method
----  ------                                  ------
1     Add entry to policies/logical-models    Manual edit of YAML
2     Verify upstream_model in provider pool  Review policies/provider-pools
3     If Bifrost config needs updating:
      a. Re-run generate-config.sh    Script
      b. Remount or reload Bifrost config     Container restart or Web UI
4     Sync New-API channels                   Script: sync-newapi-channels.sh
5     Run smoke tests 3, 4, or 5              Script: run-all.sh (or targeted)
```

### Workflow 3: Rotate ClewdR Cookies

When a ClewdR instance's cookies expire (typical frequency: weekly).

```
Step  Action                                  Method
----  ------                                  ------
1     Extract fresh cookies from Claude.ai    Browser DevTools or cookie extension
2     Port-forward to the target instance     ssh -L 18484:clewdr-N:8484 server
3     Open ClewdR admin UI                    http://localhost:18484
4     Log in with admin password              From container logs or env
5     Replace old cookies with new ones       Claude tab in admin UI
6     Verify health in ClewdR dashboard       Admin UI health indicators
7     Run smoke test 5 (risky route)          Script or manual curl
```

Rotate one instance at a time. Keep at least one healthy while rotating others.

### Workflow 4: Rotate Official Provider API Keys

When provider keys need rotation (security policy, compromise, etc.).

```
Step  Action                                  Method
----  ------                                  ------
1     Generate new key in provider dashboard  Provider's website
2     Update key in Bifrost                   Web UI or update config.json
3     If using config.json: restart Bifrost   docker-compose restart bifrost
4     Verify with smoke test 4 (premium)      Script or manual curl
5     Revoke the old key in provider dash     Provider's website
```

### Workflow 5: Change Route Policy for a Model

When the operator wants to move a model between tiers (e.g., premium to standard).

```
Step  Action                                  Method
----  ------                                  ------
1     Update policies/logical-models           Change bifrost_route_policy + risk_level
2     Update policies/route-policies if needed Change pool chain
3     Update New-API channel assignment        Move model between channel model lists
4     Re-sync to New-API                       Script: sync-newapi-channels.sh
5     If Bifrost routing rules changed:
      a. Re-run generate-config.sh     Script
      b. Reload Bifrost config                 Container restart or Web UI
6     Run relevant smoke tests                 Tests 4 and/or 5
```

### Workflow 6: Upstream Version Upgrade

See `docs/upgrade-strategy.md` for the full procedure. Summary:

```
Step  Action                                  Method
----  ------                                  ------
1     Read upstream release notes              Manual review
2     Update version in deploy/env/common.env  Manual edit
3     Pull new images                          docker-compose pull
4     Check for config schema changes          Compare with config templates
5     If schema changed: update templates      Manual edit of config/ files
6     Recreate containers                      docker-compose up -d
7     Run full smoke test suite                scripts/smoke/run-all.sh
```

## Sync Scripts

### scripts/sync/sync-newapi-channels.sh

Creates or updates New-API channels based on the logical model registry and route policy registry. The script:

1. Reads `policies/logical-models.example.yaml` (or production equivalent)
2. Groups models by `newapi_channel`
3. For each channel, calls `POST /api/channel/` (create) or `PUT /api/channel/` (update)
4. Sets model lists, model mapping, and channel configuration

Supports `--dry-run` to preview changes without applying them.

**Requirements**: `curl`, `jq`, admin token set in `NEWAPI_ADMIN_TOKEN` env var.

### scripts/sync/generate-config.sh

Generates a Bifrost `config.json` from the provider pool and route policy registries. The script:

1. Reads `policies/provider-pools.example.yaml` and `policies/route-policies.example.yaml`
2. Generates the `providers` section with keys, models, and network config
3. Generates the `governance.routing_rules` section with CEL expressions matching model patterns
4. Outputs to `config/bifrost/config.json`

The generated file can be mounted into the Bifrost container at `/app/data/config.json`.

**Requirements**: `yq` (YAML processor), `jq`.

## Distinguishing Config Types

| Label | Meaning | Lifecycle | Example |
|-------|---------|-----------|---------|
| **Generated** | Output of a script, can be re-generated any time | Rebuild from registries | `config/bifrost/config.json` |
| **Template** | Starting point, requires manual customization | Copy and edit once | `deploy/env/*.env.example` |
| **Manual** | Must be configured through a UI by hand | Operator action each time | ClewdR cookies, New-API model pricing |
| **Verified** | Source-code-verified config with struct definitions | Confirmed in Phase 1 audit | All Bifrost provider, governance, and ClewdR entries |

_All PSEUDOCONFIG labels in this repository were resolved in the Phase 1 capability audit. See capability-audit.md for full API documentation with verified struct definitions and JSON field names._

## Config Drift Detection

Over time, manual changes in upstream UIs can cause drift between the registries and the running systems. To detect drift:

1. **New-API channels**: Run `sync-newapi-channels.sh --dry-run` and check if it proposes changes. If it does, either the registry or the running config has drifted.
2. **Bifrost config**: Run `generate-config.sh` and diff the output against the mounted `config.json` or export from Bifrost Web UI.
3. **ClewdR**: Use `GET /api/cookies` per instance to check cookie health programmatically (see capability-audit.md -- Cookie Management).

Schedule a monthly drift check as part of the operational routine (see `docs/runbook.md`).

## Network Context for Sync Operations

Sync scripts run from the operator's machine or a CI/CD runner on the `ops` network zone. They need access to:

| Target | Endpoint | Network Zone | Auth |
|--------|----------|-------------|------|
| New-API admin API | `https://api.example.com/api/channel/` | `public` (through Nginx) | Admin token |
| Bifrost Web UI | `http://bifrost:8080` | `core` (requires port forward or VPN) | Admin credentials (if auth enabled) |
| ClewdR admin UI | `http://clewdr-N:8484` | `provider` (requires port forward) | Admin password |

For security, prefer SSH tunneling or a bastion host rather than exposing internal services to the operator's network.
