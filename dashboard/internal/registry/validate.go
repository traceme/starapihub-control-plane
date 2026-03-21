package registry

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/santhosh-tekuri/jsonschema/v6"
	"go.yaml.in/yaml/v3"
)

// SchemaMapping maps YAML filename to schema filename.
var SchemaMapping = map[string]string{
	"models.yaml":        "models.schema.json",
	"channels.yaml":      "channels.schema.json",
	"providers.yaml":     "providers.schema.json",
	"routing-rules.yaml": "routing-rules.schema.json",
	"pricing.yaml":       "pricing.schema.json",
}

// ValidateYAML validates a YAML byte slice against a JSON Schema file.
// Returns nil if valid, or an error with field path details if invalid.
func ValidateYAML(yamlData []byte, schemaPath string) error {
	// 1. Parse YAML to generic interface{}
	var doc interface{}
	if err := yaml.Unmarshal(yamlData, &doc); err != nil {
		return fmt.Errorf("parse YAML: %w", err)
	}

	// 2. Convert YAML types to JSON-compatible types
	doc = convertYAMLToJSON(doc)

	// 3. Compile schema
	absPath, err := filepath.Abs(schemaPath)
	if err != nil {
		return fmt.Errorf("resolve schema path: %w", err)
	}

	compiler := jsonschema.NewCompiler()
	schema, err := compiler.Compile("file://" + absPath)
	if err != nil {
		return fmt.Errorf("compile schema %s: %w", schemaPath, err)
	}

	// 4. Validate
	err = schema.Validate(doc)
	if err != nil {
		return fmt.Errorf("schema validation: %w", err)
	}

	return nil
}

// ValidateRegistry validates all YAML files in policiesDir against schemas in schemasDir.
// Also checks for duplicate key IDs across providers.
// Returns aggregated errors (all violations, not just first).
func ValidateRegistry(policiesDir, schemasDir string) error {
	// Check directories exist
	if _, err := os.Stat(policiesDir); err != nil {
		return fmt.Errorf("policies directory does not exist: %w", err)
	}
	if _, err := os.Stat(schemasDir); err != nil {
		return fmt.Errorf("schemas directory does not exist: %w", err)
	}

	var errs []string

	for yamlFile, schemaFile := range SchemaMapping {
		yamlPath := filepath.Join(policiesDir, yamlFile)
		schemaPath := filepath.Join(schemasDir, schemaFile)

		// Skip if YAML file doesn't exist (optional files)
		if _, err := os.Stat(yamlPath); os.IsNotExist(err) {
			continue
		}

		data, err := os.ReadFile(yamlPath)
		if err != nil {
			errs = append(errs, fmt.Sprintf("%s: %v", yamlFile, err))
			continue
		}

		if err := ValidateYAML(data, schemaPath); err != nil {
			errs = append(errs, fmt.Sprintf("%s: %v", yamlFile, err))
		}
	}

	// Cross-file duplicate key ID checks for providers
	if dupErr := checkDuplicateProviderKeyIDs(policiesDir); dupErr != nil {
		errs = append(errs, dupErr.Error())
	}

	if len(errs) > 0 {
		return fmt.Errorf("validation errors:\n  %s", strings.Join(errs, "\n  "))
	}
	return nil
}

// checkDuplicateProviderKeyIDs loads providers.yaml and checks that all key IDs
// are unique across all providers.
func checkDuplicateProviderKeyIDs(policiesDir string) error {
	providersPath := filepath.Join(policiesDir, "providers.yaml")
	if _, err := os.Stat(providersPath); os.IsNotExist(err) {
		return nil
	}

	pf, err := LoadProviders(providersPath)
	if err != nil {
		return fmt.Errorf("load providers for duplicate check: %w", err)
	}

	seen := make(map[string]string) // key ID -> provider name
	for provName, prov := range pf.Providers {
		for _, key := range prov.Keys {
			if prevProv, exists := seen[key.ID]; exists {
				return fmt.Errorf("duplicate provider key ID %q found in providers %q and %q", key.ID, prevProv, provName)
			}
			seen[key.ID] = provName
		}
	}
	return nil
}

// convertYAMLToJSON converts YAML-specific types to JSON-compatible ones.
// go.yaml.in/yaml/v3 decodes maps with string keys, but integers need
// special handling for JSON Schema validation.
func convertYAMLToJSON(v interface{}) interface{} {
	switch val := v.(type) {
	case map[string]interface{}:
		result := make(map[string]interface{}, len(val))
		for k, v := range val {
			result[k] = convertYAMLToJSON(v)
		}
		return result
	case []interface{}:
		result := make([]interface{}, len(val))
		for i, v := range val {
			result[i] = convertYAMLToJSON(v)
		}
		return result
	case int:
		return float64(val)
	case int64:
		return float64(val)
	case int32:
		return float64(val)
	case float32:
		return float64(val)
	default:
		return v
	}
}
