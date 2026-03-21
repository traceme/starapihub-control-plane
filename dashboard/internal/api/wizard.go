package api

import (
	"encoding/json"
	"log/slog"
	"net/http"
)

// WizardProviderRequest is the request body for step 1: add API key to Bifrost.
type WizardProviderRequest struct {
	ProviderID string `json:"provider_id"`
	APIKey     string `json:"api_key"`
	BaseURL    string `json:"base_url"`
}

// WizardModelRequest is the request body for step 2: create model.
type WizardModelRequest struct {
	ModelName   string `json:"model_name"`
	DisplayName string `json:"display_name"`
	ProviderID  string `json:"provider_id"`
	Upstream    string `json:"upstream_model"`
}

// WizardTestRequest is the request body for step 3: test request.
type WizardTestRequest struct {
	ModelName string `json:"model_name"`
	Prompt    string `json:"prompt"`
}

func (h *Handler) HandleWizardProvider(w http.ResponseWriter, r *http.Request) {
	var req WizardProviderRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, `{"error":"invalid json"}`, http.StatusBadRequest)
		return
	}

	if req.ProviderID == "" || req.APIKey == "" {
		http.Error(w, `{"error":"provider_id and api_key are required"}`, http.StatusBadRequest)
		return
	}

	payload, _ := json.Marshal(map[string]interface{}{
		"id":       req.ProviderID,
		"api_key":  req.APIKey,
		"base_url": req.BaseURL,
	})

	result, err := h.bifrost.CreateProvider(payload)
	if err != nil {
		slog.Error("wizard: create bifrost provider", "error", err)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadGateway)
		json.NewEncoder(w).Encode(map[string]string{
			"error": "failed to create provider in Bifrost: " + err.Error(),
		})
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	w.Write(result)
}

func (h *Handler) HandleWizardModel(w http.ResponseWriter, r *http.Request) {
	var req WizardModelRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, `{"error":"invalid json"}`, http.StatusBadRequest)
		return
	}

	if req.ModelName == "" || req.ProviderID == "" {
		http.Error(w, `{"error":"model_name and provider_id are required"}`, http.StatusBadRequest)
		return
	}

	// Create channel in New-API pointing to Bifrost
	channelPayload, _ := json.Marshal(map[string]interface{}{
		"name":     req.DisplayName,
		"models":   req.ModelName,
		"type":     1, // OpenAI compatible
		"key":      "bifrost-internal",
		"base_url": h.bifrost.BaseURL(),
	})

	result, err := h.newAPI.CreateChannel(h.dashboardToken, channelPayload)
	if err != nil {
		slog.Error("wizard: create newapi channel", "error", err)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadGateway)
		json.NewEncoder(w).Encode(map[string]string{
			"error": "failed to create channel in New-API: " + err.Error(),
		})
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	w.Write(result)
}

func (h *Handler) HandleWizardTest(w http.ResponseWriter, r *http.Request) {
	var req WizardTestRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, `{"error":"invalid json"}`, http.StatusBadRequest)
		return
	}

	if req.ModelName == "" {
		http.Error(w, `{"error":"model_name is required"}`, http.StatusBadRequest)
		return
	}

	if req.Prompt == "" {
		req.Prompt = "Say hello in one sentence."
	}

	// Send a test completion request through the New-API -> Bifrost chain
	testPayload, _ := json.Marshal(map[string]interface{}{
		"model": req.ModelName,
		"messages": []map[string]string{
			{"role": "user", "content": req.Prompt},
		},
		"max_tokens": 50,
	})

	result, err := h.newAPI.ListModels(h.dashboardToken)
	if err != nil {
		// If we can't even list models, the chain is broken
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadGateway)
		json.NewEncoder(w).Encode(map[string]string{
			"error": "cannot reach New-API: " + err.Error(),
		})
		return
	}

	// For now, return the test payload that would be sent and models available
	// A full implementation would POST to /v1/chat/completions
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"status":       "test_prepared",
		"model":        req.ModelName,
		"test_payload": json.RawMessage(testPayload),
		"models":       json.RawMessage(result),
	})
}

func (h *Handler) HandleWizardStatus(w http.ResponseWriter, r *http.Request) {
	snapshot := h.state.GetSnapshot()

	// Check each step
	bifrostHealthy := false
	newAPIHealthy := false
	if bh, ok := snapshot.Health["bifrost"]; ok {
		bifrostHealthy = bh.Status == "healthy"
	}
	if nh, ok := snapshot.Health["new-api"]; ok {
		newAPIHealthy = nh.Status == "healthy"
	}

	// Check if we have providers configured
	providers, _ := h.bifrost.ListProviders()
	hasProviders := providers != nil && len(providers) > 2 // more than just "[]"

	// Check if we have channels configured
	channels, _ := h.newAPI.ListChannels(h.dashboardToken)
	hasChannels := channels != nil && len(channels) > 2

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"bifrost_healthy": bifrostHealthy,
		"newapi_healthy":  newAPIHealthy,
		"has_providers":   hasProviders,
		"has_channels":    hasChannels,
		"steps": map[string]interface{}{
			"1_provider": hasProviders,
			"2_model":    hasChannels,
			"3_test":     false, // requires manual test
		},
	})
}
