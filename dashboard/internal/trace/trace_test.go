package trace

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// --- Test 1: Nginx log parsing ---

func TestTraceLogFile_Nginx(t *testing.T) {
	dir := t.TempDir()
	nginxLog := `192.168.1.1 - - [02/Jan/2024:10:00:05 +0000] "POST /v1/chat/completions HTTP/1.1" 200 1234 "-" "curl/7.88" upstream_time=1.234 req_id=test-123
192.168.1.1 - - [02/Jan/2024:10:00:06 +0000] "GET /healthz HTTP/1.1" 200 2 "-" "curl/7.88" upstream_time=0.001 req_id=other-456
`
	if err := os.WriteFile(filepath.Join(dir, "nginx.log"), []byte(nginxLog), 0644); err != nil {
		t.Fatal(err)
	}

	opts := TraceOptions{
		RequestID: "test-123",
		LogDir:    dir,
	}
	tracer := NewTracer(opts)
	result, err := tracer.Run()
	if err != nil {
		t.Fatal(err)
	}

	// Should find exactly one match in nginx layer
	nginxMatches := filterLayer(result.Layers, LayerNginx)
	if len(nginxMatches) != 1 {
		t.Fatalf("expected 1 nginx match, got %d", len(nginxMatches))
	}

	m := nginxMatches[0]
	if m.Layer != LayerNginx {
		t.Errorf("expected layer %q, got %q", LayerNginx, m.Layer)
	}
	if m.Timestamp == "" {
		t.Error("expected non-empty timestamp")
	}
	if !strings.Contains(m.Timestamp, "02/Jan/2024") {
		t.Errorf("expected timestamp to contain date, got %q", m.Timestamp)
	}
	if m.Metadata["status"] != "200" {
		t.Errorf("expected status=200, got %q", m.Metadata["status"])
	}
	if m.Metadata["upstream_time"] != "1.234" {
		t.Errorf("expected upstream_time=1.234, got %q", m.Metadata["upstream_time"])
	}
}

// --- Test 2: Bifrost JSON log parsing ---

func TestTraceLogFile_Bifrost(t *testing.T) {
	dir := t.TempDir()
	bifrostLog := `{"level":"info","timestamp":"2024-01-02T10:00:05Z","request_id":"test-123","provider":"openai","model":"gpt-4o","latency":"1.5s","msg":"request completed"}
{"level":"info","timestamp":"2024-01-02T10:00:06Z","request_id":"other-456","provider":"anthropic","model":"claude-3","msg":"request completed"}
`
	if err := os.WriteFile(filepath.Join(dir, "bifrost.log"), []byte(bifrostLog), 0644); err != nil {
		t.Fatal(err)
	}

	opts := TraceOptions{
		RequestID: "test-123",
		LogDir:    dir,
	}
	tracer := NewTracer(opts)
	result, err := tracer.Run()
	if err != nil {
		t.Fatal(err)
	}

	bifrostMatches := filterLayer(result.Layers, LayerBifrost)
	if len(bifrostMatches) != 1 {
		t.Fatalf("expected 1 bifrost match, got %d", len(bifrostMatches))
	}

	m := bifrostMatches[0]
	if m.Layer != LayerBifrost {
		t.Errorf("expected layer %q, got %q", LayerBifrost, m.Layer)
	}
	if m.Metadata["provider"] != "openai" {
		t.Errorf("expected provider=openai, got %q", m.Metadata["provider"])
	}
	if m.Metadata["model"] != "gpt-4o" {
		t.Errorf("expected model=gpt-4o, got %q", m.Metadata["model"])
	}
}

// --- Test 3: New-API log parsing ---

func TestTraceLogFile_NewAPI(t *testing.T) {
	dir := t.TempDir()
	newAPILog := `[INFO] 2024/01/02 10:00:05 relay_handler.go:123 userId=42 model="gpt-4o" channel=7 request_id=test-123 status=200
[INFO] 2024/01/02 10:00:06 relay_handler.go:124 userId=99 model="claude-3" channel=3 request_id=other-456 status=200
`
	if err := os.WriteFile(filepath.Join(dir, "new-api.log"), []byte(newAPILog), 0644); err != nil {
		t.Fatal(err)
	}

	opts := TraceOptions{
		RequestID: "test-123",
		LogDir:    dir,
	}
	tracer := NewTracer(opts)
	result, err := tracer.Run()
	if err != nil {
		t.Fatal(err)
	}

	napiMatches := filterLayer(result.Layers, LayerNewAPI)
	if len(napiMatches) != 1 {
		t.Fatalf("expected 1 new-api match, got %d", len(napiMatches))
	}

	m := napiMatches[0]
	if m.Layer != LayerNewAPI {
		t.Errorf("expected layer %q, got %q", LayerNewAPI, m.Layer)
	}
	if m.Metadata["user_id"] != "42" {
		t.Errorf("expected user_id=42, got %q", m.Metadata["user_id"])
	}
	if m.Metadata["model"] != "gpt-4o" {
		t.Errorf("expected model=gpt-4o, got %q", m.Metadata["model"])
	}
	if m.Metadata["channel"] != "7" {
		t.Errorf("expected channel=7, got %q", m.Metadata["channel"])
	}
}

// --- Test 4: No matches returns empty (no error) ---

