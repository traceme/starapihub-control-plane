package upstream

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
)

type NewAPIClient struct {
	client  *http.Client
	baseURL string
}

func NewNewAPIClient(client *http.Client, baseURL string) *NewAPIClient {
	return &NewAPIClient{client: client, baseURL: baseURL}
}

func (c *NewAPIClient) BaseURL() string {
	return c.baseURL
}

// CheckHealth checks if New-API is healthy via GET /api/status.
func (c *NewAPIClient) CheckHealth() (bool, error) {
	resp, err := c.client.Get(c.baseURL + "/api/status")
	if err != nil {
		return false, err
	}
	defer resp.Body.Close()
	io.Copy(io.Discard, resp.Body)
	return resp.StatusCode == http.StatusOK, nil
}

// ListModels lists available models via GET /v1/models.
func (c *NewAPIClient) ListModels(apiKey string) (json.RawMessage, error) {
	req, err := http.NewRequest("GET", c.baseURL+"/api/v1/models", nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+apiKey)
	resp, err := c.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("list models: status %d: %s", resp.StatusCode, string(body))
	}
	return json.RawMessage(body), nil
}

// ListChannels lists channels via GET /api/channel/.
func (c *NewAPIClient) ListChannels(adminToken string) (json.RawMessage, error) {
	req, err := http.NewRequest("GET", c.baseURL+"/api/channel/?p=0&page_size=100", nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+adminToken)
	resp, err := c.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("list channels: status %d: %s", resp.StatusCode, string(body))
	}
	return json.RawMessage(body), nil
}

// CreateChannel creates a channel via POST /api/channel/.
func (c *NewAPIClient) CreateChannel(adminToken string, channel json.RawMessage) (json.RawMessage, error) {
	req, err := http.NewRequest("POST", c.baseURL+"/api/channel/", bytes.NewReader(channel))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+adminToken)
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
		return nil, fmt.Errorf("create channel: status %d: %s", resp.StatusCode, string(body))
	}
	return json.RawMessage(body), nil
}

// DeleteChannel deletes a channel via DELETE /api/channel/{id}.
func (c *NewAPIClient) DeleteChannel(adminToken string, id string) error {
	req, err := http.NewRequest("DELETE", c.baseURL+"/api/channel/"+id, nil)
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+adminToken)
	resp, err := c.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	io.Copy(io.Discard, resp.Body)
	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusNoContent {
		return fmt.Errorf("delete channel %s: status %d", id, resp.StatusCode)
	}
	return nil
}

// SendChatCompletion sends a test chat completion request via POST /v1/chat/completions.
func (c *NewAPIClient) SendChatCompletion(apiKey string, payload json.RawMessage) (json.RawMessage, error) {
	req, err := http.NewRequest("POST", c.baseURL+"/v1/chat/completions", bytes.NewReader(payload))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+apiKey)
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
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("chat completion: status %d: %s", resp.StatusCode, string(body))
	}
	return json.RawMessage(body), nil
}

// GetOptions gets system options via GET /api/option/.
func (c *NewAPIClient) GetOptions(adminToken string) (json.RawMessage, error) {
	req, err := http.NewRequest("GET", c.baseURL+"/api/option/", nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+adminToken)
	resp, err := c.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("get options: status %d: %s", resp.StatusCode, string(body))
	}
	return json.RawMessage(body), nil
}
