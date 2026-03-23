package sync

import (
	"encoding/json"
	"fmt"
	"log"
	"strconv"

	"github.com/starapihub/dashboard/internal/registry"
	"github.com/starapihub/dashboard/internal/upstream"
)

// NewAPIChannelClient is the interface the channel reconciler needs from the upstream client.
type NewAPIChannelClient interface {
	ListChannelsTyped(adminToken string) ([]upstream.ChannelResponse, error)
	GetChannelTyped(adminToken string, id int) (*upstream.ChannelResponse, error)
	CreateChannel(adminToken string, channel json.RawMessage) (json.RawMessage, error)
	UpdateChannelTyped(adminToken string, channel json.RawMessage) error
	DeleteChannel(adminToken string, id string) error
}

// KeyResolver is a function that resolves an env var name to a key value.
type KeyResolver func(envName string) string

// ChannelReconciler reconciles New-API channels using name-based matching.
type ChannelReconciler struct {
	client     NewAPIChannelClient
	adminToken string
	prune      bool
	resolveKey KeyResolver
}

// NewChannelReconciler creates a ChannelReconciler.
func NewChannelReconciler(client NewAPIChannelClient, adminToken string, prune bool, resolveKey KeyResolver) *ChannelReconciler {
	return &ChannelReconciler{
		client:     client,
		adminToken: adminToken,
		prune:      prune,
		resolveKey: resolveKey,
	}
}

// Name returns the resource type name.
func (r *ChannelReconciler) Name() string {
	return "channel"
}

// Plan compares desired channels against live channels and returns reconciliation actions.
// desired must be map[string]registry.ChannelDesired (keyed by channel name).
// live must be []upstream.ChannelResponse.
// Channels are matched by the Name field, not by auto-increment ID.
func (r *ChannelReconciler) Plan(desired, live any) ([]Action, error) {
	desiredMap, ok := desired.(map[string]registry.ChannelDesired)
	if !ok {
		return nil, fmt.Errorf("ChannelReconciler.Plan: desired must be map[string]registry.ChannelDesired, got %T", desired)
	}
	liveSlice, ok := live.([]upstream.ChannelResponse)
	if !ok {
		return nil, fmt.Errorf("ChannelReconciler.Plan: live must be []upstream.ChannelResponse, got %T", live)
	}

	// Build live map keyed by Name
	liveByName := make(map[string]upstream.ChannelResponse, len(liveSlice))
	for _, ch := range liveSlice {
		liveByName[ch.Name] = ch
	}

	var actions []Action

	// Check each desired channel
	for name, desired := range desiredMap {
		// Match by desired.Name (display name), not the registry key
		liveCh, exists := liveByName[desired.Name]
		if !exists {
			actions = append(actions, Action{
				Type:         ActionCreate,
				ResourceType: "channel",
				ResourceID:   name,
				Desired:      desired,
				Live:         nil,
			})
			continue
		}

		// Compare: normalize live to desired-comparable form
		if channelNeedUpdate(desired, liveCh) {
			actions = append(actions, Action{
				Type:         ActionUpdate,
				ResourceType: "channel",
				ResourceID:   name,
				Desired:      desired,
				Live:         liveCh,
				Diff:         fmt.Sprintf("channel %s differs from desired state", name),
			})
		}
	}

	// Prune: live channels not in desired
	// Build set of desired display names for reverse lookup
	desiredNames := make(map[string]bool, len(desiredMap))
	for _, d := range desiredMap {
		desiredNames[d.Name] = true
	}
	if r.prune {
		for name, liveCh := range liveByName {
			if !desiredNames[name] {
				if liveCh.UsedQuota > 0 {
					log.Printf("WARNING: skipping delete of channel %s (has billing history: used_quota=%d)", name, liveCh.UsedQuota)
					continue
				}
				actions = append(actions, Action{
					Type:         ActionDelete,
					ResourceType: "channel",
					ResourceID:   name,
					Desired:      nil,
					Live:         liveCh,
				})
			}
		}
	}

	return actions, nil
}

