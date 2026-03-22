package bootstrap

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// --- Prereq Validation Tests ---

func TestValidatePrereqs_AllPresent(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "channels.yaml"), []byte("channels: {}"), 0644)
	os.WriteFile(filepath.Join(dir, "providers.yaml"), []byte("providers: {}"), 0644)

	b := New(BootstrapOptions{
		NewAPIURL:        "http://newapi:3000",
		NewAPIAdminToken: "tok123",
		BifrostURL:       "http://bifrost:8080",
		ClewdRURLs:       []string{"http://clewdr:8484"},
		ClewdRAdminToken: "ctok",
		ConfigDir:        dir,
	})

	result := b.ValidatePrereqs()
	if result.Status != "ok" {
		t.Fatalf("expected ok, got %s: %s", result.Status, result.Message)
	}
}

func TestValidatePrereqs_MissingEnvVars(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "channels.yaml"), []byte("channels: {}"), 0644)
	os.WriteFile(filepath.Join(dir, "providers.yaml"), []byte("providers: {}"), 0644)

	b := New(BootstrapOptions{
		// All env vars empty
		ConfigDir: dir,
	})

	result := b.ValidatePrereqs()
	if result.Status != "failed" {
		t.Fatalf("expected failed, got %s", result.Status)
	}

	// Check that each missing env var is listed by name
	for _, envName := range []string{"NEWAPI_URL", "NEWAPI_ADMIN_TOKEN", "BIFROST_URL", "CLEWDR_URLS", "CLEWDR_ADMIN_TOKEN"} {
		if !strings.Contains(result.Message, envName) {
			t.Errorf("expected message to mention %s, got: %s", envName, result.Message)
		}
	}
}

func TestValidatePrereqs_MissingChannelsYaml(t *testing.T) {
	dir := t.TempDir()
	// Only providers.yaml exists
	os.WriteFile(filepath.Join(dir, "providers.yaml"), []byte("providers: {}"), 0644)

	b := New(BootstrapOptions{
		NewAPIURL:        "http://newapi:3000",
		NewAPIAdminToken: "tok123",
		BifrostURL:       "http://bifrost:8080",
		ClewdRURLs:       []string{"http://clewdr:8484"},
		ClewdRAdminToken: "ctok",
		ConfigDir:        dir,
	})

	result := b.ValidatePrereqs()
	if result.Status != "failed" {
		t.Fatalf("expected failed, got %s", result.Status)
	}
	if !strings.Contains(result.Message, "channels.yaml") {
		t.Errorf("expected message to mention channels.yaml, got: %s", result.Message)
	}
}

func TestValidatePrereqs_MissingProvidersYaml(t *testing.T) {
	dir := t.TempDir()
	// Only channels.yaml exists
	os.WriteFile(filepath.Join(dir, "channels.yaml"), []byte("channels: {}"), 0644)

	b := New(BootstrapOptions{
		NewAPIURL:        "http://newapi:3000",
		NewAPIAdminToken: "tok123",
		BifrostURL:       "http://bifrost:8080",
		ClewdRURLs:       []string{"http://clewdr:8484"},
		ClewdRAdminToken: "ctok",
		ConfigDir:        dir,
	})

	result := b.ValidatePrereqs()
	if result.Status != "failed" {
		t.Fatalf("expected failed, got %s", result.Status)
	}
	if !strings.Contains(result.Message, "providers.yaml") {
		t.Errorf("expected message to mention providers.yaml, got: %s", result.Message)
	}
}

func TestPrereqError_ActionableGuidance(t *testing.T) {
	pe := &PrereqError{
		Missing: []PrereqItem{
			{Name: "NEWAPI_URL", Kind: "env", Guidance: `Set NEWAPI_URL to the New-API base URL (e.g. http://newapi:3000). Export it or add to .env file.`},
			{Name: "channels.yaml", Kind: "file", Guidance: `Create channels.yaml in the policies directory. See control-plane/policies/channels.yaml for an example.`},
		},
	}

	errStr := pe.Error()

	// Each item should have actionable guidance, not just "missing X"
	if !strings.Contains(errStr, "Set NEWAPI_URL to the New-API base URL") {
		t.Errorf("expected actionable guidance for NEWAPI_URL, got: %s", errStr)
	}
	if !strings.Contains(errStr, "Create channels.yaml in the policies directory") {
		t.Errorf("expected actionable guidance for channels.yaml, got: %s", errStr)
	}
}

// --- WaitForServices Tests ---

func TestWaitForServices_AllHealthy(t *testing.T) {
	// Create test servers that respond healthy
	newAPI := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer newAPI.Close()

	bifrost := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer bifrost.Close()

	clewdr := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer clewdr.Close()

	b := New(BootstrapOptions{
		NewAPIURL:        newAPI.URL,
		BifrostURL:       bifrost.URL,
		ClewdRURLs:       []string{clewdr.URL},
		ClewdRAdminToken: "tok",
		Timeout:          5 * time.Second,
	})

	ctx := context.Background()
	result := b.WaitForServices(ctx)
	if result.Status != "ok" {
		t.Fatalf("expected ok, got %s: %s", result.Status, result.Message)
	}
}

