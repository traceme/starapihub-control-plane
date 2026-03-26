package store

import (
	"database/sql"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"time"

	_ "modernc.org/sqlite"
)

type Store struct {
	db *sql.DB
}

type Alert struct {
	ID           int64     `json:"id"`
	Type         string    `json:"type"`
	Severity     string    `json:"severity"`
	Signal       string    `json:"signal"`
	Service      string    `json:"service"`
	Message      string    `json:"message"`
	Timestamp    time.Time `json:"timestamp"`
	Acknowledged bool      `json:"acknowledged"`
}

type LogEntry struct {
	ID           int64     `json:"id"`
	Timestamp    time.Time `json:"timestamp"`
	Method       string    `json:"method"`
	Path         string    `json:"path"`
	Status       int       `json:"status"`
	LatencyMs    float64   `json:"latency_ms"`
	RequestID    string    `json:"request_id"`
	Model        string    `json:"model"`
	UpstreamTime float64   `json:"upstream_time"`
	CreatedAt    time.Time `json:"created_at"`
}

func New(dbPath string) (*Store, error) {
	dir := filepath.Dir(dbPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil, fmt.Errorf("create db directory: %w", err)
	}

	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return nil, fmt.Errorf("open database: %w", err)
	}

	// Set pragmas
	pragmas := []string{
		"PRAGMA journal_mode=WAL",
		"PRAGMA busy_timeout=5000",
		"PRAGMA synchronous=NORMAL",
	}
	for _, p := range pragmas {
		if _, err := db.Exec(p); err != nil {
			db.Close()
			return nil, fmt.Errorf("exec pragma %q: %w", p, err)
		}
	}

	// SQLite WAL supports concurrent readers but only one writer.
	// Limit connections to avoid "database is locked" under contention.
	db.SetMaxOpenConns(1)
	db.SetMaxIdleConns(1)

	if err := migrate(db); err != nil {
		db.Close()
		return nil, fmt.Errorf("migrate: %w", err)
	}

	return &Store{db: db}, nil
}

func migrate(db *sql.DB) error {
	stmts := []string{
		// v1.7 schema: alerts table without severity/signal columns.
		// CREATE TABLE IF NOT EXISTS is a no-op when the table already exists,
		// so this only creates the table on a fresh database.
		`CREATE TABLE IF NOT EXISTS alerts (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			type TEXT NOT NULL,
			severity TEXT NOT NULL DEFAULT '',
			signal TEXT NOT NULL DEFAULT '',
			service TEXT NOT NULL,
			message TEXT NOT NULL,
			timestamp DATETIME NOT NULL DEFAULT (datetime('now')),
			acknowledged INTEGER NOT NULL DEFAULT 0
		)`,
		`CREATE TABLE IF NOT EXISTS log_entries (
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
		)`,
		`CREATE INDEX IF NOT EXISTS idx_log_entries_created_at ON log_entries(created_at)`,
		`CREATE INDEX IF NOT EXISTS idx_log_entries_request_id ON log_entries(request_id)`,
		`CREATE INDEX IF NOT EXISTS idx_alerts_timestamp ON alerts(timestamp)`,
	}
	for _, s := range stmts {
		if _, err := db.Exec(s); err != nil {
			return fmt.Errorf("exec %q: %w", s, err)
		}
	}

	// v1.8 migration: add severity and signal columns to an existing v1.7 alerts table.
	// ALTER TABLE ADD COLUMN fails if the column already exists, so we swallow that error.
	alterStmts := []string{
		`ALTER TABLE alerts ADD COLUMN severity TEXT NOT NULL DEFAULT ''`,
		`ALTER TABLE alerts ADD COLUMN signal TEXT NOT NULL DEFAULT ''`,
	}
	for _, s := range alterStmts {
		if _, err := db.Exec(s); err != nil {
			// SQLite returns "duplicate column name" if column already exists — safe to ignore.
			if !isDuplicateColumnError(err) {
				return fmt.Errorf("exec %q: %w", s, err)
			}
		}
	}

	// Backfill: set severity = type for any legacy rows that have an empty severity.
	// Signal is left empty for legacy rows — there is no reliable way to infer it.
	if _, err := db.Exec(`UPDATE alerts SET severity = type WHERE severity = ''`); err != nil {
		return fmt.Errorf("backfill severity: %w", err)
	}

	return nil
}

// isDuplicateColumnError returns true if the error is a SQLite "duplicate column name" error.
func isDuplicateColumnError(err error) bool {
	return err != nil && strings.Contains(err.Error(), "duplicate column name")
}

func (s *Store) Close() error {
	return s.db.Close()
}

// InsertAlert creates a new alert.
func (s *Store) InsertAlert(a Alert) (int64, error) {
	res, err := s.db.Exec(
		`INSERT INTO alerts (type, severity, signal, service, message, timestamp) VALUES (?, ?, ?, ?, ?, ?)`,
		a.Type, a.Severity, a.Signal, a.Service, a.Message, a.Timestamp,
	)
	if err != nil {
		return 0, err
	}
	return res.LastInsertId()
}

