package drift

import "strings"

// FieldSeverity maps field paths to their drift severity classification.
type FieldSeverity map[string]DriftSeverity

// ChannelFieldSeverity defines severity for New-API channel fields.
var ChannelFieldSeverity = FieldSeverity{
	// Blocking: changes that affect routing correctness
	"base_url":         SeverityBlocking,
	"key":              SeverityBlocking,
	"models":           SeverityBlocking,
	"model_mapping":    SeverityBlocking,
	"type":             SeverityBlocking,
	"group":            SeverityBlocking,
	"setting":          SeverityBlocking,
	"param_override":   SeverityBlocking,
	"header_override":  SeverityBlocking,

	// Warning: operational parameters
	"weight":    SeverityWarning,
	"priority":  SeverityWarning,
	"status":    SeverityWarning,
	"auto_ban":  SeverityWarning,
	"tag":       SeverityWarning,

	// Informational: read-only or cosmetic
	"id":            SeverityInformational,
	"created_time":  SeverityInformational,
	"test_time":     SeverityInformational,
	"response_time": SeverityInformational,
	"used_quota":    SeverityInformational,
	"balance":       SeverityInformational,
}

// ProviderFieldSeverity defines severity for Bifrost provider fields.
var ProviderFieldSeverity = FieldSeverity{
	// Blocking: credential and config changes
	"keys":                    SeverityBlocking,
	"custom_provider_config":  SeverityBlocking,

	// Warning: network and performance tuning
	"network_config":              SeverityWarning,
	"concurrency_and_buffer_size": SeverityWarning,

	// Informational
	"description": SeverityInformational,
}

// ConfigFieldSeverity defines severity for Bifrost config fields.
// All config fields are warning level -- notable but not routing-critical.
var ConfigFieldSeverity = FieldSeverity{
	"max_retries":                         SeverityWarning,
	"retry_backoff_initial":               SeverityWarning,
	"retry_backoff_max":                   SeverityWarning,
	"default_request_timeout_in_seconds":  SeverityWarning,
	"stream_idle_timeout_in_seconds":      SeverityWarning,
	"initial_pool_size":                   SeverityWarning,
	"max_idle_conns_per_host":             SeverityWarning,
	"proxy_url":                           SeverityWarning,
}

// RoutingRuleFieldSeverity defines severity for routing rule fields.
var RoutingRuleFieldSeverity = FieldSeverity{
	// Blocking: changes that alter routing behavior
	"cel_expression": SeverityBlocking,
	"targets":        SeverityBlocking,
	"enabled":        SeverityBlocking,
	"scope":          SeverityBlocking,
	"scope_id":       SeverityBlocking,
	"query":          SeverityBlocking,

	// Warning: ordering and fallback
	"priority":  SeverityWarning,
	"fallbacks": SeverityWarning,

	// Informational: metadata
	"description": SeverityInformational,
	"id":          SeverityInformational,
	"name":        SeverityInformational,
}

// PricingFieldSeverity defines severity for pricing/model ratio fields.
var PricingFieldSeverity = FieldSeverity{
	// Blocking: affects billing correctness
	"model_ratio":      SeverityBlocking,
	"model_price":      SeverityBlocking,
	"completion_ratio": SeverityBlocking,

	// Warning
	"cache_ratio": SeverityWarning,
}

// CookieFieldSeverity defines severity for ClewdR cookie status fields.
var CookieFieldSeverity = FieldSeverity{
	// Blocking: service availability
	"no_valid_cookies": SeverityBlocking,

	// Warning: degraded pool
	"missing_cookies": SeverityWarning,

	// Informational: metrics
	"exhausted_count": SeverityInformational,
	"invalid_count":   SeverityInformational,
}

// ResourceSeverityMap maps resource type names to their field severity tables.
var ResourceSeverityMap = map[string]FieldSeverity{
	"channel":      ChannelFieldSeverity,
	"provider":     ProviderFieldSeverity,
	"config":       ConfigFieldSeverity,
	"routing-rule": RoutingRuleFieldSeverity,
	"pricing":      PricingFieldSeverity,
	"cookie":       CookieFieldSeverity,
}

// DefaultSeverityForAction returns the default severity for a resource+action
// combination when no field-level classification is possible.
func DefaultSeverityForAction(resourceType string, actionType string) DriftSeverity {
	if actionType == "no-change" {
		return SeverityInformational
	}

	// create/delete on critical resources default to blocking
	switch resourceType {
	case "channel", "provider", "routing-rule":
		if actionType == "create" || actionType == "delete" {
			return SeverityBlocking
		}
	}

	// create/delete on config/pricing/cookie defaults to warning
	switch resourceType {
	case "config", "pricing", "cookie":
		if actionType == "create" || actionType == "delete" {
			return SeverityWarning
		}
	}

	// Fallback for update or unknown combos
	return SeverityWarning
}

// LookupFieldSeverity looks up the severity for a specific field within a resource type.
// It first tries an exact match, then tries prefix matching (e.g., "Keys.0.Models"
// matches "keys" via case-insensitive prefix). Returns warning as safe default if nothing matches.
func LookupFieldSeverity(resourceType, fieldPath string) DriftSeverity {
	fieldMap, ok := ResourceSeverityMap[resourceType]
	if !ok {
		return SeverityWarning
	}

	// Exact match (case-insensitive)
	lower := strings.ToLower(fieldPath)
	for key, sev := range fieldMap {
		if strings.ToLower(key) == lower {
			return sev
		}
	}

	// Prefix match: the fieldPath may be a dotted path like "Keys.0.Models"
	// Try matching against known field keys as prefixes of the lowered path
	lowerPath := strings.ToLower(fieldPath)
	for key, sev := range fieldMap {
		lowerKey := strings.ToLower(key)
		if strings.HasPrefix(lowerPath, lowerKey) {
			return sev
		}
	}

	// Safe default
	return SeverityWarning
}
