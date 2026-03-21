package store

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func newTestStore(t *testing.T) *Store {
	t.Helper()
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "test.db")
	s, err := New(dbPath)
	if err != nil {
		t.Fatalf("New(%q): %v", dbPath, err)
	}
	t.Cleanup(func() { s.Close() })
	return s
}

func TestNew(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "sub", "test.db")
	s, err := New(dbPath)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	defer s.Close()

	if s.db == nil {
		t.Fatal("expected non-nil db")
	}

	// Verify directory was created
	if _, err := os.Stat(filepath.Dir(dbPath)); os.IsNotExist(err) {
		t.Fatal("expected directory to be created")
	}
}

func TestNew_MaxOpenConns(t *testing.T) {
	s := newTestStore(t)
	// MaxOpenConns is set to 1 in New(); verify db is usable with sequential queries
	// (we can't directly read MaxOpenConns, but we can verify the db works under the constraint)
	for i := 0; i < 5; i++ {
		_, err := s.db.Exec("SELECT 1")
		if err != nil {
			t.Fatalf("query %d failed: %v", i, err)
		}
	}
}

func TestInsertAlert(t *testing.T) {
	s := newTestStore(t)
	now := time.Now().UTC().Truncate(time.Second)

	id, err := s.InsertAlert(Alert{
		Type:      "WARNING",
		Service:   "test-svc",
		Message:   "something happened",
		Timestamp: now,
	})
	if err != nil {
		t.Fatalf("InsertAlert: %v", err)
	}
	if id <= 0 {
		t.Fatalf("expected positive id, got %d", id)
	}
}

func TestListAlerts(t *testing.T) {
	s := newTestStore(t)

	// Insert multiple alerts
	for i := 0; i < 5; i++ {
		_, err := s.InsertAlert(Alert{
			Type:      "INFO",
			Service:   "svc",
			Message:   "msg",
			Timestamp: time.Now().Add(time.Duration(i) * time.Second),
		})
		if err != nil {
			t.Fatalf("InsertAlert %d: %v", i, err)
		}
	}

	alerts, err := s.ListAlerts(3)
	if err != nil {
		t.Fatalf("ListAlerts: %v", err)
	}
	if len(alerts) != 3 {
		t.Fatalf("expected 3 alerts, got %d", len(alerts))
	}
	// Newest first
	if alerts[0].ID < alerts[1].ID {
		t.Error("expected newest first ordering")
	}
}

func TestListAlerts_DefaultLimit(t *testing.T) {
	s := newTestStore(t)
	_, _ = s.InsertAlert(Alert{Type: "X", Service: "s", Message: "m", Timestamp: time.Now()})

	alerts, err := s.ListAlerts(0) // should default to 100
	if err != nil {
		t.Fatalf("ListAlerts: %v", err)
	}
	if len(alerts) != 1 {
		t.Fatalf("expected 1 alert, got %d", len(alerts))
	}
}

func TestAcknowledgeAlert(t *testing.T) {
	s := newTestStore(t)

	id, _ := s.InsertAlert(Alert{Type: "WARN", Service: "s", Message: "m", Timestamp: time.Now()})

	err := s.AcknowledgeAlert(id)
	if err != nil {
		t.Fatalf("AcknowledgeAlert: %v", err)
	}

	alerts, _ := s.ListAlerts(10)
	if len(alerts) != 1 {
		t.Fatalf("expected 1 alert, got %d", len(alerts))
	}
	if !alerts[0].Acknowledged {
		t.Error("expected alert to be acknowledged")
	}
}

func TestAcknowledgeAlert_NotFound(t *testing.T) {
	s := newTestStore(t)
	err := s.AcknowledgeAlert(99999)
	if err == nil {
		t.Fatal("expected error for nonexistent alert")
	}
}

