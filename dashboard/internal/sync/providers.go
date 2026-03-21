package sync

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/r3labs/diff/v3"
	"github.com/starapihub/dashboard/internal/registry"
	"github.com/starapihub/dashboard/internal/upstream"
)

// BifrostProviderClient is the interface the reconciler needs from the upstream Bifrost client.
type BifrostProviderClient interface {
	ListProvidersTyped() (map[string]upstream.BifrostProviderResponse, error)
	CreateProviderTyped(id string, provider json.RawMessage) error
	UpdateProviderTyped(id string, provider json.RawMessage) error
	DeleteProvider(id string) error
}

// ProviderReconciler reconciles Bifrost providers with create/update/delete semantics.
// Prune flag controls whether providers present in live but missing from desired are deleted.
type ProviderReconciler struct {
	client BifrostProviderClient
	prune  bool
}

// NewProviderReconciler creates a ProviderReconciler.
func NewProviderReconciler(client BifrostProviderClient, prune bool) *ProviderReconciler {
	return &ProviderReconciler{
		client: client,
		prune:  prune,
	}
}

// Name returns the resource type name.
func (r *ProviderReconciler) Name() string {
	return "provider"
}

// Plan compares desired providers against live providers and returns reconciliation actions.
// desired must be map[string]registry.BifrostProviderDesired (keyed by provider ID).
// live must be map[string]upstream.BifrostProviderResponse.
func (r *ProviderReconciler) Plan(desired, live any) ([]Action, error) {
	desiredMap, ok := desired.(map[string]registry.BifrostProviderDesired)
	if !ok {
		return nil, fmt.Errorf("ProviderReconciler.Plan: desired must be map[string]registry.BifrostProviderDesired, got %T", desired)
	}
	liveMap, ok := live.(map[string]upstream.BifrostProviderResponse)
	if !ok {
		return nil, fmt.Errorf("ProviderReconciler.Plan: live must be map[string]upstream.BifrostProviderResponse, got %T", live)
	}

	var actions []Action

	// For each desired provider
	for id, desiredProv := range desiredMap {
		liveProv, exists := liveMap[id]
		if !exists {
			actions = append(actions, Action{
				Type:         ActionCreate,
				ResourceType: "provider",
				ResourceID:   id,
				Desired:      desiredProv,
				Live:         nil,
			})
			continue
		}

		// Compare: normalize both sides for diff
		normDesired := normalizeDesiredForDiff(desiredProv)
		normLive := normalizeLiveForDiff(liveProv)

		changelog, err := diff.Diff(normLive, normDesired)
		if err != nil {
			return nil, fmt.Errorf("diff provider %s: %w", id, err)
		}
		if len(changelog) > 0 {
			diffStr := formatChangelog(changelog)
			actions = append(actions, Action{
				Type:         ActionUpdate,
				ResourceType: "provider",
				ResourceID:   id,
				Desired:      desiredProv,
				Live:         liveProv,
				Diff:         diffStr,
			})
		}
		// else: no changes, skip
	}

	// Prune: live providers not in desired
	if r.prune {
		for id, liveProv := range liveMap {
			if _, inDesired := desiredMap[id]; !inDesired {
				actions = append(actions, Action{
					Type:         ActionDelete,
					ResourceType: "provider",
					ResourceID:   id,
					Desired:      nil,
					Live:         liveProv,
				})
			}
		}
	}

	return actions, nil
}

// Apply executes a single provider reconciliation action.
func (r *ProviderReconciler) Apply(action Action) (*Result, error) {
	switch action.Type {
	case ActionCreate:
		return r.applyCreate(action)
	case ActionUpdate:
		return r.applyUpdate(action)
	case ActionDelete:
		return r.applyDelete(action)
	default:
		return &Result{Action: action, Status: StatusSkipped}, nil
	}
}

func (r *ProviderReconciler) applyCreate(action Action) (*Result, error) {
	desired, ok := action.Desired.(registry.BifrostProviderDesired)
	if !ok {
		return &Result{Action: action, Status: StatusFailed, Error: fmt.Errorf("desired is not BifrostProviderDesired")}, nil
	}

	payload, err := buildProviderPayload(desired)
	if err != nil {
		return &Result{Action: action, Status: StatusFailed, Error: err}, nil
	}

	if err := r.client.CreateProviderTyped(action.ResourceID, payload); err != nil {
		return &Result{Action: action, Status: StatusFailed, Error: err}, nil
	}
	return &Result{Action: action, Status: StatusOK}, nil
}

