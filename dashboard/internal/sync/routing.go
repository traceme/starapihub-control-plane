package sync

import (
	"encoding/json"
	"fmt"
	"reflect"

	"github.com/starapihub/dashboard/internal/registry"
	"github.com/starapihub/dashboard/internal/upstream"
)

// BifrostRoutingClient is the interface the routing reconciler needs from the upstream Bifrost client.
type BifrostRoutingClient interface {
	ListRoutingRulesTyped() ([]upstream.BifrostRoutingRuleResponse, error)
	CreateRoutingRuleTyped(rule json.RawMessage) (*upstream.BifrostRoutingRuleResponse, error)
	UpdateRoutingRuleTyped(id string, rule json.RawMessage) error
	DeleteRoutingRuleTyped(id string) error
}

// RoutingRuleReconciler reconciles Bifrost routing rules using ID-based matching.
// Supports full CRUD: create missing rules, update changed rules, delete extra rules (if prune=true).
type RoutingRuleReconciler struct {
	client BifrostRoutingClient
	prune  bool
}

// NewRoutingRuleReconciler creates a RoutingRuleReconciler.
func NewRoutingRuleReconciler(client BifrostRoutingClient, prune bool) *RoutingRuleReconciler {
	return &RoutingRuleReconciler{client: client, prune: prune}
}

// Name returns the resource type name.
func (r *RoutingRuleReconciler) Name() string {
	return "routing-rule"
}

// Plan compares desired vs live routing rules and returns actions.
// desired must be map[string]registry.RoutingRuleDesired (keyed by rule ID).
// live must be []upstream.BifrostRoutingRuleResponse.
func (r *RoutingRuleReconciler) Plan(desired, live any) ([]Action, error) {
	desiredRules, ok := desired.(map[string]registry.RoutingRuleDesired)
	if !ok {
		return nil, fmt.Errorf("RoutingRuleReconciler.Plan: desired must be map[string]registry.RoutingRuleDesired, got %T", desired)
	}
	liveRules, ok := live.([]upstream.BifrostRoutingRuleResponse)
	if !ok {
		return nil, fmt.Errorf("RoutingRuleReconciler.Plan: live must be []upstream.BifrostRoutingRuleResponse, got %T", live)
	}

	// Build live map keyed by ID
	liveMap := make(map[string]upstream.BifrostRoutingRuleResponse, len(liveRules))
	for _, lr := range liveRules {
		liveMap[lr.ID] = lr
	}

	var actions []Action

	// For each desired rule: create or update
	for id, d := range desiredRules {
		liveRule, exists := liveMap[id]
		if !exists {
			actions = append(actions, Action{
				Type:         ActionCreate,
				ResourceType: "routing-rule",
				ResourceID:   id,
				Desired:      d,
				Live:         nil,
				Diff:         fmt.Sprintf("create routing rule %s (%s)", id, d.Name),
			})
			continue
		}

		// Compare: normalize live to desired-comparable form
		normalized := normalizeRuleForComparison(liveRule)
		if rulesEqual(d, normalized) {
			// no change
			continue
		}

		diff := buildRuleDiff(id, d, normalized)
		actions = append(actions, Action{
			Type:         ActionUpdate,
			ResourceType: "routing-rule",
			ResourceID:   id,
			Desired:      d,
			Live:         liveRule,
			Diff:         diff,
		})
	}

	// If prune: delete extra live rules not in desired
	if r.prune {
		for id, lr := range liveMap {
			if _, inDesired := desiredRules[id]; !inDesired {
				actions = append(actions, Action{
					Type:         ActionDelete,
					ResourceType: "routing-rule",
					ResourceID:   id,
					Desired:      nil,
					Live:         lr,
					Diff:         fmt.Sprintf("delete routing rule %s (%s)", id, lr.Name),
				})
			}
		}
	}

	return actions, nil
}

// Apply executes a single routing rule action (create/update/delete).
func (r *RoutingRuleReconciler) Apply(action Action) (*Result, error) {
	switch action.Type {
	case ActionCreate:
		return r.applyCreate(action)
	case ActionUpdate:
		return r.applyUpdate(action)
	case ActionDelete:
		return r.applyDelete(action)
	default:
		return nil, fmt.Errorf("RoutingRuleReconciler.Apply: unsupported action type %s", action.Type)
	}
}