// channelNeedUpdate compares desired against live to detect drift.
// Only compares fields that are settable via the API (ignores server-side fields).
func channelNeedUpdate(desired registry.ChannelDesired, live upstream.ChannelResponse) bool {
	if desired.Type != live.Type {
		return true
	}
	if desired.Models != live.Models {
		return true
	}
	if desired.Group != live.Group {
		return true
	}
	if desired.Status != live.Status {
		return true
	}

	// Compare optional pointer fields
	if !ptrStringEq(desired.BaseURL, live.BaseURL) {
		return true
	}
	if !ptrStringEq(desired.Tag, live.Tag) {
		return true
	}
	if !ptrInt64Eq(desired.Priority, live.Priority) {
		return true
	}
	if !ptrUintEq(desired.Weight, live.Weight) {
		return true
	}
	if !ptrIntEq(desired.AutoBan, live.AutoBan) {
		return true
	}

	// Compare map fields (desired maps vs live JSON strings)
	if !mapMatchesJSONString(desired.ModelMapping, live.ModelMapping) {
		return true
	}
	if !mapAnyMatchesJSONString(desired.Setting, live.Setting) {
		return true
	}
	if !mapAnyMatchesJSONString(desired.ParamOverride, live.ParamOverride) {
		return true
	}
	if !mapStringMatchesJSONString(desired.HeaderOverride, live.HeaderOverride) {
		return true
	}

	return false
}

func ptrStringEq(a, b *string) bool {
	if a == nil && b == nil {
		return true
	}
	if a == nil || b == nil {
		return false
	}
	return *a == *b
}

func ptrInt64Eq(a, b *int64) bool {
	if a == nil && b == nil {
		return true
	}
	if a == nil || b == nil {
		return false
	}
	return *a == *b
}

func ptrUintEq(a, b *uint) bool {
	if a == nil && b == nil {
		return true
	}
	if a == nil || b == nil {
		return false
	}
	return *a == *b
}

func ptrIntEq(a, b *int) bool {
	if a == nil && b == nil {
		return true
	}
	if a == nil || b == nil {
		return false
	}
	return *a == *b
}

// mapMatchesJSONString compares map[string]string against a JSON-encoded *string.
func mapMatchesJSONString(m map[string]string, jsonStr *string) bool {
	if len(m) == 0 && (jsonStr == nil || *jsonStr == "" || *jsonStr == "null") {
		return true
	}
	if len(m) == 0 || jsonStr == nil {
		return false
	}
	var liveMap map[string]string
	if err := json.Unmarshal([]byte(*jsonStr), &liveMap); err != nil {
		return false
	}
	if len(m) != len(liveMap) {
		return false
	}
	for k, v := range m {
		if liveMap[k] != v {
			return false
		}
	}
	return true
}

// mapAnyMatchesJSONString compares map[string]any against a JSON-encoded *string.
func mapAnyMatchesJSONString(m map[string]any, jsonStr *string) bool {
	if len(m) == 0 && (jsonStr == nil || *jsonStr == "" || *jsonStr == "null") {
		return true
	}
	if len(m) == 0 || jsonStr == nil {
		return false
	}
	// Marshal both sides to canonical JSON for comparison
	desiredJSON, err := json.Marshal(m)
	if err != nil {
		return false
	}
	var desiredNorm, liveNorm any
	json.Unmarshal(desiredJSON, &desiredNorm)
	json.Unmarshal([]byte(*jsonStr), &liveNorm)
	d1, _ := json.Marshal(desiredNorm)
	d2, _ := json.Marshal(liveNorm)
	return string(d1) == string(d2)
}

// mapStringMatchesJSONString compares map[string]string against a JSON-encoded *string.
func mapStringMatchesJSONString(m map[string]string, jsonStr *string) bool {
	if len(m) == 0 && (jsonStr == nil || *jsonStr == "" || *jsonStr == "null") {
		return true
	}
	if len(m) == 0 || jsonStr == nil {
		return false
	}
	desiredJSON, err := json.Marshal(m)
	if err != nil {
		return false
	}
	var desiredNorm, liveNorm any
	json.Unmarshal(desiredJSON, &desiredNorm)
	json.Unmarshal([]byte(*jsonStr), &liveNorm)
	d1, _ := json.Marshal(desiredNorm)
	d2, _ := json.Marshal(liveNorm)
	return string(d1) == string(d2)
}