func (r *ProviderReconciler) applyUpdate(action Action) (*Result, error) {
	desired, ok := action.Desired.(registry.BifrostProviderDesired)
	if !ok {
		return &Result{Action: action, Status: StatusFailed, Error: fmt.Errorf("desired is not BifrostProviderDesired")}, nil
	}

	payload, err := buildProviderPayload(desired)
	if err != nil {
		return &Result{Action: action, Status: StatusFailed, Error: err}, nil
	}

	if err := r.client.UpdateProviderTyped(action.ResourceID, payload); err != nil {
		return &Result{Action: action, Status: StatusFailed, Error: err}, nil
	}
	return &Result{Action: action, Status: StatusOK}, nil
}

func (r *ProviderReconciler) applyDelete(action Action) (*Result, error) {
	if err := r.client.DeleteProvider(action.ResourceID); err != nil {
		return &Result{Action: action, Status: StatusFailed, Error: err}, nil
	}
	return &Result{Action: action, Status: StatusOK}, nil
}

// Verify reads back the provider after Apply and checks it matches expected state.
func (r *ProviderReconciler) Verify(action Action, result *Result) error {
	providers, err := r.client.ListProvidersTyped()
	if err != nil {
		result.Status = StatusUnverified
		result.DriftMsg = fmt.Sprintf("failed to read back providers: %v", err)
		return nil
	}
	result.ReadBack = providers

	switch action.Type {
	case ActionCreate, ActionUpdate:
		liveProv, exists := providers[action.ResourceID]
		if !exists {
			if action.Type == ActionCreate {
				result.Status = StatusUnverified
			} else {
				result.Status = StatusAppliedWithDrift
			}
			result.DriftMsg = fmt.Sprintf("provider %s not found in read-back", action.ResourceID)
			return nil
		}

		desired, ok := action.Desired.(registry.BifrostProviderDesired)
		if !ok {
			return nil
		}

		normDesired := normalizeDesiredForDiff(desired)
		normLive := normalizeLiveForDiff(liveProv)

		changelog, err := diff.Diff(normLive, normDesired)
		if err != nil {
			result.Status = StatusUnverified
			result.DriftMsg = fmt.Sprintf("diff error during verify: %v", err)
			return nil
		}
		if len(changelog) > 0 {
			result.Status = StatusAppliedWithDrift
			result.DriftMsg = formatChangelog(changelog)
		}

	case ActionDelete:
		if _, stillExists := providers[action.ResourceID]; stillExists {
			result.Status = StatusAppliedWithDrift
			result.DriftMsg = fmt.Sprintf("provider %s still present after delete", action.ResourceID)
		}
	}

	return nil
}

// --- Normalization and payload building ---

// providerNormalized is a comparable struct for diff purposes.
// Key values are stripped (secrets not in YAML), Description is stripped (control-plane metadata).
type providerNormalized struct {
	Keys                     []keyNormalized                        `json:"keys"`
	NetworkConfig            *upstream.BifrostNetworkConfigResponse `json:"network_config,omitempty"`
	ConcurrencyAndBufferSize *upstream.ConcurrencyBufferResponse    `json:"concurrency_and_buffer_size,omitempty"`
	CustomProviderConfig     *upstream.CustomProviderConfigResponse `json:"custom_provider_config,omitempty"`
}

type keyNormalized struct {
	ID      string   `json:"id"`
	Name    string   `json:"name"`
	Models  []string `json:"models"`
	Weight  float64  `json:"weight"`
	Enabled *bool    `json:"enabled,omitempty"`
}

// normalizeDesiredForDiff converts desired state to the normalized comparison form.
func normalizeDesiredForDiff(d registry.BifrostProviderDesired) providerNormalized {
	p := providerNormalized{}

	for _, k := range d.Keys {
		p.Keys = append(p.Keys, keyNormalized{
			ID:      k.ID,
			Name:    k.Name,
			Models:  k.Models,
			Weight:  k.Weight,
			Enabled: k.Enabled,
		})
	}

	if d.NetworkConfig != nil {
		p.NetworkConfig = &upstream.BifrostNetworkConfigResponse{
			BaseURL:                        d.NetworkConfig.BaseURL,
			ExtraHeaders:                   d.NetworkConfig.ExtraHeaders,
			DefaultRequestTimeoutInSeconds: d.NetworkConfig.DefaultRequestTimeoutInSeconds,
			MaxRetries:                     d.NetworkConfig.MaxRetries,
			RetryBackoffInitialMs:          d.NetworkConfig.RetryBackoffInitialMs,
			RetryBackoffMaxMs:              d.NetworkConfig.RetryBackoffMaxMs,
			StreamIdleTimeoutInSeconds:     d.NetworkConfig.StreamIdleTimeoutInSeconds,
		}
	}
	if d.ConcurrencyAndBufferSize != nil {
		p.ConcurrencyAndBufferSize = &upstream.ConcurrencyBufferResponse{
			Concurrency: d.ConcurrencyAndBufferSize.Concurrency,
			BufferSize:  d.ConcurrencyAndBufferSize.BufferSize,
		}
	}
	if d.CustomProviderConfig != nil {
		p.CustomProviderConfig = &upstream.CustomProviderConfigResponse{
			IsKeyLess:        d.CustomProviderConfig.IsKeyLess,
			BaseProviderType: d.CustomProviderConfig.BaseProviderType,
		}
	}

	return p
}

