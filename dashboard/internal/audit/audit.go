package audit

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/starapihub/dashboard/internal/sync"
)

// DefaultLogPath is the default audit log location.
const DefaultLogPath = "~/.starapihub/audit.log"

// BootstrapStep records a single bootstrap step's outcome in the audit log.
type BootstrapStep struct {
	Name    string `json:"name"`
	Status  string `json:"status"`
	Message string `json:"message,omitempty"`
}

// Entry is a single audit log record (one per sync/bootstrap operation).
type Entry struct {
	Timestamp      string          `json:"timestamp"`
	Operation      string          `json:"operation"`
	Targets        []string        `json:"targets"`
	TotalActions   int             `json:"total_actions"`
	Succeeded      int             `json:"succeeded"`
	Failed         int             `json:"failed"`
	DriftWarnings  int             `json:"drift_warnings"`
	Skipped        int             `json:"skipped"`
	DurationMs     int64           `json:"duration_ms"`
	Changes        []Change        `json:"changes,omitempty"`
	BootstrapSteps []BootstrapStep `json:"bootstrap_steps,omitempty"`
	BootstrapOK    *bool           `json:"bootstrap_ok,omitempty"`
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
// If path is empty, uses DefaultLogPath (with ~ expanded).
func NewLogger(path string) *Logger {
	if path == "" {
		path = expandHome(DefaultLogPath)
	}
	return &Logger{path: path}
}

// Write serializes a SyncReport as a single JSONL line and appends it to the log file.
// operation is "sync" or "bootstrap".
// targets is the list of reconciler names that were included.
// duration is how long the operation took.
func (l *Logger) Write(report *sync.SyncReport, operation string, targets []string, duration time.Duration) error {
	entry := Entry{
		Timestamp:  time.Now().UTC().Format(time.RFC3339),
		Operation:  operation,
		Targets:    targets,
		DurationMs: duration.Milliseconds(),
	}

	if report != nil {
		entry.TotalActions = report.TotalActions
		entry.Succeeded = report.Succeeded
		entry.Failed = report.Failed
		entry.DriftWarnings = report.DriftWarnings
		entry.Skipped = report.Skipped

		// Include before/after snapshots for changes only (not no-change)
		for _, r := range report.Results {
			if r.Action.Type == sync.ActionNoChange {
				continue
			}
			change := Change{
				ResourceType: r.Action.ResourceType,
				ResourceID:   r.Action.ResourceID,
				Action:       string(r.Action.Type),
				Status:       string(r.Status),
				Desired:      r.Action.Desired,
				Live:         r.Action.Live,
			}
			if r.Error != nil {
				change.Error = r.Error.Error()
			}
			entry.Changes = append(entry.Changes, change)
		}
	}

	// Serialize as single JSON line
	data, err := json.Marshal(entry)
	if err != nil {
		return fmt.Errorf("marshal audit entry: %w", err)
	}
	data = append(data, '\n')

	// Ensure parent directory exists
	dir := filepath.Dir(l.path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("create audit log dir: %w", err)
	}

	// Append to file
	f, err := os.OpenFile(l.path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return fmt.Errorf("open audit log: %w", err)
	}
	defer f.Close()

	_, err = f.Write(data)
	return err
}

// WriteBootstrap writes a bootstrap audit entry that includes step-level detail.
// syncReport may be nil if sync was skipped.
func (l *Logger) WriteBootstrap(syncReport *sync.SyncReport, steps []BootstrapStep, success bool, duration time.Duration) error {
	entry := Entry{
		Timestamp:      time.Now().UTC().Format(time.RFC3339),
		Operation:      "bootstrap",
		DurationMs:     duration.Milliseconds(),
		BootstrapSteps: steps,
		BootstrapOK:    &success,
	}

	if syncReport != nil {
		entry.TotalActions = syncReport.TotalActions
		entry.Succeeded = syncReport.Succeeded
		entry.Failed = syncReport.Failed
		entry.DriftWarnings = syncReport.DriftWarnings
		entry.Skipped = syncReport.Skipped

		for _, r := range syncReport.Results {
			if r.Action.Type == sync.ActionNoChange {
				continue
			}
			change := Change{
				ResourceType: r.Action.ResourceType,
				ResourceID:   r.Action.ResourceID,
				Action:       string(r.Action.Type),
				Status:       string(r.Status),
				Desired:      r.Action.Desired,
				Live:         r.Action.Live,
			}
			if r.Error != nil {
				change.Error = r.Error.Error()
			}
			entry.Changes = append(entry.Changes, change)
		}
	}

	data, err := json.Marshal(entry)
	if err != nil {
		return fmt.Errorf("marshal audit entry: %w", err)
	}
	data = append(data, '\n')

	dir := filepath.Dir(l.path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("create audit log dir: %w", err)
	}

	f, err := os.OpenFile(l.path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return fmt.Errorf("open audit log: %w", err)
	}
	defer f.Close()

	_, err = f.Write(data)
	return err
}

// expandHome replaces leading ~ with the user's home directory.
func expandHome(path string) string {
	if !strings.HasPrefix(path, "~") {
		return path
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return path
	}
	return filepath.Join(home, path[1:])
}
