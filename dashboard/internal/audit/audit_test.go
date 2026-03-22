package audit

import (
	"bufio"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/starapihub/dashboard/internal/sync"
)

// helper: read all JSONL lines from file
func readLines(t *testing.T, path string) []Entry {
	t.Helper()
	f, err := os.Open(path)
	if err != nil {
		t.Fatalf("open audit log: %v", err)
	}
	defer f.Close()

	var entries []Entry
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		var e Entry
		if err := json.Unmarshal(scanner.Bytes(), &e); err != nil {
			t.Fatalf("unmarshal line: %v", err)
		}
		entries = append(entries, e)
	}
	return entries
}

func TestWriteAppendsSingleLine(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "audit.log")
	logger := NewLogger(path)

	report := &sync.SyncReport{TotalActions: 1, Succeeded: 1}
	err := logger.Write(report, "sync", []string{"channel"}, 100*time.Millisecond)
	if err != nil {
		t.Fatalf("Write failed: %v", err)
	}

	entries := readLines(t, path)
	if len(entries) != 1 {
		t.Fatalf("expected 1 line, got %d", len(entries))
	}
}

func TestWriteTwoCallsProduceTwoLines(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "audit.log")
	logger := NewLogger(path)

	report := &sync.SyncReport{TotalActions: 1, Succeeded: 1}
	_ = logger.Write(report, "sync", []string{"channel"}, 50*time.Millisecond)
	_ = logger.Write(report, "sync", []string{"provider"}, 75*time.Millisecond)

	entries := readLines(t, path)
	if len(entries) != 2 {
		t.Fatalf("expected 2 lines, got %d", len(entries))
	}
}

func TestEntryTimestampRFC3339(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "audit.log")
	logger := NewLogger(path)

	report := &sync.SyncReport{}
	_ = logger.Write(report, "sync", nil, 0)

	entries := readLines(t, path)
	if len(entries) != 1 {
		t.Fatalf("expected 1 entry")
	}
	_, err := time.Parse(time.RFC3339, entries[0].Timestamp)
	if err != nil {
		t.Fatalf("timestamp not RFC3339: %q: %v", entries[0].Timestamp, err)
	}
}

func TestEntryFieldsPopulated(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "audit.log")
	logger := NewLogger(path)

	report := &sync.SyncReport{
		TotalActions:  5,
		Succeeded:     3,
		Failed:        1,
		DriftWarnings: 1,
		Skipped:       0,
	}
	targets := []string{"channel", "provider"}
	dur := 250 * time.Millisecond
	_ = logger.Write(report, "sync", targets, dur)

	entries := readLines(t, path)
	e := entries[0]
	if e.Operation != "sync" {
		t.Errorf("operation = %q, want sync", e.Operation)
	}
	if len(e.Targets) != 2 || e.Targets[0] != "channel" {
		t.Errorf("targets = %v, want [channel provider]", e.Targets)
	}
	if e.TotalActions != 5 {
		t.Errorf("total_actions = %d, want 5", e.TotalActions)
	}
	if e.Succeeded != 3 {
		t.Errorf("succeeded = %d, want 3", e.Succeeded)
	}
	if e.Failed != 1 {
		t.Errorf("failed = %d, want 1", e.Failed)
	}
	if e.DriftWarnings != 1 {
		t.Errorf("drift_warnings = %d, want 1", e.DriftWarnings)
	}
	if e.DurationMs != 250 {
		t.Errorf("duration_ms = %d, want 250", e.DurationMs)
	}
}

