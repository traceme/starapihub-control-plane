package upstream

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
)

type ClewdRClient struct {
	client *http.Client
}

type CookieResponse struct {
	Valid     []json.RawMessage `json:"valid"`
	Exhausted []json.RawMessage `json:"exhausted"`
	Invalid   []json.RawMessage `json:"invalid"`
}

func NewClewdRClient(client *http.Client) *ClewdRClient {
	return &ClewdRClient{client: client}
}

// CheckHealth checks if a ClewdR instance is reachable.
func (c *ClewdRClient) CheckHealth(url string) (bool, error) {
	resp, err := c.client.Get(url + "/")
	if err != nil {
		return false, err
	}
	defer resp.Body.Close()
	io.Copy(io.Discard, resp.Body)
	return resp.StatusCode == http.StatusOK || resp.StatusCode == http.StatusFound || resp.StatusCode == http.StatusMovedPermanently, nil
}

// GetCookies fetches cookie status from a ClewdR instance.
func (c *ClewdRClient) GetCookies(url, adminToken string) (*CookieResponse, error) {
	req, err := http.NewRequest("GET", url+"/api/cookies", nil)
	if err != nil {
		return nil, err
	}
	if adminToken != "" {
		req.Header.Set("Authorization", "Bearer "+adminToken)
	}
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
		return nil, fmt.Errorf("get cookies from %s: status %d: %s", url, resp.StatusCode, string(body))
	}
	var cr CookieResponse
	if err := json.Unmarshal(body, &cr); err != nil {
		return nil, fmt.Errorf("parse cookies from %s: %w", url, err)
	}
	return &cr, nil
}

// --- Typed methods for sync engine ---

// GetCookiesTyped fetches cookie status from a ClewdR instance, returning typed structs.
func (c *ClewdRClient) GetCookiesTyped(url, adminToken string) (*CookieResponseTyped, error) {
	req, err := http.NewRequest("GET", url+"/api/cookies", nil)
	if err != nil {
		return nil, err
	}
	if adminToken != "" {
		req.Header.Set("Authorization", "Bearer "+adminToken)
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
		return nil, fmt.Errorf("get cookies typed from %s: status %d: %s", url, resp.StatusCode, string(body))
	}

	var cr CookieResponseTyped
	if err := json.Unmarshal(body, &cr); err != nil {
		return nil, fmt.Errorf("parse cookies typed from %s: %w", url, err)
	}
	return &cr, nil
}

// PostCookie adds a cookie to a ClewdR instance via POST /api/cookie.
func (c *ClewdRClient) PostCookie(url, adminToken string, cookie string) error {
	payload, err := json.Marshal(map[string]string{"cookie": cookie})
	if err != nil {
		return err
	}
	req, err := http.NewRequest("POST", url+"/api/cookie", bytes.NewReader(payload))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	if adminToken != "" {
		req.Header.Set("Authorization", "Bearer "+adminToken)
	}

	resp, err := doWithRetry(c.client, req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	io.Copy(io.Discard, resp.Body)
	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		return fmt.Errorf("post cookie to %s: status %d", url, resp.StatusCode)
	}
	return nil
}
