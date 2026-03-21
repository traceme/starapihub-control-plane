package upstream

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
)

type BifrostClient struct {
	client  *http.Client
	baseURL string
}

func NewBifrostClient(client *http.Client, baseURL string) *BifrostClient {
	return &BifrostClient{client: client, baseURL: baseURL}
}

func (c *BifrostClient) BaseURL() string {
	return c.baseURL
}

// CheckHealth checks Bifrost health via GET /health.
func (c *BifrostClient) CheckHealth() (bool, error) {
	resp, err := c.client.Get(c.baseURL + "/health")
	if err != nil {
		return false, err
	}
	defer resp.Body.Close()
	io.Copy(io.Discard, resp.Body)
	return resp.StatusCode == http.StatusOK, nil
}

// GetConfig returns Bifrost config via GET /api/config.
func (c *BifrostClient) GetConfig() (json.RawMessage, error) {
	resp, err := c.client.Get(c.baseURL + "/api/config")
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("get config: status %d: %s", resp.StatusCode, string(body))
	}
	return json.RawMessage(body), nil
}

// UpdateConfig updates Bifrost config via PUT /api/config.
func (c *BifrostClient) UpdateConfig(config json.RawMessage) error {
	req, err := http.NewRequest("PUT", c.baseURL+"/api/config", bytes.NewReader(config))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := c.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	io.Copy(io.Discard, resp.Body)
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("update config: status %d", resp.StatusCode)
	}
	return nil
}

// ListProviders lists Bifrost providers via GET /api/providers.
func (c *BifrostClient) ListProviders() (json.RawMessage, error) {
	resp, err := c.client.Get(c.baseURL + "/api/providers")
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("list providers: status %d: %s", resp.StatusCode, string(body))
	}
	return json.RawMessage(body), nil
}

// CreateProvider creates a Bifrost provider via POST /api/providers.
func (c *BifrostClient) CreateProvider(provider json.RawMessage) (json.RawMessage, error) {
	req, err := http.NewRequest("POST", c.baseURL+"/api/providers", bytes.NewReader(provider))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := c.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		return nil, fmt.Errorf("create provider: status %d: %s", resp.StatusCode, string(body))
	}
	return json.RawMessage(body), nil
}

// DeleteProvider deletes a Bifrost provider via DELETE /api/providers/{id}.
func (c *BifrostClient) DeleteProvider(id string) error {
	req, err := http.NewRequest("DELETE", c.baseURL+"/api/providers/"+id, nil)
	if err != nil {
		return err
	}
	resp, err := c.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	io.Copy(io.Discard, resp.Body)
	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusNoContent {
		return fmt.Errorf("delete provider %s: status %d", id, resp.StatusCode)
	}
	return nil
}

// ListVirtualKeys lists Bifrost virtual keys via GET /api/governance/virtual-keys.
func (c *BifrostClient) ListVirtualKeys() (json.RawMessage, error) {
	resp, err := c.client.Get(c.baseURL + "/api/governance/virtual-keys")
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("list virtual keys: status %d: %s", resp.StatusCode, string(body))
	}
	return json.RawMessage(body), nil
}

// --- Typed methods for sync engine ---

// GetConfigTyped returns the Bifrost config as a typed struct.
func (c *BifrostClient) GetConfigTyped() (*BifrostConfigResponse, error) {
	req, err := http.NewRequest("GET", c.baseURL+"/api/config", nil)
	if err != nil {
		return nil, err
	}
	resp, err := doWithRetry(c.client, req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("get config typed: status %d: %s", resp.StatusCode, string(body))
	}

	var cfg BifrostConfigResponse
	if err := json.Unmarshal(body, &cfg); err != nil {
		return nil, fmt.Errorf("get config typed: parse: %w", err)
	}
	return &cfg, nil
}

// UpdateConfigTyped updates the Bifrost config via PUT /api/config.
func (c *BifrostClient) UpdateConfigTyped(config *BifrostConfigResponse) error {
	payload, err := json.Marshal(config)
	if err != nil {
		return err
	}
	req, err := http.NewRequest("PUT", c.baseURL+"/api/config", bytes.NewReader(payload))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := doWithRetry(c.client, req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	io.Copy(io.Discard, resp.Body)
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("update config typed: status %d", resp.StatusCode)
	}
	return nil
}

