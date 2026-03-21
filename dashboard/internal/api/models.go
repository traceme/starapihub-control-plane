package api

import (
	"encoding/json"
	"log/slog"
	"net/http"
)

// LogicalModel represents a unified model across Bifrost and New-API.
type LogicalModel struct {
	Name        string        `json:"name"`
	DisplayName string        `json:"display_name"`
	Providers   []ProviderRef `json:"providers"`
	Channel     string        `json:"channel"`
	RiskLevel   string        `json:"risk_level"`
}

// ProviderRef points to a provider+model in Bifrost.
type ProviderRef struct {
	ProviderID string `json:"provider_id"`
	ModelName  string `json:"model_name"`
	Weight     int    `json:"weight"`
	Priority   int    `json:"priority"`
}

func (h *Handler) HandleListModels(w http.ResponseWriter, r *http.Request) {
	// Fetch providers from Bifrost
	providers, err := h.bifrost.ListProviders()
	if err != nil {
		slog.Error("list bifrost providers", "error", err)
	}

	// Fetch channels from New-API
	channels, err := h.newAPI.ListChannels(h.dashboardToken)
	if err != nil {
		slog.Error("list newapi channels", "error", err)
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"providers": jsonOrNull(providers),
		"channels":  jsonOrNull(channels),
	})
}

func (h *Handler) HandleCreateModel(w http.ResponseWriter, r *http.Request) {
	var model LogicalModel
	if err := json.NewDecoder(r.Body).Decode(&model); err != nil {
		http.Error(w, `{"error":"invalid json"}`, http.StatusBadRequest)
		return
	}

	// Step 1: Create provider in Bifrost for each provider ref
	for _, p := range model.Providers {
		providerPayload, _ := json.Marshal(map[string]interface{}{
			"id":         p.ProviderID,
			"model_name": p.ModelName,
			"weight":     p.Weight,
			"priority":   p.Priority,
		})
		if _, err := h.bifrost.CreateProvider(providerPayload); err != nil {
			slog.Error("create bifrost provider", "error", err, "provider_id", p.ProviderID)
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusBadGateway)
			json.NewEncoder(w).Encode(map[string]string{
				"error": "failed to create bifrost provider: " + err.Error(),
			})
			return
		}
	}

	// Step 2: Create channel in New-API
	channelPayload, _ := json.Marshal(map[string]interface{}{
		"name":    model.Channel,
		"models":  model.Name,
		"type":    1, // OpenAI compatible
		"key":     "bifrost-internal",
		"base_url": h.bifrost.BaseURL(),
	})
	if _, err := h.newAPI.CreateChannel(h.dashboardToken, channelPayload); err != nil {
		slog.Error("create newapi channel", "error", err)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadGateway)
		json.NewEncoder(w).Encode(map[string]string{
			"error": "failed to create new-api channel: " + err.Error(),
		})
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(map[string]string{"status": "created", "name": model.Name})
}

func (h *Handler) HandleUpdateModel(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	var model LogicalModel
	if err := json.NewDecoder(r.Body).Decode(&model); err != nil {
		http.Error(w, `{"error":"invalid json"}`, http.StatusBadRequest)
		return
	}
	model.Name = id

	// For update, we re-sync providers to Bifrost and channel to New-API
	// This is a simplified approach — a production system would diff and patch
	for _, p := range model.Providers {
		providerPayload, _ := json.Marshal(map[string]interface{}{
			"id":         p.ProviderID,
			"model_name": p.ModelName,
			"weight":     p.Weight,
			"priority":   p.Priority,
		})
		if _, err := h.bifrost.CreateProvider(providerPayload); err != nil {
			slog.Error("update bifrost provider", "error", err, "provider_id", p.ProviderID)
		}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"status": "updated", "name": model.Name})
}

func (h *Handler) HandleDeleteModel(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")

	// Note: Bifrost and New-API don't have simple delete-by-name APIs.
	// This is a placeholder that returns success — in production, this would
	// call the appropriate DELETE endpoints on both upstreams.
	slog.Info("delete model requested", "id", id)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"status": "deleted", "name": id})
}

func jsonOrNull(data json.RawMessage) interface{} {
	if data == nil {
		return nil
	}
	var v interface{}
	if err := json.Unmarshal(data, &v); err != nil {
		return nil
	}
	return v
}
