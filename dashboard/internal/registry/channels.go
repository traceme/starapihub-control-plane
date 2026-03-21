package registry

import "encoding/json"

// ChannelsFile is the top-level YAML wrapper for channels.yaml.
type ChannelsFile struct {
	Channels map[string]ChannelDesired `yaml:"channels" json:"channels"`
}

// ChannelDesired is the desired state for a New-API channel.
// Derived from new-api/model/channel.go Channel struct (Phase 1 audit).
// Uses pointer types with omitempty for optional fields per CLAUDE.md convention.
type ChannelDesired struct {
	Name           string            `yaml:"name" json:"name"`
	Type           int               `yaml:"type" json:"type"`
	KeyEnv         string            `yaml:"key_env" json:"-"`
	BaseURL        *string           `yaml:"base_url,omitempty" json:"base_url,omitempty"`
	Models         string            `yaml:"models" json:"models"`
	Group          string            `yaml:"group" json:"group"`
	Tag            *string           `yaml:"tag,omitempty" json:"tag,omitempty"`
	ModelMapping   map[string]string `yaml:"model_mapping,omitempty" json:"-"`
	Priority       *int64            `yaml:"priority,omitempty" json:"priority,omitempty"`
	Weight         *uint             `yaml:"weight,omitempty" json:"weight,omitempty"`
	Status         int               `yaml:"status" json:"status"`
	AutoBan        *int              `yaml:"auto_ban,omitempty" json:"auto_ban,omitempty"`
	Setting        map[string]any    `yaml:"setting,omitempty" json:"-"`
	ParamOverride  map[string]any    `yaml:"param_override,omitempty" json:"-"`
	HeaderOverride map[string]string `yaml:"header_override,omitempty" json:"-"`
}

// NOTE: ModelMapping, Setting, ParamOverride, HeaderOverride are stored as
// maps in YAML for ergonomics but must be JSON-encoded to strings at sync time
// (Phase 3). The json:"-" tags prevent direct JSON serialization of the map form.
// Phase 3 will add MarshalJSON/ToAPIPayload methods.

// AddChannelRequest is the wrapper for POST /api/channel/ per audit finding.
// Used at sync time, not in YAML.
type AddChannelRequest struct {
	Mode    string          `json:"mode"`
	Channel json.RawMessage `json:"channel"`
}
