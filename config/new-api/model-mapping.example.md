# New-API Model Mapping Reference

## Mapping Logical Models to Upstream Models

This table shows the complete model name flow through the system. Logical models
are stable client-facing names. Upstream model IDs are version-specific identifiers
that change when providers release new versions.

| Logical Model | Upstream Model ID | Channel | Route Policy | Provider Path |
|--------------|-------------------|---------|-------------|---------------|
| `claude-sonnet` | `claude-sonnet-4-20250514` | bifrost-premium | premium | Bifrost -> Anthropic API |
| `claude-opus` | `claude-opus-4-20250514` | bifrost-premium | premium | Bifrost -> Anthropic API |
| `claude-haiku` | `claude-haiku-4-5-20251001` | bifrost-premium | premium | Bifrost -> Anthropic API |
| `gpt-4o` | `gpt-4o` | bifrost-premium | premium | Bifrost -> OpenAI API |
| `gpt-4o-mini` | `gpt-4o-mini` | bifrost-premium | premium | Bifrost -> OpenAI API |
| `cheap-chat` | `claude-sonnet-4-20250514` | bifrost-standard | standard | Bifrost -> Anthropic API (ClewdR fallback) |
| `fast-chat` | `claude-haiku-4-5-20251001` | bifrost-standard | standard | Bifrost -> Anthropic API (ClewdR fallback) |
| `lab-claude` | `claude-sonnet-4-20250514` | bifrost-risky | risky | Bifrost -> ClewdR (Anthropic fallback) |
| `lab-claude-opus` | `claude-opus-4-20250514` | bifrost-risky | risky | Bifrost -> ClewdR (Anthropic fallback) |

## How Model Names Flow Through the System

```
Client request:
  model = "claude-sonnet"

New-API receives request:
  1. Looks up "claude-sonnet" across enabled channels
  2. Finds it in "bifrost-premium" (priority 0)
  3. Applies model_mapping: "claude-sonnet" -> "claude-sonnet-4-20250514"
  4. Forwards to http://bifrost:8080/v1/chat/completions
     with model = "claude-sonnet-4-20250514"

Bifrost receives request:
  1. Receives model = "claude-sonnet-4-20250514"
  2. Checks routing rules (if any match)
  3. Finds the model in the Anthropic provider's key config
  4. Routes to Anthropic API

Anthropic receives request:
  1. model = "claude-sonnet-4-20250514"
  2. Returns response

Response flows back:
  Anthropic -> Bifrost -> New-API -> Client
```

## Model Versioning Strategy

Logical model names are intentionally version-free (e.g., `claude-sonnet` not
`claude-sonnet-4`). This allows transparent upgrades:

### When Anthropic releases a new model version

Example: `claude-sonnet-4-20250514` is replaced by `claude-sonnet-4-20251001`

1. **Update the logical model registry**: Edit `policies/logical-models.example.yaml`
   and change `upstream_model` to the new version
2. **Update New-API channel mappings**: Change the model_mapping JSON in the
   relevant channel(s) to point to the new model ID
3. **Update Bifrost provider keys**: Add the new model ID to the key's `models`
   array in config.json (remove the old one if desired)
4. **No client changes**: Clients still request `claude-sonnet` and get the latest

### When adding a completely new model

Example: Adding `gpt-5` to the platform

1. Add the model entry to `policies/logical-models.example.yaml`
2. Add it to the appropriate New-API channel's model list and mapping
3. Ensure Bifrost has a provider key that covers the new model
4. Announce the new logical model name to clients

## Billing Implications

New-API tracks usage per logical model name (`claude-sonnet`, `cheap-chat`, etc.).
The billing name in `policies/logical-models.example.yaml` should match what
New-API uses for its billing records. Different logical models pointing to the
same upstream model can have different billing rates based on the channel tier.

For example:
- `claude-sonnet` (premium channel) bills at full rate
- `cheap-chat` (standard channel, same upstream model) could bill at a discount
- `lab-claude` (risky channel, uses ClewdR) could bill at zero or minimal rate

Billing rate configuration happens in New-API's admin UI under model pricing settings.
