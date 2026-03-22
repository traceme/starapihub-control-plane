package audit

import (
	"time"

	"github.com/starapihub/dashboard/internal/sync"
)

// DefaultLogPath is the default audit log location.
const DefaultLogPath = "~/.starapihub/audit.log"

// Entry is a single audit log record (one per sync/bootstrap operation).
type Entry struct {
	Timestamp     string   `json:"timestamp"`
	Operation     string   `json:"operation"`
	Targets       []string `json:"targets"`
	TotalActions  int      `json:"total_actions"`
	Succeeded     int      `json:"succeeded"`
	Failed        int      `json:"failed"`
	DriftWarnings int      `json:"drift_warnings"`
	Skipped       int      `json:"skipped"`
	DurationMs    int64    `json:"duration_ms"`
	Changes       []Change `json:"changes,omitempty"`
}

// Change records the before/after for a single resource modification.
type Change struct {
	ResourceType string `json:"resource_type"`
	ResourceID   string `json:"resource_id"`
	Action       string `json:"action"`
	Status       string `json:"status"`
	Desired      any    `json:"desired,omitempty"`
	Live         any    `json:"live,omitempty"`
	Error        string `json:"error,omitempty"`
}

// Logger writes audit entries to a JSONL file.
type Logger struct {
	path string
}

// NewLogger creates a Logger that writes to the given path.
func NewLogger(path string) *Logger {
	return nil // stub - will fail tests
}

// Write serializes a SyncReport as a single JSONL line and appends it to the log file.
func (l *Logger) Write(report *sync.SyncReport, operation string, targets []string, duration time.Duration) error {
	return nil // stub - will fail tests
}
