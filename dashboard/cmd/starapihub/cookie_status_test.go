package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestCookieStatusCommand(t *testing.T) {
	cmd := cookieStatusCmd()
	if cmd.Use != "cookie-status" {
		t.Errorf("expected Use 'cookie-status', got %q", cmd.Use)
	}
}

func TestCookieStatusNoEnv(t *testing.T) {
	for _, env := range []string{"CLEWDR_URLS", "CLEWDR_ADMIN_TOKEN", "CLEWDR_ADMIN_TOKENS"} {
		t.Setenv(env, "")
	}

	rootCmd := buildRootCmd()
	buf := new(bytes.Buffer)
	rootCmd.SetOut(buf)
	rootCmd.SetErr(buf)
	rootCmd.SetArgs([]string{"cookie-status"})

	err := rootCmd.Execute()
	if err == nil {
		t.Fatal("expected error when no CLEWDR_URLS set")
	}
	var exitErr *ExitError
	if !errors.As(err, &exitErr) {
		t.Fatalf("expected ExitError, got: %T: %v", err, err)
	}
	if exitErr.Code != 1 {
		t.Errorf("expected exit code 1, got %d", exitErr.Code)
	}
	output := buf.String()
	if !strings.Contains(output, "No ClewdR instances configured") {
		t.Errorf("expected 'No ClewdR instances configured' message, got: %s", output)
	}
}

func TestCookieStatusMinValid_PerInstance(t *testing.T) {
	// Set up a test server that returns cookie data with only 1 valid cookie.
	// --min-valid is per-instance, so 1 < 2 should fail.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/cookies" {
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(map[string]any{
				"valid":     []any{map[string]string{"cookie": "c1"}},
				"exhausted": []any{map[string]string{"cookie": "c2"}},
				"invalid":   []any{},
			})
			return
		}
		w.WriteHeader(200)
	}))
	defer srv.Close()

	t.Setenv("CLEWDR_URLS", srv.URL)
	t.Setenv("CLEWDR_ADMIN_TOKENS", "test-token")
	t.Setenv("CLEWDR_ADMIN_TOKEN", "")

	rootCmd := buildRootCmd()
	buf := new(bytes.Buffer)
	rootCmd.SetOut(buf)
	rootCmd.SetErr(buf)
	rootCmd.SetArgs([]string{"cookie-status", "--min-valid", "2"})

	err := rootCmd.Execute()
	// 1 valid < 2 min-valid per instance => should exit non-zero
	if err == nil {
		t.Fatal("expected error when valid count < min-valid per instance")
	}
	var exitErr *ExitError
	if !errors.As(err, &exitErr) {
		t.Fatalf("expected ExitError, got: %T: %v", err, err)
	}
	if exitErr.Code != 1 {
		t.Errorf("expected exit code 1, got %d", exitErr.Code)
	}
}

func TestCookieStatusJSONOutput(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/cookies" {
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(map[string]any{
				"valid":     []any{map[string]string{"cookie": "c1"}, map[string]string{"cookie": "c2"}},
				"exhausted": []any{map[string]string{"cookie": "c3"}},
				"invalid":   []any{map[string]string{"cookie": "c4"}},
			})
			return
		}
		w.WriteHeader(200)
	}))
	defer srv.Close()

	t.Setenv("CLEWDR_URLS", srv.URL)
	t.Setenv("CLEWDR_ADMIN_TOKENS", "test-token")
	t.Setenv("CLEWDR_ADMIN_TOKEN", "")

	rootCmd := buildRootCmd()
	buf := new(bytes.Buffer)
	rootCmd.SetOut(buf)
	rootCmd.SetErr(buf)
	rootCmd.SetArgs([]string{"cookie-status", "--output", "json"})

	err := rootCmd.Execute()
	if err != nil {
		t.Fatalf("expected no error when valid >= min-valid, got: %v", err)
	}

	// Parse JSON output
	var results []struct {
		Instance  string `json:"instance"`
		Valid     int    `json:"valid"`
		Exhausted int    `json:"exhausted"`
		Invalid   int    `json:"invalid"`
		Total     int    `json:"total"`
	}
	if err := json.Unmarshal(buf.Bytes(), &results); err != nil {
		t.Fatalf("failed to parse JSON output: %v\nraw: %s", err, buf.String())
	}
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	r := results[0]
	if r.Valid != 2 {
		t.Errorf("expected valid=2, got %d", r.Valid)
	}
	if r.Exhausted != 1 {
		t.Errorf("expected exhausted=1, got %d", r.Exhausted)
	}
	if r.Invalid != 1 {
		t.Errorf("expected invalid=1, got %d", r.Invalid)
	}
	if r.Total != 4 {
		t.Errorf("expected total=4, got %d", r.Total)
	}
}