func (r *RoutingRuleReconciler) applyCreate(action Action) (*Result, error) {
	desired, ok := action.Desired.(registry.RoutingRuleDesired)
	if !ok {
		return nil, fmt.Errorf("RoutingRuleReconciler.Apply create: desired must be registry.RoutingRuleDesired")
	}

	payload := ruleToPayload(action.ResourceID, desired)
	data, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("marshal routing rule: %w", err)
	}

	_, err = r.client.CreateRoutingRuleTyped(data)
	if err != nil {
		return nil, err
	}

	return &Result{Action: action, Status: StatusOK}, nil
}

func (r *RoutingRuleReconciler) applyUpdate(action Action) (*Result, error) {
	desired, ok := action.Desired.(registry.RoutingRuleDesired)
	if !ok {
		return nil, fmt.Errorf("RoutingRuleReconciler.Apply update: desired must be registry.RoutingRuleDesired")
	}

	payload := ruleToPayload(action.ResourceID, desired)
	data, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("marshal routing rule: %w", err)
	}

	if err := r.client.UpdateRoutingRuleTyped(action.ResourceID, data); err != nil {
		return nil, err
	}

	return &Result{Action: action, Status: StatusOK}, nil
}

func (r *RoutingRuleReconciler) applyDelete(action Action) (*Result, error) {
	if err := r.client.DeleteRoutingRuleTyped(action.ResourceID); err != nil {
		return nil, err
	}
	return &Result{Action: action, Status: StatusOK}, nil
}

// Verify reads back routing rules and confirms the action took effect.
func (r *RoutingRuleReconciler) Verify(action Action, result *Result) error {
	rules, err := r.client.ListRoutingRulesTyped()
	if err != nil {
		result.Status = StatusUnverified
		result.DriftMsg = fmt.Sprintf("failed to read back routing rules: %v", err)
		return nil
	}

	// Build map for lookup
	ruleMap := make(map[string]upstream.BifrostRoutingRuleResponse, len(rules))
	for _, rule := range rules {
		ruleMap[rule.ID] = rule
	}

	switch action.Type {
	case ActionCreate, ActionUpdate:
		liveRule, exists := ruleMap[action.ResourceID]
		if !exists {
			result.Status = StatusAppliedWithDrift
			result.DriftMsg = fmt.Sprintf("rule %s not found after %s", action.ResourceID, action.Type)
			return nil
		}
		result.ReadBack = liveRule

		desired, ok := action.Desired.(registry.RoutingRuleDesired)
		if !ok {
			return fmt.Errorf("RoutingRuleReconciler.Verify: desired must be registry.RoutingRuleDesired")
		}

		normalized := normalizeRuleForComparison(liveRule)
		if !rulesEqual(desired, normalized) {
			result.Status = StatusAppliedWithDrift
			result.DriftMsg = buildRuleDiff(action.ResourceID, desired, normalized)
		}

	case ActionDelete:
		if _, exists := ruleMap[action.ResourceID]; exists {
			result.Status = StatusAppliedWithDrift
			result.DriftMsg = fmt.Sprintf("rule %s still exists after delete", action.ResourceID)
		}
	}

	return nil
}

// --- Helpers ---

// rulePayload is the JSON payload for create/update, including the ID field.
type rulePayload struct {
	ID            string                         `json:"id"`
	Name          string                         `json:"name"`
	Description   string                         `json:"description,omitempty"`
	Enabled       bool                           `json:"enabled"`
	CelExpression string                         `json:"cel_expression"`
	Targets       []registry.RoutingTargetDesired `json:"targets,omitempty"`
	Fallbacks     []string                       `json:"fallbacks,omitempty"`
	Query         map[string]any                 `json:"query,omitempty"`
	Scope         string                         `json:"scope"`
	ScopeID       *string                        `json:"scope_id,omitempty"`
	Priority      int                            `json:"priority"`
}

func ruleToPayload(id string, d registry.RoutingRuleDesired) rulePayload {
	return rulePayload{
		ID:            id,
		Name:          d.Name,
		Description:   d.Description,
		Enabled:       d.Enabled,
		CelExpression: d.CelExpression,
		Targets:       d.Targets,
		Fallbacks:     d.Fallbacks,
		Query:         d.Query,
		Scope:         d.Scope,
		ScopeID:       d.ScopeID,
		Priority:      d.Priority,
	}
}

