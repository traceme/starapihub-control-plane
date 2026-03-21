package poller

import (
	"os"
	"path/filepath"
	"regexp"
	"testing"
	"time"

	"github.com/starapihub/dashboard/internal/store"
)

func TestNewSystemState(t *testing.T) {
	s := NewSystemState()
	if s == nil {
		t.Fatal("expected non-nil SystemState")
	}
	if s.Health == nil {
		t.Error("expected Health map to be initialized")
	}
	if s.Cookies == nil {
		t.Error("expected Cookies map to be initialized")
	}
	if s.LogStats.ByModel == nil {
		t.Error("expected LogStats.ByModel map to be initialized")
	}
	if s.LogStats.ByStatus == nil {
		t.Error("expected LogStats.ByStatus map to be initialized")
	}
	if s.LogStats.Period != "60s" {
		t.Errorf("expected Period 60s, got %s", s.LogStats.Period)
	}
}

func TestSetHealth_GetHealth(t *testing.T) {
	s := NewSystemState()

	h := ServiceHealth{Status: "healthy", URL: "http://localhost:3000", Latency: 42}
	s.SetHealth("new-api", h)

	got := s.GetHealth()
	if len(got) != 1 {
		t.Fatalf("expected 1 health entry, got %d", len(got))
	}
	if got["new-api"].Status != "healthy" {
		t.Errorf("expected healthy, got %s", got["new-api"].Status)
	}
	if got["new-api"].Latency != 42 {
		t.Errorf("expected latency 42, got %d", got["new-api"].Latency)
	}
}

func TestSetHealth_UnhealthyTracking(t *testing.T) {
	s := NewSystemState()

	// Set unhealthy
	s.SetHealth("svc", ServiceHealth{Status: "unhealthy"})
	since, ok := s.GetUnhealthySince("svc")
	if !ok {
		t.Fatal("expected unhealthySince entry")
	}
	if time.Since(since) > 1*time.Second {
		t.Error("unhealthySince should be recent")
	}

	// Set healthy again - should clear
	s.SetHealth("svc", ServiceHealth{Status: "healthy"})
	_, ok = s.GetUnhealthySince("svc")
	if ok {
		t.Error("expected unhealthySince to be cleared after healthy")
	}
}

func TestSetCookies_GetCookies(t *testing.T) {
	s := NewSystemState()

	cs := CookieStatus{Valid: 5, Exhausted: 2, Invalid: 1, Total: 8}
	s.SetCookies("clewdr-1", cs)

	got := s.GetCookies()
	if len(got) != 1 {
		t.Fatalf("expected 1 cookie entry, got %d", len(got))
	}
	if got["clewdr-1"].Valid != 5 {
		t.Errorf("expected 5 valid, got %d", got["clewdr-1"].Valid)
	}
}

func TestSetLogStats(t *testing.T) {
	s := NewSystemState()

	ls := LogStats{
		RequestRate: 1.5,
		P50Latency:  10.0,
		P99Latency:  100.0,
		ErrorRate:   0.05,
		ByModel:     map[string]int{"gpt-4": 10},
		ByStatus:    map[int]int{200: 90, 500: 5},
		Period:      "60s",
	}
	s.SetLogStats(ls)

	snap := s.GetSnapshot()
	if snap.LogStats.RequestRate != 1.5 {
		t.Errorf("expected request rate 1.5, got %f", snap.LogStats.RequestRate)
	}
}