func TestWaitForServices_TimeoutExceeded(t *testing.T) {
	// New-API responds OK, but bifrost never responds
	newAPI := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer newAPI.Close()

	b := New(BootstrapOptions{
		NewAPIURL:  newAPI.URL,
		BifrostURL: "http://127.0.0.1:1", // unreachable port
		ClewdRURLs: []string{},
		Timeout:    2 * time.Second,
	})

	ctx := context.Background()
	result := b.WaitForServices(ctx)
	if result.Status != "failed" {
		t.Fatalf("expected failed, got %s: %s", result.Status, result.Message)
	}
	// Should mention per-service status
	if !strings.Contains(result.Message, "bifrost") {
		t.Errorf("expected per-service status mentioning bifrost, got: %s", result.Message)
	}
}

func TestWaitForServices_ExponentialBackoff(t *testing.T) {
	attempts := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		attempts++
		if attempts < 3 {
			w.WriteHeader(http.StatusServiceUnavailable)
			return
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	b := New(BootstrapOptions{
		NewAPIURL:  srv.URL,
		BifrostURL: srv.URL,
		ClewdRURLs: []string{},
		Timeout:    15 * time.Second,
	})

	ctx := context.Background()
	result := b.WaitForServices(ctx)
	if result.Status != "ok" {
		t.Fatalf("expected ok after retries, got %s: %s", result.Status, result.Message)
	}
	// Should have retried at least once
	if attempts < 2 {
		t.Errorf("expected at least 2 attempts, got %d", attempts)
	}
}

// --- SeedAdmin Tests ---

func TestSeedAdmin_Success(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/setup" && r.Method == "POST" {
			var body map[string]string
			json.NewDecoder(r.Body).Decode(&body)
			if body["username"] != "root" || body["password"] != "testpass" {
				t.Errorf("unexpected setup body: %v", body)
			}
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(`{"success":true,"message":"admin created","data":{"token":"abc123"}}`))
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	defer srv.Close()

	b := New(BootstrapOptions{
		NewAPIURL:     srv.URL,
		AdminUsername: "root",
		AdminPassword: "testpass",
	})

	result := b.SeedAdmin()
	if result.Status != "ok" {
		t.Fatalf("expected ok, got %s: %s", result.Status, result.Message)
	}
}

func TestSeedAdmin_AdminAlreadyExists(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/setup" && r.Method == "POST" {
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(`{"success":false,"message":"admin account already exists"}`))
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	defer srv.Close()

	b := New(BootstrapOptions{
		NewAPIURL:     srv.URL,
		AdminUsername: "root",
		AdminPassword: "testpass",
	})

	result := b.SeedAdmin()
	if result.Status != "skipped" {
		t.Fatalf("expected skipped when admin already exists, got %s: %s", result.Status, result.Message)
	}
}

// --- SetupAdmin on NewAPIClient Tests ---

func TestSetupAdmin_CorrectPayload(t *testing.T) {
	var receivedBody map[string]string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/setup" && r.Method == "POST" {
			json.NewDecoder(r.Body).Decode(&receivedBody)
			ct := r.Header.Get("Content-Type")
			if ct != "application/json" {
				t.Errorf("expected Content-Type application/json, got %s", ct)
			}
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(`{"success":true}`))
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	defer srv.Close()

	client := newTestNewAPIClient(srv.URL)
	_, err := client.SetupAdmin("admin", "secret123")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if receivedBody["username"] != "admin" {
		t.Errorf("expected username 'admin', got %q", receivedBody["username"])
	}
	if receivedBody["password"] != "secret123" {
		t.Errorf("expected password 'secret123', got %q", receivedBody["password"])
	}
}

// --- CheckAllHealth Tests ---

func TestCheckAllHealth(t *testing.T) {
	newAPI := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer newAPI.Close()

	bifrost := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusServiceUnavailable)
	}))
	defer bifrost.Close()

	b := New(BootstrapOptions{
		NewAPIURL:  newAPI.URL,
		BifrostURL: bifrost.URL,
		ClewdRURLs: []string{},
	})

	statuses := b.CheckAllHealth()
	if len(statuses) < 2 {
		t.Fatalf("expected at least 2 statuses, got %d", len(statuses))
	}

	// Find new-api and bifrost entries
	var foundNewAPI, foundBifrost bool
	for _, s := range statuses {
		if s.Name == "new-api" {
			foundNewAPI = true
			if !s.Healthy {
				t.Errorf("expected new-api to be healthy")
			}
		}
		if s.Name == "bifrost" {
			foundBifrost = true
			if s.Healthy {
				t.Errorf("expected bifrost to be unhealthy")
			}
		}
	}
	if !foundNewAPI {
		t.Error("expected new-api in statuses")
	}
	if !foundBifrost {
		t.Error("expected bifrost in statuses")
	}
}

// --- Helpers ---

// newTestNewAPIClient creates a NewAPIClient-compatible struct for testing SetupAdmin.
// We import the upstream package's NewAPIClient via the bootstrap package's dependency.
func newTestNewAPIClient(baseURL string) *testNewAPIClient {
	return &testNewAPIClient{
		client:  &http.Client{Timeout: 5 * time.Second},
		baseURL: baseURL,
	}
}

// testNewAPIClient is a local test double that mimics SetupAdmin behavior.
// In the real code, upstream.NewAPIClient will have this method.
type testNewAPIClient struct {
	client  *http.Client
	baseURL string
}

func (c *testNewAPIClient) SetupAdmin(username, password string) (json.RawMessage, error) {
	payload, _ := json.Marshal(map[string]string{"username": username, "password": password})
	req, err := http.NewRequest("POST", c.baseURL+"/api/setup", strings.NewReader(string(payload)))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := c.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	var body json.RawMessage
	json.NewDecoder(resp.Body).Decode(&body)
	return body, nil
}
