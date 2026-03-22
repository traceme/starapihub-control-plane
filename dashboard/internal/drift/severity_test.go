package drift_test

import (
	"testing"

	"github.com/starapihub/dashboard/internal/drift"
)

func TestMaxSeverity(t *testing.T) {
	tests := []struct {
		a, b drift.DriftSeverity
		want drift.DriftSeverity
	}{
		{drift.SeverityInformational, drift.SeverityInformational, drift.SeverityInformational},
		{drift.SeverityInformational, drift.SeverityWarning, drift.SeverityWarning},
		{drift.SeverityWarning, drift.SeverityInformational, drift.SeverityWarning},
		{drift.SeverityWarning, drift.SeverityBlocking, drift.SeverityBlocking},
		{drift.SeverityBlocking, drift.SeverityWarning, drift.SeverityBlocking},
		{drift.SeverityBlocking, drift.SeverityBlocking, drift.SeverityBlocking},
	}
	for _, tt := range tests {
		got := drift.MaxSeverity(tt.a, tt.b)
		if got != tt.want {
			t.Errorf("MaxSeverity(%s, %s) = %s, want %s", tt.a, tt.b, got, tt.want)
		}
	}
}

func TestLookupFieldSeverity(t *testing.T) {
	tests := []struct {
		name         string
		resourceType string
		fieldPath    string
		want         drift.DriftSeverity
	}{
		{"channel base_url exact", "channel", "base_url", drift.SeverityBlocking},
		{"channel weight exact", "channel", "weight", drift.SeverityWarning},
		{"channel id exact", "channel", "id", drift.SeverityInformational},
		{"provider keys prefix", "provider", "Keys.0.Models", drift.SeverityBlocking},
		{"routing-rule targets", "routing-rule", "targets", drift.SeverityBlocking},
		{"pricing model_ratio", "pricing", "model_ratio", drift.SeverityBlocking},
		{"pricing cache_ratio", "pricing", "cache_ratio", drift.SeverityWarning},
		{"cookie no_valid_cookies", "cookie", "no_valid_cookies", drift.SeverityBlocking},
		{"cookie exhausted_count", "cookie", "exhausted_count", drift.SeverityInformational},
		{"config max_retries", "config", "max_retries", drift.SeverityWarning},
		{"unknown resource", "unknown", "something", drift.SeverityWarning},
		{"unknown field defaults warning", "channel", "some_unknown_field", drift.SeverityWarning},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := drift.LookupFieldSeverity(tt.resourceType, tt.fieldPath)
			if got != tt.want {
				t.Errorf("LookupFieldSeverity(%s, %s) = %s, want %s",
					tt.resourceType, tt.fieldPath, got, tt.want)
			}
		})
	}
}

func TestDefaultSeverityForAction(t *testing.T) {
	tests := []struct {
		name         string
		resourceType string
		actionType   string
		want         drift.DriftSeverity
	}{
		{"channel create blocking", "channel", "create", drift.SeverityBlocking},
		{"channel delete blocking", "channel", "delete", drift.SeverityBlocking},
		{"provider create blocking", "provider", "create", drift.SeverityBlocking},
		{"routing-rule delete blocking", "routing-rule", "delete", drift.SeverityBlocking},
		{"config create warning", "config", "create", drift.SeverityWarning},
		{"pricing delete warning", "pricing", "delete", drift.SeverityWarning},
		{"cookie create warning", "cookie", "create", drift.SeverityWarning},
		{"no-change informational", "channel", "no-change", drift.SeverityInformational},
		{"update fallback warning", "channel", "update", drift.SeverityWarning},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := drift.DefaultSeverityForAction(tt.resourceType, tt.actionType)
			if got != tt.want {
				t.Errorf("DefaultSeverityForAction(%s, %s) = %s, want %s",
					tt.resourceType, tt.actionType, got, tt.want)
			}
		})
	}
}

func TestChannelFieldSeverityCount(t *testing.T) {
	if len(drift.ChannelFieldSeverity) < 15 {
		t.Errorf("ChannelFieldSeverity has %d entries, want at least 15", len(drift.ChannelFieldSeverity))
	}
}

func TestAllResourceTypesHaveSeverityMap(t *testing.T) {
	expected := []string{"channel", "provider", "config", "routing-rule", "pricing", "cookie"}
	for _, rt := range expected {
		if _, ok := drift.ResourceSeverityMap[rt]; !ok {
			t.Errorf("ResourceSeverityMap missing entry for %q", rt)
		}
	}
}
