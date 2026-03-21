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
