# Sync Plan: Step-by-Step Operator Guide

This document describes the full procedure for synchronizing control plane policies into the three upstream systems. Follow these steps in order after any policy change.

## Prerequisites

- Docker Compose stack is running (`cd deploy && docker-compose up -d`)
- You have a New-API admin token (obtain from Admin UI -> personal settings)
- You have Bifrost config access (mounted volume or Web UI)
- You have ClewdR admin UI access for each instance

## Step 1: Generate Config Fragments

Run the config generator to produce upstream-facing artifacts:

```bash
cd control-plane/scripts/sync
./generate-config.sh
```

This creates three files in `control-plane/generated/`:

| File | Purpose |
|------|---------|
| `bifrost-config-fragment.json` | Provider entries for Bifrost config.json |
| `newapi-channel-guidance.txt` | Channel definitions for New-API |
| `model-summary.txt` | Human-readable model routing table |

Review all three files before proceeding.

## Step 2: Sync Bifrost Provider Config

Bifrost reads its configuration from `config.json` (mounted into the container).

1. Open `generated/bifrost-config-fragment.json`
2. Copy the provider entries into your working Bifrost `config.json`
3. **Fill in API keys** -- the generated file uses placeholder values
4. If Bifrost is running, restart it to pick up the new config:
   ```bash
   docker-compose restart bifrost
   ```

**Note:** The generated Bifrost JSON uses verified field names from the Phase 1 capability audit. See `docs/capability-audit.md` for authoritative struct definitions. The script currently outputs providers as an array; Bifrost expects an object keyed by provider name -- the Phase 3 Go replacement will produce the correct format.

## Step 3: Sync New-API Channels

Option A -- Automated (recommended):

```bash
NEWAPI_URL=http://localhost:3000 ADMIN_TOKEN=<your-token> ./sync-newapi-channels.sh
```

Option B -- Manual via Admin UI:

1. Go to New-API Admin UI -> Channels
2. Create three channels matching `generated/newapi-channel-guidance.txt`:
   - **bifrost-premium**: priority 0, models = claude-sonnet, claude-opus, etc.
   - **bifrost-standard**: priority 5, models = cheap-chat, fast-chat
   - **bifrost-risky**: priority 10, models = lab-claude, lab-claude-opus
3. For each channel:
   - Type: OpenAI compatible (type 1)
   - Base URL: `http://bifrost:8080` (internal Docker network)
   - Key: leave empty (Bifrost handles provider auth)
   - Model mapping: copy from the generated JSON

## Step 4: Set Model Pricing in New-API

Model pricing can be set via the admin API: `PUT /api/option/` with `key=ModelRatio` (RootAuth required). The value is a JSON-encoded string mapping model names to ratios. See `docs/capability-audit.md` -- System Options section for all pricing keys (ModelRatio, ModelPrice, CompletionRatio, CacheRatio, etc.).

Manual alternative: New-API Admin UI -> Settings -> Operation -> Model Pricing.

Pricing guidelines:
1. Premium models should reflect actual provider costs
2. Standard models can be priced lower (ClewdR fallback reduces cost)
3. Risky/lab models can be priced at zero or minimal

_Corrected 2026-03-21: PUT /api/option/ verified in controller/option.go:105._

## Step 5: Configure ClewdR Instances

For each ClewdR instance (clewdr-1, clewdr-2, clewdr-3):

1. Access its admin UI at `http://<instance-ip>:8484`
2. Go to the Claude tab
3. Add/update the Claude.ai session cookie
4. Verify the cookie is valid by testing a request

ClewdR cookies expire periodically. Set a reminder to check them regularly.

## Step 6: Configure User Groups (Optional)

If using group-based model access:

1. Go to New-API Admin UI -> Users/Groups
2. Create groups matching the `allowed_groups` in `policies/logical-models.example.yaml`:
   - `all` -- default group for all users
   - `premium` -- users who can access claude-opus
   - `lab` -- users who can access lab-claude models
   - `admin` -- operators with full access
3. Assign users to appropriate groups

## Step 7: Verify with Smoke Tests

```bash
cd control-plane/scripts/smoke
NEWAPI_URL=http://localhost:3000 API_KEY=<test-user-key> ./run-all.sh
```

All tests should pass. If any fail, check the specific test output for remediation guidance.

## Ongoing Maintenance

| Trigger | Action |
|---------|--------|
| Policy file edited | Re-run Steps 1-3, then Step 7 |
| New provider added | Edit `policies/provider-pools.example.yaml`, re-run Steps 1-2 |
| New model added | Edit `policies/logical-models.example.yaml`, re-run Steps 1-3 |
| ClewdR cookie expired | Step 5 only |
| Upstream system upgraded | Re-run Step 7 to verify nothing broke |
| Provider key rotated | Step 2 (update Bifrost config.json) |