// normalizeRuleForComparison converts a live rule response to the comparable desired form.
// Strips server-managed fields (ID is the map key, not in desired struct).
func normalizeRuleForComparison(live upstream.BifrostRoutingRuleResponse) registry.RoutingRuleDesired {
	var targets []registry.RoutingTargetDesired
	for _, t := range live.Targets {
		targets = append(targets, registry.RoutingTargetDesired{
			Provider: t.Provider,
			Model:    t.Model,
			KeyID:    t.KeyID,
			Weight:   t.Weight,
		})
	}

	return registry.RoutingRuleDesired{
		Name:          live.Name,
		Description:   live.Description,
		Enabled:       live.Enabled,
		CelExpression: live.CelExpression,
		Targets:       targets,
		Fallbacks:     live.Fallbacks,
		Query:         live.Query,
		Scope:         live.Scope,
		ScopeID:       live.ScopeID,
		Priority:      live.Priority,
	}
}

// rulesEqual compares two RoutingRuleDesired structs for equality.
func rulesEqual(a, b registry.RoutingRuleDesired) bool {
	// Normalize nil slices/maps to empty for comparison
	aTargets := a.Targets
	bTargets := b.Targets
	if aTargets == nil {
		aTargets = []registry.RoutingTargetDesired{}
	}
	if bTargets == nil {
		bTargets = []registry.RoutingTargetDesired{}
	}

	aFallbacks := a.Fallbacks
	bFallbacks := b.Fallbacks
	if aFallbacks == nil {
		aFallbacks = []string{}
	}
	if bFallbacks == nil {
		bFallbacks = []string{}
	}

	aQuery := a.Query
	bQuery := b.Query
	if aQuery == nil {
		aQuery = map[string]any{}
	}
	if bQuery == nil {
		bQuery = map[string]any{}
	}

	if a.Name != b.Name || a.Description != b.Description || a.Enabled != b.Enabled ||
		a.CelExpression != b.CelExpression || a.Scope != b.Scope || a.Priority != b.Priority {
		return false
	}

	if !reflect.DeepEqual(a.ScopeID, b.ScopeID) {
		return false
	}

	if !reflect.DeepEqual(aTargets, bTargets) {
		return false
	}

	if !reflect.DeepEqual(aFallbacks, bFallbacks) {
		return false
	}

	if !reflect.DeepEqual(aQuery, bQuery) {
		return false
	}

	return true
}

// buildRuleDiff generates a human-readable diff between desired and normalized live.
func buildRuleDiff(id string, desired, live registry.RoutingRuleDesired) string {
	var diffs []string

	if desired.Name != live.Name {
		diffs = append(diffs, fmt.Sprintf("name: %q -> %q", live.Name, desired.Name))
	}
	if desired.Description != live.Description {
		diffs = append(diffs, fmt.Sprintf("description: %q -> %q", live.Description, desired.Description))
	}
	if desired.Enabled != live.Enabled {
		diffs = append(diffs, fmt.Sprintf("enabled: %v -> %v", live.Enabled, desired.Enabled))
	}
	if desired.CelExpression != live.CelExpression {
		diffs = append(diffs, fmt.Sprintf("cel_expression: %q -> %q", live.CelExpression, desired.CelExpression))
	}
	if desired.Scope != live.Scope {
		diffs = append(diffs, fmt.Sprintf("scope: %q -> %q", live.Scope, desired.Scope))
	}
	if desired.Priority != live.Priority {
		diffs = append(diffs, fmt.Sprintf("priority: %d -> %d", live.Priority, desired.Priority))
	}
	if !reflect.DeepEqual(desired.ScopeID, live.ScopeID) {
		diffs = append(diffs, fmt.Sprintf("scope_id: %v -> %v", fmtStrPtr(live.ScopeID), fmtStrPtr(desired.ScopeID)))
	}
	if !reflect.DeepEqual(desired.Targets, live.Targets) {
		diffs = append(diffs, "targets: changed")
	}
	if !reflect.DeepEqual(desired.Fallbacks, live.Fallbacks) {
		diffs = append(diffs, "fallbacks: changed")
	}
	if !reflect.DeepEqual(desired.Query, live.Query) {
		diffs = append(diffs, "query: changed")
	}

	result := fmt.Sprintf("routing rule %s:\n", id)
	for _, d := range diffs {
		result += "  " + d + "\n"
	}
	return result
}

func fmtStrPtr(p *string) string {
	if p == nil {
		return "<nil>"
	}
	return *p
}

// Compile-time interface check.
var _ Reconciler = (*RoutingRuleReconciler)(nil)
