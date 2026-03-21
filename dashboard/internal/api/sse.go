package api

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"time"
)

func (h *Handler) HandleSSE(w http.ResponseWriter, r *http.Request) {
	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, `{"error":"streaming not supported"}`, http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")

	// Send initial snapshot immediately
	snapshot := h.state.GetSnapshot()
	data, _ := json.Marshal(snapshot)
	fmt.Fprintf(w, "data: %s\n\n", data)
	flusher.Flush()

	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-r.Context().Done():
			return
		case <-ticker.C:
			snapshot := h.state.GetSnapshot()
			data, err := json.Marshal(snapshot)
			if err != nil {
				slog.Error("SSE marshal snapshot", "error", err)
				fmt.Fprintf(w, "event: error\ndata: {\"error\":\"internal marshal failure\"}\n\n")
				flusher.Flush()
				continue
			}
			fmt.Fprintf(w, "data: %s\n\n", data)
			flusher.Flush()
		}
	}
}