func TestGetSnapshot_DeepCopy(t *testing.T) {
	s := NewSystemState()

	s.SetHealth("svc", ServiceHealth{Status: "healthy"})
	s.SetCookies("c1", CookieStatus{Valid: 3})
	s.SetLogStats(LogStats{
		ByModel:  map[string]int{"m1": 1},
		ByStatus: map[int]int{200: 1},
		Period:   "60s",
	})

	snap := s.GetSnapshot()

	// Mutate the snapshot and verify original is unchanged
	snap.Health["svc"] = ServiceHealth{Status: "mutated"}
	snap.Cookies["c1"] = CookieStatus{Valid: 999}
	snap.LogStats.ByModel["m1"] = 999
	snap.LogStats.ByStatus[200] = 999

	orig := s.GetSnapshot()
	if orig.Health["svc"].Status != "healthy" {
		t.Error("snapshot mutation affected original Health")
	}
	if orig.Cookies["c1"].Valid != 3 {
		t.Error("snapshot mutation affected original Cookies")
	}
	if orig.LogStats.ByModel["m1"] != 1 {
		t.Error("snapshot mutation affected original ByModel")
	}
	if orig.LogStats.ByStatus[200] != 1 {
		t.Error("snapshot mutation affected original ByStatus")
	}
}

func TestGetUnhealthySince_NotPresent(t *testing.T) {
	s := NewSystemState()
	_, ok := s.GetUnhealthySince("nonexistent")
	if ok {
		t.Error("expected false for nonexistent service")
	}
}

func testLogRegex() *regexp.Regexp {
	return regexp.MustCompile(
		`^(\S+)\s+\[([^\]]+)\]\s+"(\S+)\s+(\S+)\s+\S+"\s+(\d+)\s+req_id=(\S+)\s+upstream=(\S+)`,
	)
}

func TestParseLine_Valid(t *testing.T) {
	re := testLogRegex()
	line := `192.168.1.1 [15/Jan/2025:10:30:00 +0000] "POST /v1/chat/completions HTTP/1.1" 200 req_id=abc-123 upstream=0.450`

	entry, ok := parseLine(line, re)
	if !ok {
		t.Fatal("expected parseLine to succeed")
	}
	if entry.Method != "POST" {
		t.Errorf("expected POST, got %s", entry.Method)
	}
	if entry.Path != "/v1/chat/completions" {
		t.Errorf("expected /v1/chat/completions, got %s", entry.Path)
	}
	if entry.Status != 200 {
		t.Errorf("expected 200, got %d", entry.Status)
	}
	if entry.RequestID != "abc-123" {
		t.Errorf("expected abc-123, got %s", entry.RequestID)
	}
	if entry.UpstreamTime != 0.450 {
		t.Errorf("expected 0.450, got %f", entry.UpstreamTime)
	}
	// LatencyMs should be upstream_time * 1000
	if entry.LatencyMs != 450.0 {
		t.Errorf("expected 450.0ms, got %f", entry.LatencyMs)
	}
}

func TestParseLine_Invalid(t *testing.T) {
	re := testLogRegex()
	_, ok := parseLine("this is not a valid log line", re)
	if ok {
		t.Error("expected parseLine to fail on invalid line")
	}
}

func TestParseLine_BadTime(t *testing.T) {
	re := testLogRegex()
	line := `10.0.0.1 [bad-time-format] "GET /test HTTP/1.1" 200 req_id=r1 upstream=0.1`
	entry, ok := parseLine(line, re)
	if !ok {
		t.Fatal("expected parseLine to succeed even with bad time")
	}
	// Should default to approximately now
	if time.Since(entry.Timestamp) > 2*time.Second {
		t.Error("expected timestamp close to now for bad time format")
	}
}

func TestDetectModel(t *testing.T) {
	tests := []struct {
		path string
		want string
	}{
		{"/v1/models/claude-3-opus/completions", "claude-3-opus"},
		{"/v1/chat/completions", ""},
		{"/api/models/gpt-4", "gpt-4"},
		{"/", ""},
		{"/models", ""},
	}
	for _, tc := range tests {
		t.Run(tc.path, func(t *testing.T) {
			got := detectModel(tc.path)
			if got != tc.want {
				t.Errorf("detectModel(%q) = %q, want %q", tc.path, got, tc.want)
			}
		})
	}
}