func TestCookieStatusTextOutput(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/cookies" {
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(map[string]any{
				"valid":     []any{map[string]string{"cookie": "c1"}, map[string]string{"cookie": "c2"}, map[string]string{"cookie": "c3"}},
				"exhausted": []any{map[string]string{"cookie": "c4"}},
				"invalid":   []any{},
			})
			return
		}
		w.WriteHeader(200)
	}))
	defer srv.Close()

	t.Setenv("CLEWDR_URLS", srv.URL)
	t.Setenv("CLEWDR_ADMIN_TOKENS", "test-token")
	t.Setenv("CLEWDR_ADMIN_TOKEN", "")

	rootCmd := buildRootCmd()
	buf := new(bytes.Buffer)
	rootCmd.SetOut(buf)
	rootCmd.SetErr(buf)
	rootCmd.SetArgs([]string{"cookie-status"})

	err := rootCmd.Execute()
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	output := buf.String()
	if !strings.Contains(output, "ClewdR Cookie Status") {
		t.Errorf("expected 'ClewdR Cookie Status' header, got: %s", output)
	}
	if !strings.Contains(output, "valid: 3") {
		t.Errorf("expected 'valid: 3' in output, got: %s", output)
	}
	if !strings.Contains(output, "exhausted: 1") {
		t.Errorf("expected 'exhausted: 1' in output, got: %s", output)
	}
	if !strings.Contains(output, "Summary:") {
		t.Errorf("expected 'Summary:' line, got: %s", output)
	}
	if !strings.Contains(output, "per instance") {
		t.Errorf("expected 'per instance' in summary, got: %s", output)
	}
}

func TestResolveClewdRTokens_PluralTakesPrecedence(t *testing.T) {
	t.Setenv("CLEWDR_ADMIN_TOKENS", "tok1,tok2,tok3")
	t.Setenv("CLEWDR_ADMIN_TOKEN", "single-tok")

	tokens := resolveClewdRTokens()
	if len(tokens) != 3 {
		t.Fatalf("expected 3 tokens, got %d", len(tokens))
	}
	if tokens[0] != "tok1" || tokens[1] != "tok2" || tokens[2] != "tok3" {
		t.Errorf("unexpected tokens: %v", tokens)
	}
}

func TestResolveClewdRTokens_FallbackToSingular(t *testing.T) {
	t.Setenv("CLEWDR_ADMIN_TOKENS", "")
	t.Setenv("CLEWDR_ADMIN_TOKEN", "single-tok")

	tokens := resolveClewdRTokens()
	if len(tokens) != 1 {
		t.Fatalf("expected 1 token, got %d", len(tokens))
	}
	if tokens[0] != "single-tok" {
		t.Errorf("expected 'single-tok', got %q", tokens[0])
	}
}

func TestResolveClewdRTokens_NoneSet(t *testing.T) {
	t.Setenv("CLEWDR_ADMIN_TOKENS", "")
	t.Setenv("CLEWDR_ADMIN_TOKEN", "")

	tokens := resolveClewdRTokens()
	if tokens != nil {
		t.Errorf("expected nil, got %v", tokens)
	}
}

func TestCookieStatusMinValid_SingularFallback(t *testing.T) {
	// Verify backward compatibility: CLEWDR_ADMIN_TOKEN (singular) still works
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/cookies" {
			// Verify the token was sent
			if r.Header.Get("Authorization") != "Bearer legacy-tok" {
				w.WriteHeader(401)
				return
			}
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(map[string]any{
				"valid":     []any{map[string]string{"cookie": "c1"}, map[string]string{"cookie": "c2"}, map[string]string{"cookie": "c3"}},
				"exhausted": []any{},
				"invalid":   []any{},
			})
			return
		}
		w.WriteHeader(200)
	}))
	defer srv.Close()

	t.Setenv("CLEWDR_URLS", srv.URL)
	t.Setenv("CLEWDR_ADMIN_TOKENS", "")
	t.Setenv("CLEWDR_ADMIN_TOKEN", "legacy-tok")

	rootCmd := buildRootCmd()
	buf := new(bytes.Buffer)
	rootCmd.SetOut(buf)
	rootCmd.SetErr(buf)
	rootCmd.SetArgs([]string{"cookie-status"})

	err := rootCmd.Execute()
	if err != nil {
		t.Fatalf("expected no error with singular token fallback, got: %v", err)
	}
}
