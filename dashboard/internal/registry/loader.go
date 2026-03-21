package registry

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"go.yaml.in/yaml/v3"
)

// checkDuplicateKeys decodes YAML as a yaml.Node tree and checks for duplicate
// map keys at the second level (under the top-level resource key like "providers",
// "channels", etc.). go.yaml.in/yaml/v3 silently overwrites duplicate map keys,
// so we must use Node decoding to detect them.
func checkDuplicateKeys(data []byte) error {
	var doc yaml.Node
	if err := yaml.Unmarshal(data, &doc); err != nil {
		return err
	}
	// doc is a Document node; its first child is the top-level mapping.
	if doc.Kind != yaml.DocumentNode || len(doc.Content) == 0 {
		return nil
	}
	topMap := doc.Content[0]
	if topMap.Kind != yaml.MappingNode {
		return nil
	}
	// Iterate over top-level map entries (key, value pairs).
	for i := 1; i < len(topMap.Content); i += 2 {
		valueNode := topMap.Content[i]
		if valueNode.Kind != yaml.MappingNode {
			continue
		}
		// Check for duplicate keys in this inner mapping.
		seen := make(map[string]int) // key -> line number
		for j := 0; j < len(valueNode.Content); j += 2 {
			keyNode := valueNode.Content[j]
			if keyNode.Kind != yaml.ScalarNode {
				continue
			}
			if prevLine, exists := seen[keyNode.Value]; exists {
				return fmt.Errorf("duplicate key %q (lines %d and %d)", keyNode.Value, prevLine, keyNode.Line)
			}
			seen[keyNode.Value] = keyNode.Line
		}
	}
	return nil
}

// loadAndParse reads a YAML file, checks for duplicate keys, and unmarshals
// into the provided target.
func loadAndParse(path string, target interface{}) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("read %s: %w", path, err)
	}
	if err := checkDuplicateKeys(data); err != nil {
		return fmt.Errorf("parse %s: %w", path, err)
	}
	// Use plain Unmarshal (not KnownFields) to tolerate extra fields in example
	// YAML files that have control-plane-only metadata not in our structs.
	if err := yaml.Unmarshal(data, target); err != nil {
		return fmt.Errorf("parse %s: %w", path, err)
	}
	return nil
}

// LoadModels loads and parses a models YAML file.
func LoadModels(path string) (*ModelsFile, error) {
	var f ModelsFile
	if err := loadAndParse(path, &f); err != nil {
		return nil, err
	}
	return &f, nil
}

// LoadRoutePolicies loads and parses a route-policies YAML file.
func LoadRoutePolicies(path string) (*RoutePoliciesFile, error) {
	var f RoutePoliciesFile
	if err := loadAndParse(path, &f); err != nil {
		return nil, err
	}
	return &f, nil
}

// LoadProviderPools loads and parses a provider-pools YAML file.
func LoadProviderPools(path string) (*ProviderPoolsFile, error) {
	var f ProviderPoolsFile
	if err := loadAndParse(path, &f); err != nil {
		return nil, err
	}
	return &f, nil
}

// LoadChannels loads and parses a channels YAML file.
func LoadChannels(path string) (*ChannelsFile, error) {
	var f ChannelsFile
	if err := loadAndParse(path, &f); err != nil {
		return nil, err
	}
	return &f, nil
}

// LoadProviders loads and parses a providers YAML file.
func LoadProviders(path string) (*ProvidersFile, error) {
	var f ProvidersFile
	if err := loadAndParse(path, &f); err != nil {
		return nil, err
	}
	return &f, nil
}

// LoadRoutingRules loads and parses a routing-rules YAML file.
func LoadRoutingRules(path string) (*RoutingRulesFile, error) {
	var f RoutingRulesFile
	if err := loadAndParse(path, &f); err != nil {
		return nil, err
	}
	return &f, nil
}

// LoadPricing loads and parses a pricing YAML file.
func LoadPricing(path string) (*PricingFile, error) {
	var f PricingFile
	if err := loadAndParse(path, &f); err != nil {
		return nil, err
	}
	return &f, nil
}

// Registry holds all loaded desired-state files.
type Registry struct {
	Models        *ModelsFile
	Channels      *ChannelsFile
	Providers     *ProvidersFile
	RoutePolicies *RoutePoliciesFile
	ProviderPools *ProviderPoolsFile
	RoutingRules  *RoutingRulesFile
	Pricing       *PricingFile
}

// LoadAll loads all YAML files from a directory and checks for duplicate IDs.
// Files are optional -- missing files are skipped (nil in Registry).
// Supported file names:
//   - models.yaml (fallback: logical-models.example.yaml)
//   - channels.yaml
//   - providers.yaml
//   - route-policies.yaml (fallback: route-policies.example.yaml)
//   - provider-pools.yaml (fallback: provider-pools.example.yaml)
//   - routing-rules.yaml
//   - pricing.yaml
func LoadAll(dir string) (*Registry, error) {
	r := &Registry{}
	var errs []string

	// Helper to try primary name, then fallback.
	tryLoad := func(primary, fallback string, loader func(string) error) {
		path := filepath.Join(dir, primary)
		if _, err := os.Stat(path); err != nil {
			if fallback != "" {
				path = filepath.Join(dir, fallback)
				if _, err := os.Stat(path); err != nil {
					return // both missing, skip
				}
			} else {
				return // missing, skip
			}
		}
		if err := loader(path); err != nil {
			errs = append(errs, err.Error())
		}
	}

	tryLoad("models.yaml", "logical-models.example.yaml", func(p string) error {
		f, err := LoadModels(p)
		r.Models = f
		return err
	})
	tryLoad("channels.yaml", "", func(p string) error {
		f, err := LoadChannels(p)
		r.Channels = f
		return err
	})
	tryLoad("providers.yaml", "", func(p string) error {
		f, err := LoadProviders(p)
		r.Providers = f
		return err
	})
	tryLoad("route-policies.yaml", "route-policies.example.yaml", func(p string) error {
		f, err := LoadRoutePolicies(p)
		r.RoutePolicies = f
		return err
	})
	tryLoad("provider-pools.yaml", "provider-pools.example.yaml", func(p string) error {
		f, err := LoadProviderPools(p)
		r.ProviderPools = f
		return err
	})
	tryLoad("routing-rules.yaml", "", func(p string) error {
		f, err := LoadRoutingRules(p)
		r.RoutingRules = f
		return err
	})
	tryLoad("pricing.yaml", "", func(p string) error {
		f, err := LoadPricing(p)
		r.Pricing = f
		return err
	})

	if len(errs) > 0 {
		return nil, fmt.Errorf("load errors:\n%s", strings.Join(errs, "\n"))
	}
	return r, nil
}