// normalizeLiveForDiff converts live state to the normalized comparison form.
// Strips key Value fields (secrets not in desired state YAML).
func normalizeLiveForDiff(l upstream.BifrostProviderResponse) providerNormalized {
	p := providerNormalized{}

	for _, k := range l.Keys {
		p.Keys = append(p.Keys, keyNormalized{
			ID:      k.ID,
			Name:    k.Name,
			Models:  k.Models,
			Weight:  k.Weight,
			Enabled: k.Enabled,
		})
	}

	p.NetworkConfig = l.NetworkConfig
	p.ConcurrencyAndBufferSize = l.ConcurrencyAndBufferSize
	p.CustomProviderConfig = l.CustomProviderConfig

	return p
}

// bedrockKeyConfigPayload is the API payload for Bedrock key config.
type bedrockKeyConfigPayload struct {
	AwsAccessKey string `json:"aws_access_key"`
	AwsSecretKey string `json:"aws_secret_key"`
	AwsRegion    string `json:"aws_region"`
}

// keyPayload is the API payload for a single provider key.
type keyPayload struct {
	ID               string                   `json:"id"`
	Name             string                   `json:"name"`
	Value            string                   `json:"value"`
	Models           []string                 `json:"models"`
	Weight           float64                  `json:"weight"`
	Enabled          *bool                    `json:"enabled,omitempty"`
	BedrockKeyConfig *bedrockKeyConfigPayload `json:"bedrock_key_config,omitempty"`
}

// provPayload is the API payload for a Bifrost provider.
type provPayload struct {
	Keys                     []keyPayload                       `json:"keys"`
	NetworkConfig            *registry.BifrostNetworkConfig     `json:"network_config,omitempty"`
	ConcurrencyAndBufferSize *registry.ConcurrencyAndBufferSize `json:"concurrency_and_buffer_size,omitempty"`
	CustomProviderConfig     *registry.CustomProviderConfig     `json:"custom_provider_config,omitempty"`
}

// buildProviderPayload creates the JSON payload for the Bifrost API.
// Resolves ValueEnv fields to actual key values and BedrockKeyConfig env vars.
// Omits Description (control-plane-only metadata).
func buildProviderPayload(desired registry.BifrostProviderDesired) (json.RawMessage, error) {

	payload := provPayload{
		NetworkConfig:            desired.NetworkConfig,
		ConcurrencyAndBufferSize: desired.ConcurrencyAndBufferSize,
		CustomProviderConfig:     desired.CustomProviderConfig,
	}

	for _, k := range desired.Keys {
		kp := keyPayload{
			ID:      k.ID,
			Name:    k.Name,
			Models:  k.Models,
			Weight:  k.Weight,
			Enabled: k.Enabled,
		}

		// Resolve key value from env var
		if k.ValueEnv != "" {
			val, err := ResolveEnvVar(k.ValueEnv)
			if err != nil {
				return nil, fmt.Errorf("key %s: %w", k.ID, err)
			}
			kp.Value = val
		}

		// Resolve Bedrock key config env vars
		if k.BedrockKeyConfig != nil {
			bkc := &bedrockKeyConfigPayload{
				AwsRegion: k.BedrockKeyConfig.AwsRegion,
			}
			if k.BedrockKeyConfig.AwsAccessKeyEnv != "" {
				val, err := ResolveEnvVar(k.BedrockKeyConfig.AwsAccessKeyEnv)
				if err != nil {
					return nil, fmt.Errorf("key %s bedrock aws_access_key: %w", k.ID, err)
				}
				bkc.AwsAccessKey = val
			}
			if k.BedrockKeyConfig.AwsSecretKeyEnv != "" {
				val, err := ResolveEnvVar(k.BedrockKeyConfig.AwsSecretKeyEnv)
				if err != nil {
					return nil, fmt.Errorf("key %s bedrock aws_secret_key: %w", k.ID, err)
				}
				bkc.AwsSecretKey = val
			}
			kp.BedrockKeyConfig = bkc
		}

		payload.Keys = append(payload.Keys, kp)
	}

	return json.Marshal(payload)
}

// formatChangelog converts r3labs/diff changelog to a human-readable string.
func formatChangelog(changelog diff.Changelog) string {
	var parts []string
	for _, change := range changelog {
		path := strings.Join(change.Path, ".")
		parts = append(parts, fmt.Sprintf("%s %s: %v -> %v", change.Type, path, change.From, change.To))
	}
	return strings.Join(parts, "; ")
}

// Compile-time interface check.
var _ Reconciler = (*ProviderReconciler)(nil)
