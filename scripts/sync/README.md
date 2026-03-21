# Sync Scripts

## Overview

The control plane defines routing policies, logical models, and provider pools in `policies/*.yaml` files. These definitions must be translated into configuration that each upstream system understands:

- **New-API** needs channels with model mappings (via admin API)
- **Bifrost** needs provider config and routing rules (via config.json file)
- **ClewdR** needs cookies and admin passwords (via its web UI -- not scriptable)

The sync strategy uses three tiers:

| Tier | Method | Scripts |
|------|--------|---------|
| Generated config | Script reads policies, writes config fragments | `generate-config.sh` |
| API sync | Script calls upstream admin APIs | `sync-newapi-channels.sh` |
| Manual steps | Operator performs via admin UI | Documented in `plan-sync.md` |

## Scripts

### generate-config.sh

Reads all `policies/*.yaml` files and produces:

- `generated/bifrost-config-fragment.json` -- PSEUDOCONFIG for Bifrost providers (fill in API keys manually)
- `generated/newapi-channel-guidance.txt` -- Human-readable channel definitions
- `generated/model-summary.txt` -- Table of all logical models with routing info

No running stack or secrets required. Safe to run at any time.

```bash
./generate-config.sh                          # output to control-plane/generated/
./generate-config.sh /tmp/my-output           # custom output directory
```

### sync-newapi-channels.sh

Creates Bifrost-facing channels in New-API via its admin API (`POST /api/channel/`). Requires a running New-API instance and an admin token.

```bash
# Dry run (preview without creating)
NEWAPI_URL=http://localhost:3000 ADMIN_TOKEN=<token> DRY_RUN=true ./sync-newapi-channels.sh

# Live sync
NEWAPI_URL=http://localhost:3000 ADMIN_TOKEN=<token> ./sync-newapi-channels.sh
```

**Required environment:**
- `NEWAPI_URL` -- New-API base URL
- `ADMIN_TOKEN` -- Admin bearer token (obtain from New-API admin UI -> personal settings)

**Optional:**
- `BIFROST_URL` -- Internal Bifrost URL used as the channel base_url (default: `http://bifrost:8080`)
- `DRY_RUN` -- Set to `true` to preview without creating (default: `false`)

## Manual Steps (Not Scriptable)

Some configuration cannot be automated without modifying upstream code:

| Task | System | Method |
|------|--------|--------|
| Set model pricing | New-API | Admin UI -> Settings -> Operation -> Model Pricing |
| Add ClewdR cookies | ClewdR | Each instance's web admin UI -> Claude tab |
| Set Bifrost API keys | Bifrost | Edit config.json or use Bifrost Web UI |
| Configure user group permissions | New-API | Admin UI -> Users/Groups |
| Set ClewdR admin password | ClewdR | `clewdr.toml` or environment variable |

See `plan-sync.md` for the complete step-by-step operator procedure.

## When to Run

- After initial deployment (run `generate-config.sh` then `sync-newapi-channels.sh`)
- After editing any file in `policies/`
- After adding or removing a provider
- After changing model mappings
- Before running smoke tests
