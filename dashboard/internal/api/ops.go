package api

import (
	"bufio"
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"strings"
)

// HandleSyncDryRun returns the output of `starapihub sync --dry-run --output json`.
// The dashboard runs the CLI in-process via the sync package rather than shelling out.
// For v1, we expose the last sync report from the audit log.
func (h *Handler) HandleSyncStatus(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	auditPath := resolveAuditLogPath()
	entries := readLastNAuditEntries(auditPath, 1, "sync")
	if len(entries) == 0 {
		json.NewEncoder(w).Encode(map[string]any{
			"status":  "no_data",
			"message": "No sync operations recorded yet. Run: starapihub sync",
		})
		return
	}
	json.NewEncoder(w).Encode(entries[0])
}

// HandleDiffStatus returns the last drift detection result from the audit log.
func (h *Handler) HandleDiffStatus(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	auditPath := resolveAuditLogPath()
	entries := readLastNAuditEntries(auditPath, 1, "sync")
	if len(entries) == 0 {
		json.NewEncoder(w).Encode(map[string]any{
			"status":  "no_data",
			"message": "No drift checks recorded. Run: starapihub diff",
		})
		return
	}
	json.NewEncoder(w).Encode(entries[0])
}

// HandleAuditLog returns the last N audit log entries.
func (h *Handler) HandleAuditLog(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	limitStr := r.URL.Query().Get("limit")
	limit := 20
	if limitStr != "" {
		if n, err := parseInt(limitStr); err == nil && n > 0 && n <= 100 {
			limit = n
		}
	}
	opFilter := r.URL.Query().Get("operation") // "sync", "bootstrap", or ""

	auditPath := resolveAuditLogPath()
	entries := readLastNAuditEntries(auditPath, limit, opFilter)
	json.NewEncoder(w).Encode(map[string]any{
		"entries": entries,
		"total":   len(entries),
	})
}

// HandleBootstrapStatus returns the last bootstrap audit entry.
func (h *Handler) HandleBootstrapStatus(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	auditPath := resolveAuditLogPath()
	entries := readLastNAuditEntries(auditPath, 1, "bootstrap")
	if len(entries) == 0 {
		json.NewEncoder(w).Encode(map[string]any{
			"status":  "no_data",
			"message": "No bootstrap operations recorded. Run: starapihub bootstrap",
		})
		return
	}
	json.NewEncoder(w).Encode(entries[0])
}

// --- helpers ---

func resolveAuditLogPath() string {
	path := os.Getenv("STARAPIHUB_AUDIT_LOG")
	if path != "" {
		return path
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	return filepath.Join(home, ".starapihub", "audit.log")
}

// readLastNAuditEntries reads the JSONL audit log and returns up to the last N entries,
// optionally filtered by operation type.
func readLastNAuditEntries(path string, n int, opFilter string) []json.RawMessage {
	if path == "" {
		return nil
	}
	f, err := os.Open(path)
	if err != nil {
		return nil
	}
	defer f.Close()

	var all []json.RawMessage
	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 1024*1024), 1024*1024) // 1MB line buffer
	for scanner.Scan() {
		line := scanner.Bytes()
		if len(line) == 0 {
			continue
		}
		if opFilter != "" {
			// Quick check without full parse
			if !strings.Contains(string(line), `"operation":"`+opFilter+`"`) {
				continue
			}
		}
		cp := make([]byte, len(line))
		copy(cp, line)
		all = append(all, json.RawMessage(cp))
	}

	// Return last N
	if len(all) <= n {
		return all
	}
	return all[len(all)-n:]
}

func parseInt(s string) (int, error) {
	var n int
	err := json.Unmarshal([]byte(s), &n)
	return n, err
}
