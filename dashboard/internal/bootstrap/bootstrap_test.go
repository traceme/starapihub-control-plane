package bootstrap

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	syncpkg "github.com/starapihub/dashboard/internal/sync"
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

// --- Run Orchestration Tests ---

// stubReconciler implements sync.Reconciler for testing.
type stubReconciler struct {
	name       string
	planFunc   func(desired, live any) ([]syncpkg.Action, error)
	applyFunc  func(action syncpkg.Action) (*syncpkg.Result, error)
	verifyFunc func(action syncpkg.Action, result *syncpkg.Result) error
}

func (s *stubReconciler) Name() string { return s.name }
func (s *stubReconciler) Plan(desired, live any) ([]syncpkg.Action, error) {
	if s.planFunc != nil {
		return s.planFunc(desired, live)
	}
	return nil, nil
}
func (s *stubReconciler) Apply(action syncpkg.Action) (*syncpkg.Result, error) {
	if s.applyFunc != nil {
		return s.applyFunc(action)
	}
	return &syncpkg.Result{Action: action, Status: syncpkg.StatusOK}, nil
}
func (s *stubReconciler) Verify(action syncpkg.Action, result *syncpkg.Result) error {
	if s.verifyFunc != nil {
		return s.verifyFunc(action, result)
	}
	return nil
}

func newPassingReconciler(name string, actionCount int) *stubReconciler {
	return &stubReconciler{
		name: name,
		planFunc: func(desired, live any) ([]syncpkg.Action, error) {
			var actions []syncpkg.Action
			for i := 0; i < actionCount; i++ {
				actions = append(actions, syncpkg.Action{
					Type:         syncpkg.ActionCreate,
					ResourceType: name,
					ResourceID:   fmt.Sprintf("%s-%d", name, i),
				})
			}
			return actions, nil
		},
	}
}

func newFailingReconciler(name string) *stubReconciler {
	return &stubReconciler{
		name: name,
		planFunc: func(desired, live any) ([]syncpkg.Action, error) {
			return nil, fmt.Errorf("plan failed for %s", name)
		},
	}
}

func makeBootstrapperWithHealthyServices(t *testing.T) (*Bootstrapper, func()) {
	t.Helper()
	newAPI := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/setup" && r.Method == "POST" {
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(`{"success":true,"message":"admin created"}`))
			return
		}
		w.WriteHeader(http.StatusOK)
	}))
	bifrost := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	clewdr := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "channels.yaml"), []byte("channels: {}"), 0644)
	os.WriteFile(filepath.Join(dir, "providers.yaml"), []byte("providers: {}"), 0644)

	b := New(BootstrapOptions{
		NewAPIURL:        newAPI.URL,
		NewAPIAdminToken: "tok",
		BifrostURL:       bifrost.URL,
		ClewdRURLs:       []string{clewdr.URL},
		ClewdRAdminToken: "ctok",
		ConfigDir:        dir,
		AdminUsername:    "root",
		AdminPassword:    "testpass",
		Timeout:          5 * time.Second,
	})

	cleanup := func() {
		newAPI.Close()
		bifrost.Close()
		clewdr.Close()
	}
	return b, cleanup
}

func TestRunCallsAllStepsInOrder(t *testing.T) {
	b, cleanup := makeBootstrapperWithHealthyServices(t)
	defer cleanup()

	// Set up sync deps with a passing reconciler
	rec := newPassingReconciler("provider", 2)
	b.SetSyncDeps(SyncDeps{
		Reconcilers:  []syncpkg.Reconciler{rec},
		DesiredState: map[string]any{"provider": nil},
		LiveState:    map[string]any{"provider": nil},
	})

	ctx := context.Background()
	report := b.Run(ctx)

	if !report.Success {
		t.Fatalf("expected success, got failure. Steps: %+v", report.Steps)
	}

	// Should have 5 steps: validate-prereqs, wait-services, seed-admin, run-sync, verify-health
	if len(report.Steps) != 5 {
		t.Fatalf("expected 5 steps, got %d: %+v", len(report.Steps), report.Steps)
	}

	expectedNames := []string{"validate-prereqs", "wait-services", "seed-admin", "run-sync", "verify-health"}
	for i, name := range expectedNames {
		if report.Steps[i].Name != name {
			t.Errorf("step %d: expected name %q, got %q", i, name, report.Steps[i].Name)
		}
	}
}

func TestRunStopsOnFailedStep(t *testing.T) {
	// Create a bootstrapper with missing prereqs to trigger early failure
	b := New(BootstrapOptions{
		// Missing env vars -- validate-prereqs will fail
		ConfigDir: t.TempDir(),
	})

	ctx := context.Background()
	report := b.Run(ctx)

	if report.Success {
		t.Fatal("expected failure when prereqs missing")
	}
	if len(report.Steps) != 1 {
		t.Fatalf("expected 1 step (stopped at prereqs), got %d", len(report.Steps))
	}
	if report.Steps[0].Status != "failed" {
		t.Errorf("expected first step to be failed, got %s", report.Steps[0].Status)
	}
}

