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

	// Step 1: Create providers in Bifrost
	var createdProviderIDs []string
	for _, p := range model.Providers {
		providerPayload, err := json.Marshal(map[string]interface{}{
			"id":         p.ProviderID,
			"model_name": p.ModelName,
			"weight":     p.Weight,
			"priority":   p.Priority,
		})
		if err != nil {
			slog.Error("marshal bifrost provider payload", "error", err)
			respondError(w, http.StatusInternalServerError, "internal error")
			return
		}
		if _, err := h.bifrost.CreateProvider(providerPayload); err != nil {
			slog.Error("create bifrost provider", "error", err, "provider_id", p.ProviderID)
			// Compensate: delete already-created providers
			h.rollbackBifrostProviders(createdProviderIDs)
			respondError(w, http.StatusBadGateway, "failed to create bifrost provider: "+err.Error())
			return
		}
		createdProviderIDs = append(createdProviderIDs, p.ProviderID)
	}

	// Step 2: Create channel in New-API
	channelPayload, err := json.Marshal(map[string]interface{}{
		"name":     model.Channel,
		"models":   model.Name,
		"type":     1, // OpenAI compatible
		"key":      "bifrost-internal",
		"base_url": h.bifrost.BaseURL(),
	})
	if err != nil {
		slog.Error("marshal newapi channel payload", "error", err)
		h.rollbackBifrostProviders(createdProviderIDs)
		respondError(w, http.StatusInternalServerError, "internal error")
		return
	}
	if _, err := h.newAPI.CreateChannel(h.dashboardToken, channelPayload); err != nil {
		slog.Error("create newapi channel", "error", err)
		// Compensate: roll back Bifrost providers
		h.rollbackBifrostProviders(createdProviderIDs)
		respondError(w, http.StatusBadGateway, "failed to create new-api channel: "+err.Error())
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

	// Re-sync providers to Bifrost (upsert semantics)
	for _, p := range model.Providers {
		providerPayload, err := json.Marshal(map[string]interface{}{
			"id":         p.ProviderID,
			"model_name": p.ModelName,
			"weight":     p.Weight,
			"priority":   p.Priority,
		})
		if err != nil {
			slog.Error("marshal bifrost provider payload", "error", err)
			continue
		}
		if _, err := h.bifrost.CreateProvider(providerPayload); err != nil {
			slog.Error("update bifrost provider", "error", err, "provider_id", p.ProviderID)
		}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"status": "updated", "name": model.Name})
}

func (h *Handler) HandleDeleteModel(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")

	// Delete from Bifrost
	if err := h.bifrost.DeleteProvider(id); err != nil {
		slog.Error("delete bifrost provider", "error", err, "id", id)
		// Continue to attempt New-API deletion even if Bifrost fails
	}

	// Delete from New-API
	if err := h.newAPI.DeleteChannel(h.dashboardToken, id); err != nil {
		slog.Error("delete newapi channel", "error", err, "id", id)
		respondError(w, http.StatusBadGateway, "failed to delete from upstreams: "+err.Error())
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"status": "deleted", "name": id})
}

// rollbackBifrostProviders attempts to delete providers that were already created.
func (h *Handler) rollbackBifrostProviders(ids []string) {
	for _, id := range ids {
		if err := h.bifrost.DeleteProvider(id); err != nil {
			slog.Error("compensating delete of bifrost provider failed", "error", err, "provider_id", id)
		}
	}
}

func respondError(w http.ResponseWriter, status int, msg string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(map[string]string{"error": msg})
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