func TestChangesIncludeDesiredLiveSnapshots(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "audit.log")
	logger := NewLogger(path)

	report := &sync.SyncReport{
		TotalActions: 2,
		Succeeded:    2,
		Results: []sync.Result{
			{
				Action: sync.Action{
					Type:         sync.ActionCreate,
					ResourceType: "channel",
					ResourceID:   "openai-main",
					Desired:      map[string]string{"name": "openai-main"},
					Live:         nil,
				},
				Status: sync.StatusOK,
			},
			{
				Action: sync.Action{
					Type:         sync.ActionUpdate,
					ResourceType: "provider",
					ResourceID:   "bifrost-openai",
					Desired:      map[string]string{"model": "gpt-4"},
					Live:         map[string]string{"model": "gpt-3.5"},
				},
				Status: sync.StatusOK,
			},
		},
	}
	_ = logger.Write(report, "sync", nil, 0)

	entries := readLines(t, path)
	if len(entries[0].Changes) != 2 {
		t.Fatalf("expected 2 changes, got %d", len(entries[0].Changes))
	}

	c0 := entries[0].Changes[0]
	if c0.Action != "create" || c0.ResourceType != "channel" || c0.ResourceID != "openai-main" {
		t.Errorf("change[0] mismatch: %+v", c0)
	}
	if c0.Desired == nil {
		t.Error("change[0] desired should not be nil for create")
	}

	c1 := entries[0].Changes[1]
	if c1.Action != "update" || c1.ResourceType != "provider" {
		t.Errorf("change[1] mismatch: %+v", c1)
	}
	if c1.Live == nil {
		t.Error("change[1] live should not be nil for update")
	}
}

func TestNoChangeActionsExcludedFromChanges(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "audit.log")
	logger := NewLogger(path)

	report := &sync.SyncReport{
		TotalActions: 2,
		Succeeded:    2,
		Results: []sync.Result{
			{
				Action: sync.Action{Type: sync.ActionNoChange, ResourceType: "channel", ResourceID: "ch1"},
				Status: sync.StatusOK,
			},
			{
				Action: sync.Action{Type: sync.ActionCreate, ResourceType: "channel", ResourceID: "ch2"},
				Status: sync.StatusOK,
			},
		},
	}
	_ = logger.Write(report, "sync", nil, 0)

	entries := readLines(t, path)
	if len(entries[0].Changes) != 1 {
		t.Fatalf("expected 1 change (no-change excluded), got %d", len(entries[0].Changes))
	}
	if entries[0].Changes[0].ResourceID != "ch2" {
		t.Errorf("expected ch2, got %s", entries[0].Changes[0].ResourceID)
	}
}

func TestFailedActionErrorSerialized(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "audit.log")
	logger := NewLogger(path)

	report := &sync.SyncReport{
		TotalActions: 1,
		Failed:       1,
		Results: []sync.Result{
			{
				Action: sync.Action{Type: sync.ActionCreate, ResourceType: "channel", ResourceID: "ch1"},
				Status: sync.StatusFailed,
				Error:  errors.New("connection refused"),
			},
		},
	}
	_ = logger.Write(report, "sync", nil, 0)

	entries := readLines(t, path)
	if entries[0].Changes[0].Error != "connection refused" {
		t.Errorf("error = %q, want 'connection refused'", entries[0].Changes[0].Error)
	}
}

func TestLoggerCreatesParentDirectories(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "deep", "nested", "audit.log")
	logger := NewLogger(path)

	report := &sync.SyncReport{}
	err := logger.Write(report, "sync", nil, 0)
	if err != nil {
		t.Fatalf("Write failed: %v", err)
	}
	if _, err := os.Stat(path); os.IsNotExist(err) {
		t.Fatal("expected file to be created in nested directory")
	}
}

func TestNilSyncReportProducesZeroCounts(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "audit.log")
	logger := NewLogger(path)

	err := logger.Write(nil, "bootstrap", nil, 0)
	if err != nil {
		t.Fatalf("Write failed: %v", err)
	}

	entries := readLines(t, path)
	e := entries[0]
	if e.TotalActions != 0 || e.Succeeded != 0 || e.Failed != 0 {
		t.Errorf("expected zero counts for nil report, got total=%d succ=%d fail=%d",
			e.TotalActions, e.Succeeded, e.Failed)
	}
	if e.Operation != "bootstrap" {
		t.Errorf("operation = %q, want bootstrap", e.Operation)
	}
}