func TestRunSkipSeed(t *testing.T) {
	b, cleanup := makeBootstrapperWithHealthyServices(t)
	defer cleanup()
	b.opts.SkipSeed = true

	b.SetSyncDeps(SyncDeps{
		Reconcilers:  []syncpkg.Reconciler{newPassingReconciler("provider", 1)},
		DesiredState: map[string]any{"provider": nil},
		LiveState:    map[string]any{"provider": nil},
	})

	ctx := context.Background()
	report := b.Run(ctx)

	if !report.Success {
		t.Fatalf("expected success, got failure: %+v", report.Steps)
	}

	// Find seed-admin step
	for _, step := range report.Steps {
		if step.Name == "seed-admin" {
			if step.Status != "skipped" {
				t.Errorf("expected seed-admin to be skipped, got %s: %s", step.Status, step.Message)
			}
			if !strings.Contains(step.Message, "skip-seed") {
				t.Errorf("expected skip-seed message, got: %s", step.Message)
			}
			return
		}
	}
	t.Error("seed-admin step not found in report")
}

func TestRunSkipSync(t *testing.T) {
	b, cleanup := makeBootstrapperWithHealthyServices(t)
	defer cleanup()
	b.opts.SkipSync = true

	ctx := context.Background()
	report := b.Run(ctx)

	if !report.Success {
		t.Fatalf("expected success, got failure: %+v", report.Steps)
	}

	// Both run-sync and verify-health should be skipped
	syncFound, verifyFound := false, false
	for _, step := range report.Steps {
		if step.Name == "run-sync" {
			syncFound = true
			if step.Status != "skipped" {
				t.Errorf("expected run-sync to be skipped, got %s", step.Status)
			}
		}
		if step.Name == "verify-health" {
			verifyFound = true
			if step.Status != "skipped" {
				t.Errorf("expected verify-health to be skipped, got %s", step.Status)
			}
		}
	}
	if !syncFound {
		t.Error("run-sync step not found")
	}
	if !verifyFound {
		t.Error("verify-health step not found")
	}
}

func TestRunDryRun(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "channels.yaml"), []byte("channels: {}"), 0644)
	os.WriteFile(filepath.Join(dir, "providers.yaml"), []byte("providers: {}"), 0644)

	b := New(BootstrapOptions{
		NewAPIURL:        "http://dummy:3000",
		NewAPIAdminToken: "tok",
		BifrostURL:       "http://dummy:8080",
		ClewdRURLs:       []string{"http://dummy:8484"},
		ClewdRAdminToken: "ctok",
		ConfigDir:        dir,
		DryRun:           true,
	})

	ctx := context.Background()
	report := b.Run(ctx)

	if !report.Success {
		t.Fatalf("expected dry-run success, got failure: %+v", report.Steps)
	}

	// wait-services, seed-admin, verify-health should all be skipped in dry-run
	for _, step := range report.Steps {
		if step.Name == "wait-services" || step.Name == "seed-admin" || step.Name == "verify-health" {
			if step.Status != "skipped" {
				t.Errorf("expected %s to be skipped in dry-run, got %s", step.Name, step.Status)
			}
		}
	}
}

func TestBootstrapReportSuccess(t *testing.T) {
	report := &BootstrapReport{Success: true}
	report.AddStep(&StepResult{Name: "step1", Status: "ok"})
	report.AddStep(&StepResult{Name: "step2", Status: "ok"})

	if !report.Success {
		t.Error("expected success when all steps ok")
	}
	if len(report.Steps) != 2 {
		t.Errorf("expected 2 steps, got %d", len(report.Steps))
	}
}

func TestBootstrapReportFailure(t *testing.T) {
	report := &BootstrapReport{Success: true}
	report.AddStep(&StepResult{Name: "step1", Status: "ok"})
	report.AddStep(&StepResult{Name: "step2", Status: "failed"})

	if report.Success {
		t.Error("expected failure when a step failed")
	}
}

func TestRunSyncSuccess(t *testing.T) {
	b, cleanup := makeBootstrapperWithHealthyServices(t)
	defer cleanup()

	rec := newPassingReconciler("provider", 3)
	b.SetSyncDeps(SyncDeps{
		Reconcilers:  []syncpkg.Reconciler{rec},
		DesiredState: map[string]any{"provider": nil},
		LiveState:    map[string]any{"provider": nil},
	})

	result := b.RunSync()
	if result.Status != "ok" {
		t.Fatalf("expected ok, got %s: %s", result.Status, result.Message)
	}
	if result.Name != "run-sync" {
		t.Errorf("expected name run-sync, got %s", result.Name)
	}
	if !strings.Contains(result.Message, "synced") {
		t.Errorf("expected message to contain resource count, got: %s", result.Message)
	}
}

func TestRunSyncNoDeps(t *testing.T) {
	b := New(BootstrapOptions{})
	result := b.RunSync()
	if result.Status != "failed" {
		t.Fatalf("expected failed when no sync deps, got %s", result.Status)
	}
}

func TestVerifyHealthAllHealthy(t *testing.T) {
	b, cleanup := makeBootstrapperWithHealthyServices(t)
	defer cleanup()

	// No sync deps -- just checks service health
	result := b.VerifyHealth()
	if result.Status != "ok" {
		t.Fatalf("expected ok, got %s: %s", result.Status, result.Message)
	}
	if !strings.Contains(result.Message, "healthy") {
		t.Errorf("expected message to mention healthy, got: %s", result.Message)
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