// ListProvidersTyped returns all Bifrost providers as a typed map.
func (c *BifrostClient) ListProvidersTyped() (map[string]BifrostProviderResponse, error) {
	req, err := http.NewRequest("GET", c.baseURL+"/api/providers", nil)
	if err != nil {
		return nil, err
	}
	resp, err := doWithRetry(c.client, req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("list providers typed: status %d: %s", resp.StatusCode, string(body))
	}

	var providers BifrostProvidersMapResponse
	if err := json.Unmarshal(body, &providers); err != nil {
		return nil, fmt.Errorf("list providers typed: parse: %w", err)
	}
	return providers, nil
}

// CreateProviderTyped creates a Bifrost provider via POST /api/providers.
func (c *BifrostClient) CreateProviderTyped(id string, provider json.RawMessage) error {
	// Bifrost expects the provider keyed by ID in the request body
	wrapped := map[string]json.RawMessage{id: provider}
	payload, err := json.Marshal(wrapped)
	if err != nil {
		return err
	}
	req, err := http.NewRequest("POST", c.baseURL+"/api/providers", bytes.NewReader(payload))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := doWithRetry(c.client, req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	io.Copy(io.Discard, resp.Body)
	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		return fmt.Errorf("create provider %s: status %d", id, resp.StatusCode)
	}
	return nil
}

// UpdateProviderTyped updates a Bifrost provider via PUT /api/providers/{id}.
func (c *BifrostClient) UpdateProviderTyped(id string, provider json.RawMessage) error {
	req, err := http.NewRequest("PUT", c.baseURL+"/api/providers/"+id, bytes.NewReader(provider))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := doWithRetry(c.client, req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	io.Copy(io.Discard, resp.Body)
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("update provider %s: status %d", id, resp.StatusCode)
	}
	return nil
}

// ListRoutingRulesTyped returns all Bifrost routing rules as typed structs.
func (c *BifrostClient) ListRoutingRulesTyped() ([]BifrostRoutingRuleResponse, error) {
	req, err := http.NewRequest("GET", c.baseURL+"/api/governance/routing-rules", nil)
	if err != nil {
		return nil, err
	}
	resp, err := doWithRetry(c.client, req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("list routing rules typed: status %d: %s", resp.StatusCode, string(body))
	}

	var rules []BifrostRoutingRuleResponse
	if err := json.Unmarshal(body, &rules); err != nil {
		return nil, fmt.Errorf("list routing rules typed: parse: %w", err)
	}
	return rules, nil
}

// CreateRoutingRuleTyped creates a Bifrost routing rule via POST /api/governance/routing-rules.
func (c *BifrostClient) CreateRoutingRuleTyped(rule json.RawMessage) (*BifrostRoutingRuleResponse, error) {
	req, err := http.NewRequest("POST", c.baseURL+"/api/governance/routing-rules", bytes.NewReader(rule))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := doWithRetry(c.client, req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		return nil, fmt.Errorf("create routing rule: status %d: %s", resp.StatusCode, string(body))
	}

	var created BifrostRoutingRuleResponse
	if err := json.Unmarshal(body, &created); err != nil {
		return nil, fmt.Errorf("create routing rule: parse: %w", err)
	}
	return &created, nil
}

// UpdateRoutingRuleTyped updates a Bifrost routing rule via PUT /api/governance/routing-rules/{id}.
func (c *BifrostClient) UpdateRoutingRuleTyped(id string, rule json.RawMessage) error {
	req, err := http.NewRequest("PUT", c.baseURL+"/api/governance/routing-rules/"+id, bytes.NewReader(rule))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := doWithRetry(c.client, req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	io.Copy(io.Discard, resp.Body)
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("update routing rule %s: status %d", id, resp.StatusCode)
	}
	return nil
}

// DeleteRoutingRuleTyped deletes a Bifrost routing rule via DELETE /api/governance/routing-rules/{id}.
func (c *BifrostClient) DeleteRoutingRuleTyped(id string) error {
	req, err := http.NewRequest("DELETE", c.baseURL+"/api/governance/routing-rules/"+id, nil)
	if err != nil {
		return err
	}

	resp, err := doWithRetry(c.client, req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	io.Copy(io.Discard, resp.Body)
	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusNoContent {
		return fmt.Errorf("delete routing rule %s: status %d", id, resp.StatusCode)
	}
	return nil
}