func TestInsertLogEntries(t *testing.T) {
	s := newTestStore(t)
	now := time.Now().UTC().Truncate(time.Second)

	entries := []LogEntry{
		{Timestamp: now, Method: "GET", Path: "/api/test", Status: 200, LatencyMs: 10.5, RequestID: "req-1", Model: "gpt-4"},
		{Timestamp: now, Method: "POST", Path: "/v1/chat", Status: 500, LatencyMs: 100.0, RequestID: "req-2", Model: "claude"},
	}
	err := s.InsertLogEntries(entries)
	if err != nil {
		t.Fatalf("InsertLogEntries: %v", err)
	}

	// Verify they are queryable
	results, err := s.QueryLogs(0, 0, "", time.Time{}, time.Time{}, 10)
	if err != nil {
		t.Fatalf("QueryLogs: %v", err)
	}
	if len(results) != 2 {
		t.Fatalf("expected 2 entries, got %d", len(results))
	}
}

func TestInsertLogEntries_Empty(t *testing.T) {
	s := newTestStore(t)
	err := s.InsertLogEntries(nil)
	if err != nil {
		t.Fatalf("InsertLogEntries(nil): %v", err)
	}
}

func TestInsertLogEntry(t *testing.T) {
	s := newTestStore(t)
	err := s.InsertLogEntry(LogEntry{
		Timestamp: time.Now(), Method: "GET", Path: "/test", Status: 200,
		LatencyMs: 5.0, RequestID: "r1", Model: "m1",
	})
	if err != nil {
		t.Fatalf("InsertLogEntry: %v", err)
	}
}

func TestQueryLogs_StatusRange(t *testing.T) {
	s := newTestStore(t)
	now := time.Now().UTC().Truncate(time.Second)

	entries := []LogEntry{
		{Timestamp: now, Method: "GET", Path: "/a", Status: 200, RequestID: "r1"},
		{Timestamp: now, Method: "GET", Path: "/b", Status: 201, RequestID: "r2"},
		{Timestamp: now, Method: "GET", Path: "/c", Status: 404, RequestID: "r3"},
		{Timestamp: now, Method: "GET", Path: "/d", Status: 500, RequestID: "r4"},
	}
	if err := s.InsertLogEntries(entries); err != nil {
		t.Fatalf("InsertLogEntries: %v", err)
	}

	tests := []struct {
		name     string
		min, max int
		want     int
	}{
		{"2xx range", 200, 299, 2},
		{"exact 404", 404, 404, 1},
		{"5xx range", 500, 599, 1},
		{"no filter", 0, 0, 4},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			results, err := s.QueryLogs(tc.min, tc.max, "", time.Time{}, time.Time{}, 100)
			if err != nil {
				t.Fatalf("QueryLogs: %v", err)
			}
			if len(results) != tc.want {
				t.Errorf("expected %d entries, got %d", tc.want, len(results))
			}
		})
	}
}

func TestQueryLogs_ModelFilter(t *testing.T) {
	s := newTestStore(t)
	now := time.Now().UTC()

	entries := []LogEntry{
		{Timestamp: now, Method: "GET", Path: "/a", Status: 200, Model: "gpt-4"},
		{Timestamp: now, Method: "GET", Path: "/b", Status: 200, Model: "claude"},
		{Timestamp: now, Method: "GET", Path: "/c", Status: 200, Model: "gpt-4"},
	}
	s.InsertLogEntries(entries)

	results, err := s.QueryLogs(0, 0, "gpt-4", time.Time{}, time.Time{}, 100)
	if err != nil {
		t.Fatalf("QueryLogs: %v", err)
	}
	if len(results) != 2 {
		t.Errorf("expected 2 gpt-4 entries, got %d", len(results))
	}
}

func TestQueryLogs_TimeFilter(t *testing.T) {
	s := newTestStore(t)
	base := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)

	entries := []LogEntry{
		{Timestamp: base, Method: "GET", Path: "/a", Status: 200},
		{Timestamp: base.Add(1 * time.Hour), Method: "GET", Path: "/b", Status: 200},
		{Timestamp: base.Add(2 * time.Hour), Method: "GET", Path: "/c", Status: 200},
	}
	s.InsertLogEntries(entries)

	results, err := s.QueryLogs(0, 0, "", base.Add(30*time.Minute), base.Add(90*time.Minute), 100)
	if err != nil {
		t.Fatalf("QueryLogs: %v", err)
	}
	if len(results) != 1 {
		t.Errorf("expected 1 entry in time range, got %d", len(results))
	}
}