func TestClewdrName(t *testing.T) {
	tests := []struct {
		url  string
		want string
	}{
		{"http://clewdr-1:8484", "clewdr-1"},
		{"https://clewdr-2:8484", "clewdr-2"},
		{"http://localhost:8484", "localhost"},
		{"clewdr-3:8484", "clewdr-3"},
	}
	for _, tc := range tests {
		t.Run(tc.url, func(t *testing.T) {
			got := clewdrName(tc.url)
			if got != tc.want {
				t.Errorf("clewdrName(%q) = %q, want %q", tc.url, got, tc.want)
			}
		})
	}
}

func TestComputeStats_Empty(t *testing.T) {
	stats := computeStats(nil)
	if stats.RequestRate != 0 {
		t.Errorf("expected 0 request rate, got %f", stats.RequestRate)
	}
	if stats.Period != "60s" {
		t.Errorf("expected period 60s, got %s", stats.Period)
	}
}

func TestComputeStats(t *testing.T) {
	entries := []store.LogEntry{
		{Status: 200, LatencyMs: 10, Model: "gpt-4"},
		{Status: 200, LatencyMs: 20, Model: "gpt-4"},
		{Status: 500, LatencyMs: 100, Model: "claude"},
		{Status: 200, LatencyMs: 30, Model: ""},
	}

	stats := computeStats(entries)

	if stats.RequestRate != 4.0/60.0 {
		t.Errorf("expected rate %f, got %f", 4.0/60.0, stats.RequestRate)
	}
	if stats.ErrorRate != 0.25 {
		t.Errorf("expected error rate 0.25, got %f", stats.ErrorRate)
	}
	if stats.ByModel["gpt-4"] != 2 {
		t.Errorf("expected 2 gpt-4, got %d", stats.ByModel["gpt-4"])
	}
	if stats.ByModel["claude"] != 1 {
		t.Errorf("expected 1 claude, got %d", stats.ByModel["claude"])
	}
	if _, ok := stats.ByModel[""]; ok {
		t.Error("empty model should not be counted in ByModel")
	}
	if stats.ByStatus[200] != 3 {
		t.Errorf("expected 3 status 200, got %d", stats.ByStatus[200])
	}
	if stats.ByStatus[500] != 1 {
		t.Errorf("expected 1 status 500, got %d", stats.ByStatus[500])
	}
}

func TestPercentile(t *testing.T) {
	tests := []struct {
		name   string
		sorted []float64
		p      float64
		want   float64
	}{
		{"empty", nil, 0.5, 0},
		{"single", []float64{42}, 0.5, 42},
		{"p50 even", []float64{10, 20, 30, 40}, 0.5, 25},
		{"p99", []float64{1, 2, 3, 4, 5, 6, 7, 8, 9, 10}, 0.99, 9.91},
		{"p0", []float64{1, 2, 3}, 0.0, 1},
		{"p100", []float64{1, 2, 3}, 1.0, 3},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := percentile(tc.sorted, tc.p)
			diff := got - tc.want
			if diff < -0.01 || diff > 0.01 {
				t.Errorf("percentile(%v, %f) = %f, want %f", tc.sorted, tc.p, got, tc.want)
			}
		})
	}
}

func TestTailLog_FileNotExist(t *testing.T) {
	re := testLogRegex()
	entries, offset, inode := tailLog("/nonexistent/path/access.log", 0, 0, re)
	if len(entries) != 0 {
		t.Error("expected no entries for nonexistent file")
	}
	if offset != 0 {
		t.Error("expected offset 0")
	}
	if inode != 0 {
		t.Error("expected inode 0")
	}
}