// Apply executes a single channel reconciliation action.
func (r *ChannelReconciler) Apply(action Action) (*Result, error) {
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

func (r *ChannelReconciler) applyCreate(action Action) (*Result, error) {
	desired, ok := action.Desired.(registry.ChannelDesired)
	if !ok {
		return &Result{Action: action, Status: StatusFailed, Error: fmt.Errorf("desired is not ChannelDesired")}, nil
	}

	key := r.resolveKey(desired.KeyEnv)
	payload, err := desired.ToAPIPayload(key)
	if err != nil {
		return &Result{Action: action, Status: StatusFailed, Error: err}, nil
	}

	payloadJSON, err := json.Marshal(payload)
	if err != nil {
		return &Result{Action: action, Status: StatusFailed, Error: err}, nil
	}

	// Wrap in AddChannelRequest
	wrapper := registry.AddChannelRequest{
		Mode:    "single",
		Channel: payloadJSON,
	}
	wrapperJSON, err := json.Marshal(wrapper)
	if err != nil {
		return &Result{Action: action, Status: StatusFailed, Error: err}, nil
	}

	_, err = r.client.CreateChannel(r.adminToken, wrapperJSON)
	if err != nil {
		return &Result{Action: action, Status: StatusFailed, Error: err}, nil
	}

	return &Result{Action: action, Status: StatusOK}, nil
}

func (r *ChannelReconciler) applyUpdate(action Action) (*Result, error) {
	desired, ok := action.Desired.(registry.ChannelDesired)
	if !ok {
		return &Result{Action: action, Status: StatusFailed, Error: fmt.Errorf("desired is not ChannelDesired")}, nil
	}
	liveCh, ok := action.Live.(upstream.ChannelResponse)
	if !ok {
		return &Result{Action: action, Status: StatusFailed, Error: fmt.Errorf("live is not ChannelResponse")}, nil
	}

	key := r.resolveKey(desired.KeyEnv)
	payload, err := desired.ToAPIPayload(key)
	if err != nil {
		return &Result{Action: action, Status: StatusFailed, Error: err}, nil
	}

	// Set the live channel ID for the PUT
	payload.ID = liveCh.ID

	payloadJSON, err := json.Marshal(payload)
	if err != nil {
		return &Result{Action: action, Status: StatusFailed, Error: err}, nil
	}

	err = r.client.UpdateChannelTyped(r.adminToken, payloadJSON)
	if err != nil {
		return &Result{Action: action, Status: StatusFailed, Error: err}, nil
	}

	return &Result{Action: action, Status: StatusOK}, nil
}

func (r *ChannelReconciler) applyDelete(action Action) (*Result, error) {
	liveCh, ok := action.Live.(upstream.ChannelResponse)
	if !ok {
		return &Result{Action: action, Status: StatusFailed, Error: fmt.Errorf("live is not ChannelResponse")}, nil
	}

	err := r.client.DeleteChannel(r.adminToken, strconv.Itoa(liveCh.ID))
	if err != nil {
		return &Result{Action: action, Status: StatusFailed, Error: err}, nil
	}

	return &Result{Action: action, Status: StatusOK}, nil
}

// Verify reads back the resource after Apply and checks it matches expected state.
func (r *ChannelReconciler) Verify(action Action, result *Result) error {
	switch action.Type {
	case ActionCreate:
		return r.verifyCreate(action, result)
	case ActionUpdate:
		return r.verifyUpdate(action, result)
	case ActionDelete:
		return r.verifyDelete(action, result)
	}
	return nil
}

func (r *ChannelReconciler) verifyCreate(action Action, result *Result) error {
	channels, err := r.client.ListChannelsTyped(r.adminToken)
	if err != nil {
		result.Status = StatusUnverified
		result.DriftMsg = fmt.Sprintf("read-back failed: %v", err)
		return nil
	}
	result.ReadBack = channels

	for _, ch := range channels {
		if ch.Name == action.ResourceID {
			return nil
		}
	}
	result.Status = StatusAppliedWithDrift
	result.DriftMsg = fmt.Sprintf("channel %s not found in read-back after create", action.ResourceID)
	return nil
}

func (r *ChannelReconciler) verifyUpdate(action Action, result *Result) error {
	liveCh, ok := action.Live.(upstream.ChannelResponse)
	if !ok {
		return fmt.Errorf("live is not ChannelResponse")
	}

	ch, err := r.client.GetChannelTyped(r.adminToken, liveCh.ID)
	if err != nil {
		result.Status = StatusUnverified
		result.DriftMsg = fmt.Sprintf("read-back failed: %v", err)
		return nil
	}
	result.ReadBack = ch

	desired, ok := action.Desired.(registry.ChannelDesired)
	if !ok {
		return nil
	}

	// Verify key fields match
	if ch.Name != desired.Name || ch.Models != desired.Models || ch.Group != desired.Group {
		result.Status = StatusAppliedWithDrift
		result.DriftMsg = fmt.Sprintf("channel %s field mismatch after update", action.ResourceID)
	}
	return nil
}

func (r *ChannelReconciler) verifyDelete(action Action, result *Result) error {
	channels, err := r.client.ListChannelsTyped(r.adminToken)
	if err != nil {
		result.Status = StatusUnverified
		result.DriftMsg = fmt.Sprintf("read-back failed: %v", err)
		return nil
	}
	result.ReadBack = channels

	for _, ch := range channels {
		if ch.Name == action.ResourceID {
			result.Status = StatusAppliedWithDrift
			result.DriftMsg = fmt.Sprintf("channel %s still present after delete", action.ResourceID)
			return nil
		}
	}
	return nil
}

// Compile-time interface check.
var _ Reconciler = (*ChannelReconciler)(nil)
