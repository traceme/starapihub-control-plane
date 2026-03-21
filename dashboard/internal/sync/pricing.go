package sync

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/starapihub/dashboard/internal/registry"
	"github.com/starapihub/dashboard/internal/upstream"
)

// ratioKeys are the New-API option keys for model pricing.
var ratioKeys = []string{"ModelRatio", "ModelPrice", "CompletionRatio", "CacheRatio"}

// NewAPIPricingClient is the interface the pricing reconciler needs.
type NewAPIPricingClient interface {
	GetOptionsTyped(adminToken string) ([]upstream.OptionEntry, error)
	PutOption(adminToken string, key string, value string) error
}

// PricingReconciler reconciles model pricing into New-API option JSON strings.
// It merges per-model ratios into existing option values, preserving entries
// for models it does not manage.
type PricingReconciler struct {
	client     NewAPIPricingClient
	adminToken string
}

// NewPricingReconciler creates a PricingReconciler.
func NewPricingReconciler(client NewAPIPricingClient, adminToken string) *PricingReconciler {
	return &PricingReconciler{
		client:     client,
		adminToken: adminToken,
	}
}

// Name returns the resource type name.
func (r *PricingReconciler) Name() string {
	return "pricing"
}

// Plan compares desired pricing against live option entries and returns update actions.
// desired must be map[string]registry.ModelPricing (keyed by model name).
// live must be []upstream.OptionEntry.
func (r *PricingReconciler) Plan(desired, live any) ([]Action, error) {
	desiredMap, ok := desired.(map[string]registry.ModelPricing)
	if !ok {
		return nil, fmt.Errorf("PricingReconciler.Plan: desired must be map[string]registry.ModelPricing, got %T", desired)
	}
	liveEntries, ok := live.([]upstream.OptionEntry)
	if !ok {
		return nil, fmt.Errorf("PricingReconciler.Plan: live must be []upstream.OptionEntry, got %T", live)
	}

	// Parse live options into maps
	liveMaps := make(map[string]map[string]float64, len(ratioKeys))
	for _, entry := range liveEntries {
		for _, key := range ratioKeys {
			if entry.Key == key {
				var m map[string]float64
				if err := json.Unmarshal([]byte(entry.Value), &m); err != nil {
					m = make(map[string]float64)
				}
				liveMaps[key] = m
				break
			}
		}
	}

	var actions []Action

	for _, ratioKey := range ratioKeys {
		// Build desired values for this ratio key
		desiredValues := make(map[string]float64)
		for model, pricing := range desiredMap {
			val := getRatioValue(pricing, ratioKey)
			if val != nil {
				desiredValues[model] = *val
			}
		}

		if len(desiredValues) == 0 {
			continue
		}

		// Compare against live
		liveMap := liveMaps[ratioKey]
		if liveMap == nil {
			liveMap = make(map[string]float64)
		}

		var diffs []string
		needsUpdate := false
		for model, desiredVal := range desiredValues {
			liveVal, exists := liveMap[model]
			if !exists {
				diffs = append(diffs, fmt.Sprintf("%s: added=%.4g", model, desiredVal))
				needsUpdate = true
			} else if liveVal != desiredVal {
				diffs = append(diffs, fmt.Sprintf("%s: %.4g->%.4g", model, liveVal, desiredVal))
				needsUpdate = true
			}
		}

		if needsUpdate {
			actions = append(actions, Action{
				Type:         ActionUpdate,
				ResourceType: "pricing",
				ResourceID:   ratioKey,
				Desired:      desiredMap,
				Diff:         strings.Join(diffs, ", "),
			})
		}
	}

	return actions, nil
}

// getRatioValue extracts the appropriate ratio value from a ModelPricing based on the ratio key.
func getRatioValue(p registry.ModelPricing, ratioKey string) *float64 {
	switch ratioKey {
	case "ModelRatio":
		return p.ModelRatio
	case "ModelPrice":
		return p.ModelPrice
	case "CompletionRatio":
		return p.CompletionRatio
	case "CacheRatio":
		return p.CacheRatio
	}
	return nil
}

// Apply executes a single pricing update action by merging values into the existing option.
func (r *PricingReconciler) Apply(action Action) (*Result, error) {
	desiredMap, ok := action.Desired.(map[string]registry.ModelPricing)
	if !ok {
		return &Result{Action: action, Status: StatusFailed, Error: fmt.Errorf("desired is not map[string]registry.ModelPricing")}, nil
	}

	ratioKey := action.ResourceID

	// Read current live value for this key
	currentMap := make(map[string]float64)
	options, err := r.client.GetOptionsTyped(r.adminToken)
	if err == nil {
		for _, entry := range options {
			if entry.Key == ratioKey {
				json.Unmarshal([]byte(entry.Value), &currentMap)
				break
			}
		}
	}

	// Merge desired values into current map (preserves unmanaged models)
	for model, pricing := range desiredMap {
		val := getRatioValue(pricing, ratioKey)
		if val != nil {
			currentMap[model] = *val
		}
	}

	// Marshal and put
	merged, err := json.Marshal(currentMap)
	if err != nil {
		return &Result{Action: action, Status: StatusFailed, Error: err}, nil
	}

	err = r.client.PutOption(r.adminToken, ratioKey, string(merged))
	if err != nil {
		return &Result{Action: action, Status: StatusFailed, Error: err}, nil
	}

	return &Result{Action: action, Status: StatusOK}, nil
}

// Verify reads back the option and confirms desired model entries are present and correct.
func (r *PricingReconciler) Verify(action Action, result *Result) error {
	desiredMap, ok := action.Desired.(map[string]registry.ModelPricing)
	if !ok {
		return fmt.Errorf("desired is not map[string]registry.ModelPricing")
	}

	ratioKey := action.ResourceID

	options, err := r.client.GetOptionsTyped(r.adminToken)
	if err != nil {
		result.Status = StatusUnverified
		result.DriftMsg = fmt.Sprintf("read-back failed: %v", err)
		return nil
	}
	result.ReadBack = options

	// Find the option by key
	var liveMap map[string]float64
	for _, entry := range options {
		if entry.Key == ratioKey {
			if err := json.Unmarshal([]byte(entry.Value), &liveMap); err != nil {
				result.Status = StatusAppliedWithDrift
				result.DriftMsg = fmt.Sprintf("failed to parse %s value: %v", ratioKey, err)
				return nil
			}
			break
		}
	}

	if liveMap == nil {
		result.Status = StatusAppliedWithDrift
		result.DriftMsg = fmt.Sprintf("option %s not found in read-back", ratioKey)
		return nil
	}

	// Check each desired model's ratio
	var drifts []string
	for model, pricing := range desiredMap {
		val := getRatioValue(pricing, ratioKey)
		if val == nil {
			continue
		}
		liveVal, exists := liveMap[model]
		if !exists {
			drifts = append(drifts, fmt.Sprintf("%s: missing", model))
		} else if liveVal != *val {
			drifts = append(drifts, fmt.Sprintf("%s: expected=%.4g got=%.4g", model, *val, liveVal))
		}
	}

	if len(drifts) > 0 {
		result.Status = StatusAppliedWithDrift
		result.DriftMsg = strings.Join(drifts, "; ")
	}

	return nil
}

// Compile-time interface check.
var _ Reconciler = (*PricingReconciler)(nil)
