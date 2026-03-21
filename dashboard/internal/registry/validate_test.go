package registry

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// writeFile is a test helper that writes content to a file in the given dir.
func writeFile(t *testing.T, dir, name, content string) string {
	t.Helper()
	p := filepath.Join(dir, name)
	if err := os.WriteFile(p, []byte(content), 0644); err != nil {
		t.Fatalf("write %s: %v", p, err)
	}
	return p
}

func TestValidateSchemaValid(t *testing.T) {
	tmp := t.TempDir()

	yamlContent := `models:
  test-model:
    display_name: "Test Model"
    billing_name: "test"
    upstream_model: "gpt-4o"
    risk_level: low
    allowed_groups: ["all"]
    channel: "default"
    route_policy: "standard"
`

	schemaDir := filepath.Join(tmp, "schemas")
	os.MkdirAll(schemaDir, 0755)

	// Copy the real schema
	realSchema, err := os.ReadFile(filepath.Join("..", "..", "..", "..", "schemas", "models.schema.json"))
	if err != nil {
		// Try relative from test working dir
		realSchema, err = os.ReadFile(filepath.Join(findRepoRoot(t), "control-plane", "schemas", "models.schema.json"))
		if err != nil {
			t.Fatalf("read schema: %v", err)
		}
	}
	schemaPath := writeFile(t, schemaDir, "models.schema.json", string(realSchema))

	err = ValidateYAML([]byte(yamlContent), schemaPath)
	if err != nil {
		t.Fatalf("expected valid YAML to pass, got: %v", err)
	}
}

func TestValidateSchemaInvalid(t *testing.T) {
	tmp := t.TempDir()

	// Missing required field: display_name
	yamlContent := `models:
  bad-model:
    billing_name: "bad"
    upstream_model: "test"
    risk_level: low
    allowed_groups: ["all"]
    channel: "test"
    route_policy: "test"
`

	schemaDir := filepath.Join(tmp, "schemas")
	os.MkdirAll(schemaDir, 0755)

	realSchema, err := os.ReadFile(filepath.Join(findRepoRoot(t), "control-plane", "schemas", "models.schema.json"))
	if err != nil {
		t.Fatalf("read schema: %v", err)
	}
	schemaPath := writeFile(t, schemaDir, "models.schema.json", string(realSchema))

	err = ValidateYAML([]byte(yamlContent), schemaPath)
	if err == nil {
		t.Fatal("expected error for missing display_name, got nil")
	}

	// Error should contain field path information
	errStr := err.Error()
	if !strings.Contains(errStr, "display_name") {
		t.Errorf("expected error to mention 'display_name', got: %s", errStr)
	}
}

func TestValidateSchemaInvalidType(t *testing.T) {
	tmp := t.TempDir()

	// Wrong type: type should be integer, not string
	yamlContent := `channels:
  bad-channel:
    name: "Bad Channel"
    type: "not-an-integer"
    models: "test"
    group: "default"
    status: 1
`

	schemaDir := filepath.Join(tmp, "schemas")
	os.MkdirAll(schemaDir, 0755)

	realSchema, err := os.ReadFile(filepath.Join(findRepoRoot(t), "control-plane", "schemas", "channels.schema.json"))
	if err != nil {
		t.Fatalf("read schema: %v", err)
	}
	schemaPath := writeFile(t, schemaDir, "channels.schema.json", string(realSchema))

	err = ValidateYAML([]byte(yamlContent), schemaPath)
	if err == nil {
		t.Fatal("expected error for wrong type, got nil")
	}
}

func TestValidateRegistryValid(t *testing.T) {
	repoRoot := findRepoRoot(t)
	policiesDir := filepath.Join(repoRoot, "control-plane", "policies")
	schemasDir := filepath.Join(repoRoot, "control-plane", "schemas")

	err := ValidateRegistry(policiesDir, schemasDir)
	if err != nil {
		t.Fatalf("expected valid registry to pass, got: %v", err)
	}
}

func TestValidateRegistryMissing(t *testing.T) {
	err := ValidateRegistry("/nonexistent/path/to/nowhere", "/also/nonexistent")
	if err == nil {
		t.Fatal("expected error for missing dirs, got nil")
	}
	errStr := err.Error()
	if !strings.Contains(errStr, "no such file") && !strings.Contains(errStr, "not exist") && !strings.Contains(errStr, "does not exist") {
		t.Errorf("expected error about missing directory, got: %s", errStr)
	}
}

func TestDuplicateIDCrossFile(t *testing.T) {
	tmp := t.TempDir()

	policiesDir := filepath.Join(tmp, "policies")
	schemasDir := filepath.Join(tmp, "schemas")
	os.MkdirAll(policiesDir, 0755)
	os.MkdirAll(schemasDir, 0755)

	// Copy real schemas
	repoRoot := findRepoRoot(t)
	srcSchemas := filepath.Join(repoRoot, "control-plane", "schemas")
	for _, name := range []string{"providers.schema.json"} {
		data, err := os.ReadFile(filepath.Join(srcSchemas, name))
		if err != nil {
			t.Fatalf("read schema %s: %v", name, err)
		}
		writeFile(t, schemasDir, name, string(data))
	}

	// Providers with duplicate key IDs across different providers
	writeFile(t, policiesDir, "providers.yaml", `providers:
  provider-a:
    keys:
      - id: duplicate-key
        name: "Key A"
        models: ["test"]
        weight: 1.0
  provider-b:
    keys:
      - id: duplicate-key
        name: "Key B"
        models: ["test"]
        weight: 1.0
`)

	err := ValidateRegistry(policiesDir, schemasDir)
	if err == nil {
		t.Fatal("expected error for duplicate key IDs, got nil")
	}
	errStr := err.Error()
	if !strings.Contains(errStr, "duplicate") {
		t.Errorf("expected error to mention 'duplicate', got: %s", errStr)
	}
}

// findRepoRoot walks up from the current working directory to find the starapihub repo root.
func findRepoRoot(t *testing.T) string {
	t.Helper()
	// We're in control-plane/dashboard/internal/registry/ during tests
	// The repo root is 4 levels up, but let's just search for control-plane/schemas
	dir, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, "control-plane", "schemas")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			t.Fatal("could not find repo root (looking for control-plane/schemas)")
		}
		dir = parent
	}
}