func TestQueryLogs_LimitClamping(t *testing.T) {
	s := newTestStore(t)
	now := time.Now().UTC()

	for i := 0; i < 5; i++ {
		s.InsertLogEntry(LogEntry{Timestamp: now, Method: "GET", Path: "/x", Status: 200})
	}

	// Limit 2
	results, _ := s.QueryLogs(0, 0, "", time.Time{}, time.Time{}, 2)
	if len(results) != 2 {
		t.Errorf("expected 2 with limit=2, got %d", len(results))
	}

	// Negative limit defaults to 200
	results, _ = s.QueryLogs(0, 0, "", time.Time{}, time.Time{}, -1)
	if len(results) != 5 {
		t.Errorf("expected 5 with default limit, got %d", len(results))
	}
}

func TestGetLogByRequestID(t *testing.T) {
	s := newTestStore(t)
	now := time.Now().UTC()

	s.InsertLogEntry(LogEntry{Timestamp: now, Method: "GET", Path: "/a", Status: 200, RequestID: "abc-123"})
	s.InsertLogEntry(LogEntry{Timestamp: now, Method: "GET", Path: "/b", Status: 200, RequestID: "xyz-456"})

	entries, err := s.GetLogByRequestID("abc-123")
	if err != nil {
		t.Fatalf("GetLogByRequestID: %v", err)
	}
	if len(entries) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(entries))
	}
	if entries[0].RequestID != "abc-123" {
		t.Errorf("expected request_id abc-123, got %s", entries[0].RequestID)
	}
}

func TestGetLogByRequestID_NotFound(t *testing.T) {
	s := newTestStore(t)

	entries, err := s.GetLogByRequestID("nonexistent")
	if err != nil {
		t.Fatalf("GetLogByRequestID: %v", err)
	}
	if len(entries) != 0 {
		t.Errorf("expected 0 entries, got %d", len(entries))
	}
}

func TestHasRecentAlert(t *testing.T) {
	s := newTestStore(t)

	s.InsertAlert(Alert{Type: "WARN", Service: "svc", Message: "m", Timestamp: time.Now()})

	has, err := s.HasRecentAlert("WARN", "svc", 1*time.Minute)
	if err != nil {
		t.Fatalf("HasRecentAlert: %v", err)
	}
	if !has {
		t.Error("expected recent alert to be found")
	}

	has, err = s.HasRecentAlert("CRITICAL", "svc", 1*time.Minute)
	if err != nil {
		t.Fatalf("HasRecentAlert: %v", err)
	}
	if has {
		t.Error("expected no recent CRITICAL alert")
	}
}

func TestCleanupOldLogs(t *testing.T) {
	s := newTestStore(t)

	// Insert a log entry directly with an old created_at
	old := time.Now().Add(-48 * time.Hour)
	_, err := s.db.Exec(
		`INSERT INTO log_entries (timestamp, method, path, status, latency_ms, request_id, model, upstream_time, created_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		old, "GET", "/old", 200, 1.0, "old-req", "", 0.0, old,
	)
	if err != nil {
		t.Fatalf("insert old entry: %v", err)
	}

	// Insert a recent entry
	s.InsertLogEntry(LogEntry{Timestamp: time.Now(), Method: "GET", Path: "/new", Status: 200})

	err = s.CleanupOldLogs(24 * time.Hour)
	if err != nil {
		t.Fatalf("CleanupOldLogs: %v", err)
	}

	entries, _ := s.QueryLogs(0, 0, "", time.Time{}, time.Time{}, 100)
	if len(entries) != 1 {
		t.Errorf("expected 1 entry after cleanup, got %d", len(entries))
	}
}

func TestNew_WALMode(t *testing.T) {
	s := newTestStore(t)
	var mode string
	err := s.db.QueryRow("PRAGMA journal_mode").Scan(&mode)
	if err != nil {
		t.Fatalf("PRAGMA journal_mode: %v", err)
	}
	if mode != "wal" {
		t.Errorf("expected WAL mode, got %q", mode)
	}
}
