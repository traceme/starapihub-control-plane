package registry

import (
	"reflect"
	"strings"
	"testing"
)

// hasYAMLTag checks that a struct field has a yaml tag containing the expected key.
func hasYAMLTag(t *testing.T, st reflect.Type, fieldName, expectedTag string) {
	t.Helper()
	field, ok := st.FieldByName(fieldName)
	if !ok {
		t.Errorf("struct %s missing field %s", st.Name(), fieldName)
		return
	}
	tag := field.Tag.Get("yaml")
	if !strings.HasPrefix(tag, expectedTag) {
		t.Errorf("struct %s field %s: yaml tag = %q, want prefix %q", st.Name(), fieldName, tag, expectedTag)
	}
}

// hasJSONTag checks that a struct field has a json tag containing the expected key.
func hasJSONTag(t *testing.T, st reflect.Type, fieldName, expectedTag string) {
	t.Helper()
	field, ok := st.FieldByName(fieldName)
	if !ok {
		t.Errorf("struct %s missing field %s", st.Name(), fieldName)
		return
	}
	tag := field.Tag.Get("json")
	if tag == "" {
		t.Errorf("struct %s field %s: missing json tag", st.Name(), fieldName)
		return
	}
	if !strings.HasPrefix(tag, expectedTag) {
		t.Errorf("struct %s field %s: json tag = %q, want prefix %q", st.Name(), fieldName, tag, expectedTag)
	}
}

func TestLogicalModelYAMLTags(t *testing.T) {
	st := reflect.TypeOf(LogicalModel{})
	tags := map[string]string{
		"DisplayName":       "display_name",
		"BillingName":       "billing_name",
		"UpstreamModel":     "upstream_model",
		"RiskLevel":         "risk_level",
		"AllowedGroups":     "allowed_groups",
		"Channel":           "channel",
		"RoutePolicy":       "route_policy",
		"UnofficialAllowed": "unofficial_allowed",
		"CachingAllowed":    "caching_allowed",
	}
	for field, tag := range tags {
		hasYAMLTag(t, st, field, tag)
	}
}

func TestChannelDesiredYAMLAndJSONTags(t *testing.T) {
	st := reflect.TypeOf(ChannelDesired{})
	yamlTags := map[string]string{
		"Name":           "name",
		"Type":           "type",
		"KeyEnv":         "key_env",
		"BaseURL":        "base_url",
		"Models":         "models",
		"Group":          "group",
		"Tag":            "tag",
		"ModelMapping":   "model_mapping",
		"Priority":       "priority",
		"Weight":         "weight",
		"Status":         "status",
		"AutoBan":        "auto_ban",
		"Setting":        "setting",
		"ParamOverride":  "param_override",
		"HeaderOverride": "header_override",
	}
	for field, tag := range yamlTags {
		hasYAMLTag(t, st, field, tag)
	}

	// JSON tags for serializable fields
	jsonTags := map[string]string{
		"Name":   "name",
		"Type":   "type",
		"Models": "models",
		"Group":  "group",
		"Status": "status",
	}
	for field, tag := range jsonTags {
		hasJSONTag(t, st, field, tag)
	}
}

func TestBifrostProviderDesiredYAMLAndJSONTags(t *testing.T) {
	st := reflect.TypeOf(BifrostProviderDesired{})
	tags := map[string]string{
		"Keys":                     "keys",
		"NetworkConfig":            "network_config",
		"ConcurrencyAndBufferSize": "concurrency_and_buffer_size",
		"CustomProviderConfig":     "custom_provider_config",
	}
	for field, tag := range tags {
		hasYAMLTag(t, st, field, tag)
		hasJSONTag(t, st, field, tag)
	}
}

func TestRoutingRuleDesiredYAMLAndJSONTags(t *testing.T) {
	st := reflect.TypeOf(RoutingRuleDesired{})
	tags := map[string]string{
		"Name":          "name",
		"Enabled":       "enabled",
		"CelExpression": "cel_expression",
		"Targets":       "targets",
		"Fallbacks":     "fallbacks",
		"Scope":         "scope",
		"Priority":      "priority",
	}
	for field, tag := range tags {
		hasYAMLTag(t, st, field, tag)
		hasJSONTag(t, st, field, tag)
	}
}

func TestModelPricingYAMLAndJSONTags(t *testing.T) {
	st := reflect.TypeOf(ModelPricing{})
	tags := map[string]string{
		"ModelRatio":      "model_ratio",
		"ModelPrice":      "model_price",
		"CompletionRatio": "completion_ratio",
		"CacheRatio":      "cache_ratio",
	}
	for field, tag := range tags {
		hasYAMLTag(t, st, field, tag)
		hasJSONTag(t, st, field, tag)
	}
}

func TestOptionalPointerFieldsHaveOmitempty(t *testing.T) {
	// Check that all pointer fields have omitempty on both yaml and json tags
	types := []reflect.Type{
		reflect.TypeOf(LogicalModel{}),
		reflect.TypeOf(ChannelDesired{}),
		reflect.TypeOf(BifrostProviderDesired{}),
		reflect.TypeOf(BifrostKeyDesired{}),
		reflect.TypeOf(BifrostNetworkConfig{}),
		reflect.TypeOf(RoutingRuleDesired{}),
		reflect.TypeOf(RoutingTargetDesired{}),
		reflect.TypeOf(ModelPricing{}),
		reflect.TypeOf(BifrostClientConfig{}),
	}

	for _, st := range types {
		for i := 0; i < st.NumField(); i++ {
			field := st.Field(i)
			if field.Type.Kind() != reflect.Ptr {
				continue
			}
			yamlTag := field.Tag.Get("yaml")
			if yamlTag != "" && yamlTag != "-" && !strings.Contains(yamlTag, "omitempty") {
				t.Errorf("%s.%s: pointer field missing omitempty in yaml tag: %q", st.Name(), field.Name, yamlTag)
			}
			jsonTag := field.Tag.Get("json")
			if jsonTag != "" && jsonTag != "-" && !strings.Contains(jsonTag, "omitempty") {
				t.Errorf("%s.%s: pointer field missing omitempty in json tag: %q", st.Name(), field.Name, jsonTag)
			}
		}
	}
}