func TestTailLog_ReadNewLines(t *testing.T) {
	re := testLogRegex()
	dir := t.TempDir()
	logPath := filepath.Join(dir, "access.log")

	// Write initial content
	line1 := `10.0.0.1 [15/Jan/2025:10:00:00 +0000] "GET /health HTTP/1.1" 200 req_id=r1 upstream=0.01` + "\n"
	os.WriteFile(logPath, []byte(line1), 0644)

	entries, offset, inode := tailLog(logPath, 0, 0, re)
	if len(entries) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(entries))
	}
	if entries[0].RequestID != "r1" {
		t.Errorf("expected r1, got %s", entries[0].RequestID)
	}

	// Append more lines
	line2 := `10.0.0.2 [15/Jan/2025:10:01:00 +0000] "POST /api HTTP/1.1" 201 req_id=r2 upstream=0.05` + "\n"
	f, _ := os.OpenFile(logPath, os.O_APPEND|os.O_WRONLY, 0644)
	f.WriteString(line2)
	f.Close()

	entries2, offset2, _ := tailLog(logPath, offset, inode, re)
	if len(entries2) != 1 {
		t.Fatalf("expected 1 new entry, got %d", len(entries2))
	}
	if entries2[0].RequestID != "r2" {
		t.Errorf("expected r2, got %s", entries2[0].RequestID)
	}
	if offset2 <= offset {
		t.Error("expected offset to advance")
	}
}

func TestTailLog_Truncation(t *testing.T) {
	re := testLogRegex()
	dir := t.TempDir()
	logPath := filepath.Join(dir, "access.log")

	// Write a long file
	content := `10.0.0.1 [15/Jan/2025:10:00:00 +0000] "GET /a HTTP/1.1" 200 req_id=r1 upstream=0.01` + "\n"
	content += `10.0.0.1 [15/Jan/2025:10:00:01 +0000] "GET /b HTTP/1.1" 200 req_id=r2 upstream=0.02` + "\n"
	os.WriteFile(logPath, []byte(content), 0644)

	_, offset, inode := tailLog(logPath, 0, 0, re)

	// Truncate the file (simulating log rotation via truncation)
	os.WriteFile(logPath, []byte(`10.0.0.1 [15/Jan/2025:10:00:02 +0000] "GET /c HTTP/1.1" 200 req_id=r3 upstream=0.03`+"\n"), 0644)

	entries, _, _ := tailLog(logPath, offset, inode, re)
	if len(entries) != 1 {
		t.Fatalf("expected 1 entry after truncation, got %d", len(entries))
	}
	if entries[0].RequestID != "r3" {
		t.Errorf("expected r3, got %s", entries[0].RequestID)
	}
}

func TestTailLog_InodeChange(t *testing.T) {
	re := testLogRegex()
	dir := t.TempDir()
	logPath := filepath.Join(dir, "access.log")

	line := `10.0.0.1 [15/Jan/2025:10:00:00 +0000] "GET /a HTTP/1.1" 200 req_id=r1 upstream=0.01` + "\n"
	os.WriteFile(logPath, []byte(line), 0644)

	_, offset, inode := tailLog(logPath, 0, 0, re)

	// Simulate inode change by removing and recreating
	os.Remove(logPath)
	newLine := `10.0.0.2 [15/Jan/2025:10:01:00 +0000] "POST /b HTTP/1.1" 201 req_id=r2 upstream=0.05` + "\n"
	os.WriteFile(logPath, []byte(newLine), 0644)

	entries, _, _ := tailLog(logPath, offset, inode, re)
	// After inode change, offset resets to 0, so we should read the new file
	if len(entries) != 1 {
		t.Fatalf("expected 1 entry after inode change, got %d", len(entries))
	}
}

func TestTailLog_NoNewContent(t *testing.T) {
	re := testLogRegex()
	dir := t.TempDir()
	logPath := filepath.Join(dir, "access.log")

	line := `10.0.0.1 [15/Jan/2025:10:00:00 +0000] "GET /a HTTP/1.1" 200 req_id=r1 upstream=0.01` + "\n"
	os.WriteFile(logPath, []byte(line), 0644)

	_, offset, inode := tailLog(logPath, 0, 0, re)

	// Read again with same offset - should get nothing
	entries, offset2, _ := tailLog(logPath, offset, inode, re)
	if len(entries) != 0 {
		t.Errorf("expected 0 entries, got %d", len(entries))
	}
	if offset2 != offset {
		t.Errorf("expected offset unchanged, got %d vs %d", offset2, offset)
	}
}
