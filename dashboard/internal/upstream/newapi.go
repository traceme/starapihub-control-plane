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

// SetupAdmin calls POST /api/setup to create the initial admin user.
// Returns the response body (contains token) on success.
// Returns error on network failure or unexpected status codes.
func (c *NewAPIClient) SetupAdmin(username, password string) (json.RawMessage, error) {
	payload, err := json.Marshal(map[string]string{"username": username, "password": password})
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequest("POST", c.baseURL+"/api/setup", bytes.NewReader(payload))
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
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("setup admin: status %d: %s", resp.StatusCode, string(body))
	}
	return json.RawMessage(body), nil
}

// --- Typed methods for sync engine ---

// ListChannelsTyped fetches all channels with pagination, returning typed structs.
func (c *NewAPIClient) ListChannelsTyped(adminToken string) ([]ChannelResponse, error) {
	var all []ChannelResponse
	pageSize := 100
	page := 0

	for {
		url := fmt.Sprintf("%s/api/channel/?p=%d&page_size=%d", c.baseURL, page, pageSize)
		req, err := http.NewRequest("GET", url, nil)
		if err != nil {
			return nil, err
		}
		req.Header.Set("Authorization", "Bearer "+adminToken)

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
			return nil, fmt.Errorf("list channels typed: status %d: %s", resp.StatusCode, string(body))
		}

		var clr ChannelListResponse
		if err := json.Unmarshal(body, &clr); err != nil {
			return nil, fmt.Errorf("list channels typed: parse: %w", err)
		}
		if !clr.Success {
			return nil, fmt.Errorf("list channels typed: API error: %s", clr.Message)
		}

		all = append(all, clr.Data.Items...)

		if len(all) >= clr.Data.Total {
			break
		}
		page++
	}

	return all, nil
}

// GetChannelTyped fetches a single channel by ID, returning a typed struct.
func (c *NewAPIClient) GetChannelTyped(adminToken string, id int) (*ChannelResponse, error) {
	url := fmt.Sprintf("%s/api/channel/%d", c.baseURL, id)
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+adminToken)

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
		return nil, fmt.Errorf("get channel %d: status %d: %s", id, resp.StatusCode, string(body))
	}

	var scr SingleChannelResponse
	if err := json.Unmarshal(body, &scr); err != nil {
		return nil, fmt.Errorf("get channel %d: parse: %w", id, err)
	}
	if !scr.Success {
		return nil, fmt.Errorf("get channel %d: API error: %s", id, scr.Message)
	}
	return &scr.Data, nil
}

// UpdateChannelTyped updates a channel via PUT /api/channel/.
func (c *NewAPIClient) UpdateChannelTyped(adminToken string, channel json.RawMessage) error {
	req, err := http.NewRequest("PUT", c.baseURL+"/api/channel/", bytes.NewReader(channel))
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+adminToken)
	req.Header.Set("Content-Type", "application/json")

	resp, err := doWithRetry(c.client, req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("update channel: status %d: %s", resp.StatusCode, string(body))
	}

	var result struct {
		Success bool   `json:"success"`
		Message string `json:"message"`
	}
	if err := json.Unmarshal(body, &result); err != nil {
		return fmt.Errorf("update channel: parse response: %w", err)
	}
	if !result.Success {
		return fmt.Errorf("update channel: API error: %s", result.Message)
	}
	return nil
}

// PutOption sets a system option via PUT /api/option/.
func (c *NewAPIClient) PutOption(adminToken string, key string, value string) error {
	payload, err := json.Marshal(map[string]string{"key": key, "value": value})
	if err != nil {
		return err
	}
	req, err := http.NewRequest("PUT", c.baseURL+"/api/option/", bytes.NewReader(payload))
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+adminToken)
	req.Header.Set("Content-Type", "application/json")

	resp, err := doWithRetry(c.client, req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("put option %s: status %d: %s", key, resp.StatusCode, string(body))
	}

	var result struct {
		Success bool   `json:"success"`
		Message string `json:"message"`
	}
	if err := json.Unmarshal(body, &result); err != nil {
		return fmt.Errorf("put option %s: parse response: %w", key, err)
	}
	if !result.Success {
		return fmt.Errorf("put option %s: API error: %s", key, result.Message)
	}
	return nil
}

// GetOptionsTyped fetches all system options, returning typed entries.
func (c *NewAPIClient) GetOptionsTyped(adminToken string) ([]OptionEntry, error) {
	req, err := http.NewRequest("GET", c.baseURL+"/api/option/", nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+adminToken)

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
		return nil, fmt.Errorf("get options typed: status %d: %s", resp.StatusCode, string(body))
	}

	var olr OptionListResponse
	if err := json.Unmarshal(body, &olr); err != nil {
		return nil, fmt.Errorf("get options typed: parse: %w", err)
	}
	if !olr.Success {
		return nil, fmt.Errorf("get options typed: API error: %s", olr.Message)
	}
	return olr.Data, nil
}
