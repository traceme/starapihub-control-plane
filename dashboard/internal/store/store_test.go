package store

import (
	"database/sql"
	"os"
	"path/filepath"
	"testing"
	"time"

	_ "modernc.org/sqlite"
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

	s.InsertAlert(Alert{Type: "WARNING", Severity: "WARNING", Signal: "config-drift", Service: "svc", Message: "m", Timestamp: time.Now()})

	has, err := s.HasRecentAlert("WARNING", "svc", 1*time.Minute)
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

// TestMigrate_LegacyV17Database creates a v1.7 alerts table (no severity/signal columns),
// inserts a legacy row, then opens the store (which runs migrate) and proves:
// 1. The migration succeeds without error
// 2. The severity column was added and backfilled from type
// 3. The signal column was added and left empty (no way to infer it)
// 4. Legacy data is preserved
func TestMigrate_LegacyV17Database(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "legacy.db")

	// Step 1: Create a v1.7-style database with the old schema (no severity/signal)
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		t.Fatalf("open raw db: %v", err)
	}
	_, err = db.Exec(`CREATE TABLE alerts (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		type TEXT NOT NULL,
		service TEXT NOT NULL,
		message TEXT NOT NULL,
		timestamp DATETIME NOT NULL DEFAULT (datetime('now')),
		acknowledged INTEGER NOT NULL DEFAULT 0
	)`)
	if err != nil {
		t.Fatalf("create legacy alerts table: %v", err)
	}
	// Also create the log_entries table (v1.7 had it)
	_, err = db.Exec(`CREATE TABLE log_entries (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		timestamp DATETIME NOT NULL,
		method TEXT NOT NULL,
		path TEXT NOT NULL,
		status INTEGER NOT NULL,
		latency_ms REAL NOT NULL DEFAULT 0,
		request_id TEXT NOT NULL DEFAULT '',
		model TEXT NOT NULL DEFAULT '',
		upstream_time REAL NOT NULL DEFAULT 0,
		created_at DATETIME NOT NULL DEFAULT (datetime('now'))
	)`)
	if err != nil {
		t.Fatalf("create legacy log_entries table: %v", err)
	}

	// Insert a legacy alert (no severity/signal columns)
	_, err = db.Exec(`INSERT INTO alerts (type, service, message, timestamp) VALUES (?, ?, ?, ?)`,
		"CRITICAL", "new-api", "legacy alert", time.Now())
	if err != nil {
		t.Fatalf("insert legacy alert: %v", err)
	}
	db.Close()

	// Step 2: Open with store.New — this runs migrate()
	s, err := New(dbPath)
	if err != nil {
		t.Fatalf("store.New on legacy db: %v", err)
	}
	defer s.Close()

	// Step 3: Verify migration results
	alerts, err := s.ListAlerts(10)
	if err != nil {
		t.Fatalf("ListAlerts after migration: %v", err)
	}
	if len(alerts) != 1 {
		t.Fatalf("expected 1 legacy alert, got %d", len(alerts))
	}

	a := alerts[0]
	if a.Type != "CRITICAL" {
		t.Errorf("expected type CRITICAL, got %s", a.Type)
	}
	if a.Severity != "CRITICAL" {
		t.Errorf("expected severity backfilled to CRITICAL, got %q", a.Severity)
	}
	if a.Signal != "" {
		t.Errorf("expected signal empty for legacy row, got %q", a.Signal)
	}
	if a.Service != "new-api" {
		t.Errorf("expected service new-api, got %s", a.Service)
	}
	if a.Message != "legacy alert" {
		t.Errorf("expected message 'legacy alert', got %s", a.Message)
	}

	// Step 4: Verify new alerts can be inserted with all fields
	id, err := s.InsertAlert(Alert{
		Type:      "WARNING",
		Severity:  "WARNING",
		Signal:    "config-drift",
		Service:   "drift",
		Message:   "new v1.8 alert",
		Timestamp: time.Now(),
	})
	if err != nil {
		t.Fatalf("InsertAlert after migration: %v", err)
	}
	if id <= 0 {
		t.Fatalf("expected positive id, got %d", id)
	}

	alerts, _ = s.ListAlerts(10)
	if len(alerts) != 2 {
		t.Fatalf("expected 2 alerts total, got %d", len(alerts))
	}
}

// TestMigrate_FreshDatabase verifies that migrate works on a completely fresh database.
func TestMigrate_FreshDatabase(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "fresh.db")

	s, err := New(dbPath)
	if err != nil {
		t.Fatalf("store.New on fresh db: %v", err)
	}
	defer s.Close()

	// Insert an alert with all v1.8 fields
	id, err := s.InsertAlert(Alert{
		Type:      "CRITICAL",
		Severity:  "CRITICAL",
		Signal:    "service-down",
		Service:   "bifrost",
		Message:   "fresh db test",
		Timestamp: time.Now(),
	})
	if err != nil {
		t.Fatalf("InsertAlert: %v", err)
	}
	if id <= 0 {
		t.Fatalf("expected positive id, got %d", id)
	}

	alerts, _ := s.ListAlerts(10)
	if len(alerts) != 1 {
		t.Fatalf("expected 1 alert, got %d", len(alerts))
	}
	if alerts[0].Severity != "CRITICAL" || alerts[0].Signal != "service-down" {
		t.Errorf("unexpected severity=%q signal=%q", alerts[0].Severity, alerts[0].Signal)
	}
}

// TestMigrate_Idempotent verifies that running migrate twice doesn't fail.
func TestMigrate_Idempotent(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "idempotent.db")

	// First open creates schema
	s1, err := New(dbPath)
	if err != nil {
		t.Fatalf("first store.New: %v", err)
	}
	s1.InsertAlert(Alert{Type: "INFO", Severity: "INFO", Signal: "test", Service: "s", Message: "m", Timestamp: time.Now()})
	s1.Close()

	// Second open runs migrate again — should not fail
	s2, err := New(dbPath)
	if err != nil {
		t.Fatalf("second store.New (idempotent): %v", err)
	}
	defer s2.Close()

	alerts, _ := s2.ListAlerts(10)
	if len(alerts) != 1 {
		t.Fatalf("expected 1 alert after idempotent migration, got %d", len(alerts))
	}
}
