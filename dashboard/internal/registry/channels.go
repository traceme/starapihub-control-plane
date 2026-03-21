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

// ChannelAPIPayload is the JSON representation sent to New-API POST/PUT /api/channel/.
// It converts YAML map fields to JSON-encoded strings as required by New-API.
type ChannelAPIPayload struct {
	ID             int     `json:"id,omitempty"`
	Name           string  `json:"name"`
	Type           int     `json:"type"`
	Key            string  `json:"key"`
	BaseURL        *string `json:"base_url,omitempty"`
	Models         string  `json:"models"`
	Group          string  `json:"group"`
	Tag            *string `json:"tag,omitempty"`
	ModelMapping   *string `json:"model_mapping,omitempty"`
	Priority       *int64  `json:"priority,omitempty"`
	Weight         *uint   `json:"weight,omitempty"`
	Status         int     `json:"status"`
	AutoBan        *int    `json:"auto_ban,omitempty"`
	Setting        *string `json:"setting,omitempty"`
	ParamOverride  *string `json:"param_override,omitempty"`
	HeaderOverride *string `json:"header_override,omitempty"`
}

// ToAPIPayload converts a ChannelDesired to a ChannelAPIPayload suitable for New-API.
// resolvedKey is the actual API key value (caller resolves KeyEnv via env lookup).
// Map fields (ModelMapping, Setting, ParamOverride, HeaderOverride) are JSON-encoded to strings.
func (c *ChannelDesired) ToAPIPayload(resolvedKey string) (*ChannelAPIPayload, error) {
	p := &ChannelAPIPayload{
		Name:     c.Name,
		Type:     c.Type,
		Key:      resolvedKey,
		BaseURL:  c.BaseURL,
		Models:   c.Models,
		Group:    c.Group,
		Tag:      c.Tag,
		Priority: c.Priority,
		Weight:   c.Weight,
		Status:   c.Status,
		AutoBan:  c.AutoBan,
	}

	if len(c.ModelMapping) > 0 {
		b, err := json.Marshal(c.ModelMapping)
		if err != nil {
			return nil, err
		}
		s := string(b)
		p.ModelMapping = &s
	}

	if len(c.Setting) > 0 {
		b, err := json.Marshal(c.Setting)
		if err != nil {
			return nil, err
		}
		s := string(b)
		p.Setting = &s
	}

	if len(c.ParamOverride) > 0 {
		b, err := json.Marshal(c.ParamOverride)
		if err != nil {
			return nil, err
		}
		s := string(b)
		p.ParamOverride = &s
	}

	if len(c.HeaderOverride) > 0 {
		b, err := json.Marshal(c.HeaderOverride)
		if err != nil {
			return nil, err
		}
		s := string(b)
		p.HeaderOverride = &s
	}

	return p, nil
}

// AddChannelRequest is the wrapper for POST /api/channel/ per audit finding.
// Used at sync time, not in YAML.
type AddChannelRequest struct {
	Mode    string          `json:"mode"`
	Channel json.RawMessage `json:"channel"`
}
