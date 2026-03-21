package api

import (
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
	"time"
)

func (h *Handler) HandleListLogs(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()

	// Parse status: supports exact code (200) or range strings (2xx, 4xx, 5xx)
	statusParam := q.Get("status")
	statusMin, statusMax := parseStatusRange(statusParam)
	model := q.Get("model")
	limit, _ := strconv.Atoi(q.Get("limit"))
	if limit <= 0 {
		limit = 200
	}

	var since, until time.Time
	if s := q.Get("since"); s != "" {
		since, _ = time.Parse(time.RFC3339, s)
	}
	if u := q.Get("until"); u != "" {
		until, _ = time.Parse(time.RFC3339, u)
	}

	entries, err := h.store.QueryLogs(statusMin, statusMax, model, since, until, limit)
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"entries": entries,
		"count":   len(entries),
	})
}

func (h *Handler) HandleGetLog(w http.ResponseWriter, r *http.Request) {
	requestID := r.PathValue("requestId")
	if requestID == "" {
		http.Error(w, `{"error":"missing request id"}`, http.StatusBadRequest)
		return
	}

	entries, err := h.store.GetLogByRequestID(requestID)
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
		return
	}

	if len(entries) == 0 {
		http.Error(w, `{"error":"not found"}`, http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"request_id": requestID,
		"entries":    entries,
	})
}

// parseStatusRange parses a status parameter into min/max range.
// "2xx" → (200, 299), "4xx" → (400, 499), "200" → (200, 200), "" → (0, 0)
func parseStatusRange(s string) (int, int) {
	if s == "" {
		return 0, 0
	}
	s = strings.TrimSpace(s)
	if strings.HasSuffix(strings.ToLower(s), "xx") {
		prefix := s[:len(s)-2]
		base, err := strconv.Atoi(prefix)
		if err != nil {
			return 0, 0
		}
		return base * 100, base*100 + 99
	}
	code, err := strconv.Atoi(s)
	if err != nil {
		return 0, 0
	}
	return code, code
}
