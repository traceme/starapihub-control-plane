package buildinfo

import "testing"

func TestMode_Upstream(t *testing.T) {
	got := Mode(func(k string) string {
		if k == "STARAPIHUB_MODE" {
			return "upstream"
		}
		return ""
	})
	if got != "upstream" {
		t.Errorf("expected upstream, got %s", got)
	}
}

func TestMode_Appliance(t *testing.T) {
	got := Mode(func(k string) string {
		if k == "STARAPIHUB_MODE" {
			return "appliance"
		}
		return ""
	})
	if got != "appliance" {
		t.Errorf("expected appliance, got %s", got)
	}
}

func TestMode_Empty(t *testing.T) {
	got := Mode(func(string) string { return "" })
	if got != "unknown" {
		t.Errorf("expected unknown, got %s", got)
	}
}

func TestMode_Garbage(t *testing.T) {
	got := Mode(func(k string) string {
		if k == "STARAPIHUB_MODE" {
			return "something-else"
		}
		return ""
	})
	if got != "unknown" {
		t.Errorf("expected unknown, got %s", got)
	}
}

func TestInfo_ContainsAllKeys(t *testing.T) {
	info := Info(func(string) string { return "" })
	for _, key := range []string{"version", "build_date", "go_version", "mode"} {
		if _, ok := info[key]; !ok {
			t.Errorf("missing key %s in Info()", key)
		}
	}
}

func TestInfo_ReflectsVars(t *testing.T) {
	old := Version
	defer func() { Version = old }()

	Version = "1.2.3-test"
	info := Info(func(string) string { return "" })
	if info["version"] != "1.2.3-test" {
		t.Errorf("expected 1.2.3-test, got %s", info["version"])
	}
}
