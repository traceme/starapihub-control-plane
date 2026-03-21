package registry

// RoutingRulesFile is the top-level YAML wrapper for routing-rules.yaml.
type RoutingRulesFile struct {
	Rules map[string]RoutingRuleDesired `yaml:"routing_rules" json:"routing_rules"`
}

// RoutingRuleDesired is the desired state for a Bifrost routing rule.
// Derived from bifrost/framework/configstore/tables/routing_rules.go.
type RoutingRuleDesired struct {
	Name          string                 `yaml:"name" json:"name"`
	Description   string                 `yaml:"description,omitempty" json:"description,omitempty"`
	Enabled       bool                   `yaml:"enabled" json:"enabled"`
	CelExpression string                 `yaml:"cel_expression" json:"cel_expression"`
	Targets       []RoutingTargetDesired `yaml:"targets,omitempty" json:"targets,omitempty"`
	Fallbacks     []string               `yaml:"fallbacks,omitempty" json:"fallbacks,omitempty"`
	Query         map[string]any         `yaml:"query,omitempty" json:"query,omitempty"`
	Scope         string                 `yaml:"scope" json:"scope"`
	ScopeID       *string                `yaml:"scope_id,omitempty" json:"scope_id,omitempty"`
	Priority      int                    `yaml:"priority" json:"priority"`
}

// RoutingTargetDesired is a target within a Bifrost routing rule.
type RoutingTargetDesired struct {
	Provider *string `yaml:"provider,omitempty" json:"provider,omitempty"`
	Model    *string `yaml:"model,omitempty" json:"model,omitempty"`
	KeyID    *string `yaml:"key_id,omitempty" json:"key_id,omitempty"`
	Weight   float64 `yaml:"weight" json:"weight"`
}