func TestTraceLogFile_NoMatches(t *testing.T) {
	dir := t.TempDir()
	// Write logs with no matching request ID
	if err := os.WriteFile(filepath.Join(dir, "nginx.log"), []byte("no match here\n"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "bifrost.log"), []byte("no match here\n"), 0644); err != nil {
		t.Fatal(err)
	}

	opts := TraceOptions{
		RequestID: "nonexistent-id",
		LogDir:    dir,
	}
	tracer := NewTracer(opts)
	result, err := tracer.Run()
	if err != nil {
		t.Fatalf("expected no error on no matches, got: %v", err)
	}
	if len(result.Layers) != 0 {
		t.Errorf("expected 0 matches, got %d", len(result.Layers))
	}
}

// --- Test 5: FormatTextTrace ---

func TestFormatTextTrace(t *testing.T) {
	result := &TraceResult{
		RequestID: "test-123",
		Layers: []LayerMatch{
			{
				Layer:     LayerNginx,
				Timestamp: "02/Jan/2024:10:00:05",
				Line:      "full log line here",
				Metadata:  map[string]string{"status": "200", "upstream_time": "1.234"},
			},
			{
				Layer:     LayerBifrost,
				Timestamp: "2024-01-02T10:00:05Z",
				Line:      "bifrost log line",
				Metadata:  map[string]string{"provider": "openai", "model": "gpt-4o"},
			},
		},
	}

	text := FormatTextTrace(result, false)
	if !strings.Contains(text, "test-123") {
		t.Error("text output should contain request ID")
	}
	if !strings.Contains(text, "nginx") {
		t.Error("text output should contain nginx layer")
	}
	if !strings.Contains(text, "bifrost") {
		t.Error("text output should contain bifrost layer")
	}
	if !strings.Contains(text, "status=200") {
		t.Error("text output should contain metadata")
	}
	// Non-verbose should not contain the full log line
	if strings.Contains(text, "full log line here") {
		t.Error("non-verbose output should not contain full log line")
	}

	// Verbose mode
	verboseText := FormatTextTrace(result, true)
	if !strings.Contains(verboseText, "full log line here") {
		t.Error("verbose output should contain full log line")
	}
}

// --- Test 6: FormatJSONTrace ---

func TestFormatJSONTrace(t *testing.T) {
	result := &TraceResult{
		RequestID: "test-123",
		Layers: []LayerMatch{
			{
				Layer:     LayerNginx,
				Timestamp: "02/Jan/2024:10:00:05",
				Line:      "log line",
				Metadata:  map[string]string{"status": "200"},
			},
		},
	}

	data, err := FormatJSONTrace(result)
	if err != nil {
		t.Fatal(err)
	}

	// Must be valid JSON
	var parsed TraceResult
	if err := json.Unmarshal(data, &parsed); err != nil {
		t.Fatalf("output is not valid JSON: %v", err)
	}

	if parsed.RequestID != "test-123" {
		t.Errorf("expected request_id=test-123, got %q", parsed.RequestID)
	}
	if len(parsed.Layers) != 1 {
		t.Fatalf("expected 1 layer, got %d", len(parsed.Layers))
	}
	if parsed.Layers[0].Layer != LayerNginx {
		t.Errorf("expected layer=nginx, got %q", parsed.Layers[0].Layer)
	}
}

// --- Test: Docker log exec mock ---

func TestTraceDockerLogs(t *testing.T) {
	// Simulate docker logs output via mock exec
	opts := TraceOptions{
		RequestID:      "docker-req-001",
		ContainerNames: []string{"cp-nginx", "cp-bifrost"},
	}
	tracer := NewTracer(opts)
	tracer.SetExecCmd(func(name string, args ...string) ([]byte, error) {
		// Return fake docker logs based on container name
		container := args[len(args)-1]
		switch container {
		case "cp-nginx":
			return []byte(`192.168.1.1 - - [03/Jan/2024:11:00:00 +0000] "POST /v1/chat HTTP/1.1" 200 999 "-" "sdk/1.0" upstream_time=0.5 req_id=docker-req-001` + "\n"), nil
		case "cp-bifrost":
			return []byte(`{"level":"info","request_id":"docker-req-001","provider":"anthropic","model":"claude-3","latency":"0.8s"}` + "\n"), nil
		default:
			return []byte(""), nil
		}
	})

	result, err := tracer.Run()
	if err != nil {
		t.Fatal(err)
	}

	if len(result.Layers) != 2 {
		t.Fatalf("expected 2 matches, got %d", len(result.Layers))
	}
}

// --- Test: Missing log file degrades gracefully ---

func TestTraceLogFile_MissingFileGraceful(t *testing.T) {
	dir := t.TempDir()
	// Only create nginx log, leave others missing
	if err := os.WriteFile(filepath.Join(dir, "nginx.log"), []byte("req_id=test-123\n"), 0644); err != nil {
		t.Fatal(err)
	}

	opts := TraceOptions{
		RequestID: "test-123",
		LogDir:    dir,
	}
	tracer := NewTracer(opts)
	result, err := tracer.Run()
	if err != nil {
		t.Fatalf("expected no error with missing log files, got: %v", err)
	}

	// Should still find the nginx match
	if len(result.Layers) != 1 {
		t.Errorf("expected 1 match from available log, got %d", len(result.Layers))
	}
}

// --- helpers ---

func filterLayer(matches []LayerMatch, layer string) []LayerMatch {
	var out []LayerMatch
	for _, m := range matches {
		if m.Layer == layer {
			out = append(out, m)
		}
	}
	return out
}
