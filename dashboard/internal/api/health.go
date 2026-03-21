package api

import (
	"encoding/json"
	"net/http"
)

func (h *Handler) HandleHealth(w http.ResponseWriter, r *http.Request) {
	snapshot := h.state.GetSnapshot()

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"services":   snapshot.Health,
		"log_stats":  snapshot.LogStats,
		"updated_at": snapshot.UpdatedAt,
	})
}
