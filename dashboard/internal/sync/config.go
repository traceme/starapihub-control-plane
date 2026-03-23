package sync

// ConfigReconciler -- full implementation expected from plan 03-03.
// This stub exists to allow the sync package to compile while other
// reconcilers (channels, pricing) are being built in plan 03-04.

import (
	"fmt"
	"reflect"
	"strings"

	"github.com/starapihub/dashboard/internal/registry"
	"github.com/starapihub/dashboard/internal/upstream"
)

// BifrostConfigClient is the interface the config reconciler needs.
type BifrostConfigClient interface {
	GetConfigTyped() (*upstream.BifrostConfigResponse, error)
	UpdateConfigTyped(config *upstream.BifrostConfigResponse) error
}

// ConfigReconciler reconciles Bifrost global config via PUT /api/config.
type ConfigReconciler struct {
	client BifrostConfigClient
}

// NewConfigReconciler creates a ConfigReconciler.
func NewConfigReconciler(client BifrostConfigClient) *ConfigReconciler {
	return &ConfigReconciler{client: client}
}

// Name returns the resource type name.
func (r *ConfigReconciler) Name() string {
	return "config"
}

// Plan compares desired vs live Bifrost config and returns actions.
func (r *ConfigReconciler) Plan(desired, live any) ([]Action, error) {
	desiredCfg, ok := desired.(*registry.BifrostClientConfig)
	if !ok {
		return nil, fmt.Errorf("ConfigReconciler.Plan: desired must be *registry.BifrostClientConfig, got %T", desired)
	}
	// Nil guard: when no config section exists in providers.yaml, buildDesiredState
	// passes (*BifrostClientConfig)(nil). The type assertion succeeds (typed nil),
	// but reflect.ValueOf would panic on Elem(). Return no-op.
	if desiredCfg == nil {
		return nil, nil
	}
	liveCfg, ok := live.(*upstream.BifrostConfigResponse)
	if !ok {
		return nil, fmt.Errorf("ConfigReconciler.Plan: live must be *upstream.BifrostConfigResponse, got %T", live)
	}

	// Compare only fields that are non-nil in desired
	dv := reflect.ValueOf(desiredCfg).Elem()
	lv := reflect.ValueOf(liveCfg).Elem()
	dt := dv.Type()

	var diffs []string
	for i := 0; i < dt.NumField(); i++ {
		df := dv.Field(i)
		if df.IsNil() {
			continue
		}
		lf := lv.Field(i)
		if lf.IsNil() || df.Elem().Interface() != lf.Elem().Interface() {
			diffs = append(diffs, dt.Field(i).Name)
		}
	}

	if len(diffs) == 0 {
		return nil, nil
	}

	return []Action{{
		Type:         ActionUpdate,
		ResourceType: "config",
		ResourceID:   "bifrost-global",
		Desired:      desiredCfg,
		Live:         liveCfg,
		Diff:         fmt.Sprintf("changed fields: %s", strings.Join(diffs, ", ")),
	}}, nil
}

// Apply sends the config update to Bifrost.
func (r *ConfigReconciler) Apply(action Action) (*Result, error) {
	desiredCfg, ok := action.Desired.(*registry.BifrostClientConfig)
	if !ok {
		return &Result{Action: action, Status: StatusFailed, Error: fmt.Errorf("desired is not *BifrostClientConfig")}, nil
	}

	// Build update payload with only non-nil desired fields
	update := &upstream.BifrostConfigResponse{}
	dv := reflect.ValueOf(desiredCfg).Elem()
	uv := reflect.ValueOf(update).Elem()
	dt := dv.Type()

	for i := 0; i < dt.NumField(); i++ {
		df := dv.Field(i)
		if !df.IsNil() {
			uv.Field(i).Set(df)
		}
	}

	if err := r.client.UpdateConfigTyped(update); err != nil {
		return &Result{Action: action, Status: StatusFailed, Error: err}, fmt.Errorf("config apply: %w", err)
	}

	// Read back
	readBack, err := r.client.GetConfigTyped()
	if err != nil {
		return &Result{Action: action, Status: StatusUnverified, DriftMsg: fmt.Sprintf("read-back failed: %v", err)}, nil
	}
	return &Result{Action: action, Status: StatusOK, ReadBack: readBack}, nil
}

// Verify reads back config and confirms desired fields match.
func (r *ConfigReconciler) Verify(action Action, result *Result) error {
	desiredCfg, ok := action.Desired.(*registry.BifrostClientConfig)
	if !ok {
		return fmt.Errorf("desired is not *BifrostClientConfig")
	}

	liveCfg, err := r.client.GetConfigTyped()
	if err != nil {
		result.Status = StatusUnverified
		result.DriftMsg = fmt.Sprintf("verify read-back failed: %v", err)
		return nil
	}
	result.ReadBack = liveCfg

	// Check each non-nil desired field matches live
	dv := reflect.ValueOf(desiredCfg).Elem()
	lv := reflect.ValueOf(liveCfg).Elem()
	dt := dv.Type()

	var drifts []string
	for i := 0; i < dt.NumField(); i++ {
		df := dv.Field(i)
		if df.IsNil() {
			continue
		}
		lf := lv.Field(i)
		if lf.IsNil() || df.Elem().Interface() != lf.Elem().Interface() {
			drifts = append(drifts, fmt.Sprintf("%s: desired=%v live=%v", dt.Field(i).Name, df.Elem().Interface(), lf.Interface()))
		}
	}

	if len(drifts) > 0 {
		result.Status = StatusAppliedWithDrift
		result.DriftMsg = strings.Join(drifts, "; ")
	}
	return nil
}

// Compile-time interface check.
var _ Reconciler = (*ConfigReconciler)(nil)
