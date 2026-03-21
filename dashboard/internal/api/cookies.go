package api

import (
	"encoding/json"
	"net/http"
)

func (h *Handler) HandleCookies(w http.ResponseWriter, r *http.Request) {
	cookies := h.state.GetCookies()

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(cookies)
}