// ListAlerts returns recent alerts, newest first.
func (s *Store) ListAlerts(limit int) ([]Alert, error) {
	if limit <= 0 {
		limit = 100
	}
	rows, err := s.db.Query(
		`SELECT id, type, severity, signal, service, message, timestamp, acknowledged FROM alerts ORDER BY timestamp DESC LIMIT ?`,
		limit,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var alerts []Alert
	for rows.Next() {
		var a Alert
		var ack int
		if err := rows.Scan(&a.ID, &a.Type, &a.Severity, &a.Signal, &a.Service, &a.Message, &a.Timestamp, &ack); err != nil {
			slog.Error("scan alert row", "error", err)
			continue
		}
		a.Acknowledged = ack != 0
		alerts = append(alerts, a)
	}
	return alerts, rows.Err()
}

// AcknowledgeAlert marks an alert as acknowledged.
func (s *Store) AcknowledgeAlert(id int64) error {
	res, err := s.db.Exec(`UPDATE alerts SET acknowledged = 1 WHERE id = ?`, id)
	if err != nil {
		return err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return fmt.Errorf("alert %d not found", id)
	}
	return nil
}

// InsertLogEntry inserts a parsed log line.
func (s *Store) InsertLogEntry(e LogEntry) error {
	_, err := s.db.Exec(
		`INSERT INTO log_entries (timestamp, method, path, status, latency_ms, request_id, model, upstream_time)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		e.Timestamp, e.Method, e.Path, e.Status, e.LatencyMs, e.RequestID, e.Model, e.UpstreamTime,
	)
	return err
}

// InsertLogEntries batch-inserts log entries.
func (s *Store) InsertLogEntries(entries []LogEntry) error {
	if len(entries) == 0 {
		return nil
	}
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	stmt, err := tx.Prepare(
		`INSERT INTO log_entries (timestamp, method, path, status, latency_ms, request_id, model, upstream_time)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
	)
	if err != nil {
		return err
	}
	defer stmt.Close()

	for _, e := range entries {
		if _, err := stmt.Exec(e.Timestamp, e.Method, e.Path, e.Status, e.LatencyMs, e.RequestID, e.Model, e.UpstreamTime); err != nil {
			slog.Error("insert log entry", "error", err)
		}
	}
	return tx.Commit()
}

// QueryLogs returns log entries matching the given filters.
// statusMin/statusMax define a range (e.g., 200-299 for "2xx"). Both 0 means no filter.
func (s *Store) QueryLogs(statusMin, statusMax int, model string, since, until time.Time, limit int) ([]LogEntry, error) {
	if limit <= 0 {
		limit = 200
	}
	if limit > 1000 {
		limit = 1000
	}
	query := `SELECT id, timestamp, method, path, status, latency_ms, request_id, model, upstream_time, created_at
	          FROM log_entries WHERE 1=1`
	var args []interface{}

	if statusMin > 0 && statusMax > 0 {
		if statusMin == statusMax {
			query += ` AND status = ?`
			args = append(args, statusMin)
		} else {
			query += ` AND status >= ? AND status <= ?`
			args = append(args, statusMin, statusMax)
		}
	}
	if model != "" {
		query += ` AND model = ?`
		args = append(args, model)
	}
	if !since.IsZero() {
		query += ` AND timestamp >= ?`
		args = append(args, since)
	}
	if !until.IsZero() {
		query += ` AND timestamp <= ?`
		args = append(args, until)
	}

	query += ` ORDER BY timestamp DESC LIMIT ?`
	args = append(args, limit)

	rows, err := s.db.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var entries []LogEntry
	for rows.Next() {
		var e LogEntry
		if err := rows.Scan(&e.ID, &e.Timestamp, &e.Method, &e.Path, &e.Status, &e.LatencyMs, &e.RequestID, &e.Model, &e.UpstreamTime, &e.CreatedAt); err != nil {
			slog.Error("scan log entry row", "error", err)
			continue
		}
		entries = append(entries, e)
	}
	return entries, rows.Err()
}

// GetLogByRequestID returns log entries for a given request ID.
func (s *Store) GetLogByRequestID(requestID string) ([]LogEntry, error) {
	rows, err := s.db.Query(
		`SELECT id, timestamp, method, path, status, latency_ms, request_id, model, upstream_time, created_at
		 FROM log_entries WHERE request_id = ? ORDER BY timestamp`,
		requestID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var entries []LogEntry
	for rows.Next() {
		var e LogEntry
		if err := rows.Scan(&e.ID, &e.Timestamp, &e.Method, &e.Path, &e.Status, &e.LatencyMs, &e.RequestID, &e.Model, &e.UpstreamTime, &e.CreatedAt); err != nil {
			continue
		}
		entries = append(entries, e)
	}
	return entries, rows.Err()
}

// CleanupOldLogs deletes log entries older than the given duration.
func (s *Store) CleanupOldLogs(maxAge time.Duration) error {
	cutoff := time.Now().Add(-maxAge)
	res, err := s.db.Exec(`DELETE FROM log_entries WHERE created_at < ?`, cutoff)
	if err != nil {
		return err
	}
	n, _ := res.RowsAffected()
	if n > 0 {
		slog.Info("cleaned up old log entries", "deleted", n)
	}
	return nil
}

// HasRecentAlert checks if a similar alert was already fired recently.
// Deduplicates on severity+service (not type, which is a backward-compat alias).
func (s *Store) HasRecentAlert(severity, service string, within time.Duration) (bool, error) {
	cutoff := time.Now().Add(-within)
	var count int
	err := s.db.QueryRow(
		`SELECT COUNT(*) FROM alerts WHERE severity = ? AND service = ? AND timestamp > ?`,
		severity, service, cutoff,
	).Scan(&count)
	if err != nil {
		return false, err
	}
	return count > 0, nil
}
