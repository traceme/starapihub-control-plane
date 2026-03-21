package registry

// PricingFile is the top-level YAML wrapper for pricing.yaml.
type PricingFile struct {
	Pricing map[string]ModelPricing `yaml:"pricing" json:"pricing"`
}

// ModelPricing is the desired state for model pricing in New-API.
// Synced via PUT /api/option/ with keys: ModelRatio, ModelPrice, CompletionRatio.
type ModelPricing struct {
	ModelRatio      *float64 `yaml:"model_ratio,omitempty" json:"model_ratio,omitempty"`
	ModelPrice      *float64 `yaml:"model_price,omitempty" json:"model_price,omitempty"`
	CompletionRatio *float64 `yaml:"completion_ratio,omitempty" json:"completion_ratio,omitempty"`
	CacheRatio      *float64 `yaml:"cache_ratio,omitempty" json:"cache_ratio,omitempty"`
}
