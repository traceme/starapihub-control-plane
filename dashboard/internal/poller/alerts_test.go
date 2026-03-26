package poller

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/starapihub/dashboard/internal/store"
)

func newTestStoreForPoller(t *testing.T) *store.Store {
	t.Helper()
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "test.db")
	s, err := store.New(dbPath)
	if err != nil {
		t.Fatalf("store.New: %v", err)
	}
	t.Cleanup(func() { s.Close() })
	return s
}

func TestFireAlert_Dedup(t *testing.T) {
	st := newTestStoreForPoller(t)
	state := NewSystemState()

	cfg := Config{
		Store: st,
		State: state,
	}

	alert := store.Alert{
		Type:      "WARNING",
		Severity:  "WARNING",
		Signal:    "config-drift",
		Service:   "test-svc",
		Message:   "test alert",
		Timestamp: time.Now(),
	}

	// Fire first alert
	fireAlert(cfg, alert)

	// Fire duplicate - should be suppressed (same severity+service)
	fireAlert(cfg, alert)

	alerts, err := st.ListAlerts(10)
	if err != nil {
		t.Fatalf("ListAlerts: %v", err)
	}
	if len(alerts) != 1 {
		t.Errorf("expected 1 alert (dedup), got %d", len(alerts))
	}
}

func TestFireAlert_DifferentSeverities(t *testing.T) {
	st := newTestStoreForPoller(t)
	state := NewSystemState()

	cfg := Config{
		Store: st,
		State: state,
	}

	fireAlert(cfg, store.Alert{Type: "WARNING", Severity: "WARNING", Signal: "config-drift", Service: "svc", Message: "warn", Timestamp: time.Now()})
	fireAlert(cfg, store.Alert{Type: "CRITICAL", Severity: "CRITICAL", Signal: "service-down", Service: "svc", Message: "crit", Timestamp: time.Now()})

	alerts, _ := st.ListAlerts(10)
	if len(alerts) != 2 {
		t.Errorf("expected 2 alerts for different severities, got %d", len(alerts))
	}
}

func TestFireAlert_Webhook(t *testing.T) {
	st := newTestStoreForPoller(t)
	state := NewSystemState()

	var received bool
	var mu sync.Mutex
	var receivedAlert store.Alert
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		defer mu.Unlock()
		received = true
		json.NewDecoder(r.Body).Decode(&receivedAlert)
		w.WriteHeader(http.StatusOK)
	}))
	defer ts.Close()

	cfg := Config{
		Store:           st,
		State:           state,
		AlertWebhookURL: ts.URL,
	}

	fireAlert(cfg, store.Alert{
		Type:      "CRITICAL",
		Severity:  "CRITICAL",
		Signal:    "service-down",
		Service:   "new-api",
		Message:   "webhook test",
		Timestamp: time.Now(),
	})

	// Wait a bit for the async webhook goroutine
	time.Sleep(100 * time.Millisecond)

	mu.Lock()
	defer mu.Unlock()
	if !received {
		t.Fatal("expected webhook to be called")
	}
	if receivedAlert.Severity != "CRITICAL" {
		t.Errorf("webhook got severity %s, want CRITICAL", receivedAlert.Severity)
	}
	if receivedAlert.Signal != "service-down" {
		t.Errorf("webhook got signal %s, want service-down", receivedAlert.Signal)
	}
	if receivedAlert.Type != "CRITICAL" {
		t.Errorf("webhook got type %s, want CRITICAL (backward compat)", receivedAlert.Type)
	}
}

func TestCheckAlerts_NoCookies(t *testing.T) {
	st := newTestStoreForPoller(t)
	state := NewSystemState()
	cfg := Config{Store: st, State: state}

	// No cookies set, no health set - should not panic
	checkAlerts(cfg)

	alerts, _ := st.ListAlerts(10)
	if len(alerts) != 0 {
		t.Errorf("expected 0 alerts, got %d", len(alerts))
	}
}

func TestCheckAlerts_ZeroValidCookies(t *testing.T) {
	st := newTestStoreForPoller(t)
	state := NewSystemState()

	state.SetCookies("clewdr-1", CookieStatus{Valid: 0, Exhausted: 3, Total: 3})

	cfg := Config{Store: st, State: state}
	checkAlerts(cfg)

	alerts, _ := st.ListAlerts(10)
	if len(alerts) != 1 {
		t.Fatalf("expected 1 CRITICAL alert, got %d", len(alerts))
	}
	if alerts[0].Severity != "CRITICAL" {
		t.Errorf("expected severity CRITICAL, got %s", alerts[0].Severity)
	}
	if alerts[0].Signal != "cookie-exhaustion" {
		t.Errorf("expected signal cookie-exhaustion, got %s", alerts[0].Signal)
	}
}

func TestCheckAlerts_HighUtilization(t *testing.T) {
	st := newTestStoreForPoller(t)
	state := NewSystemState()

	state.SetCookies("clewdr-1", CookieStatus{Valid: 5, HighUtil: 3, Total: 5})

	cfg := Config{Store: st, State: state}
	checkAlerts(cfg)

	alerts, _ := st.ListAlerts(10)
	if len(alerts) != 1 {
		t.Fatalf("expected 1 WARNING alert, got %d", len(alerts))
	}
	if alerts[0].Severity != "WARNING" {
		t.Errorf("expected severity WARNING, got %s", alerts[0].Severity)
	}
	if alerts[0].Signal != "cookie-exhaustion" {
		t.Errorf("expected signal cookie-exhaustion, got %s", alerts[0].Signal)
	}
}

func TestCheckAlerts_UnhealthyService_Critical(t *testing.T) {
	st := newTestStoreForPoller(t)
	state := NewSystemState()

	// new-api is a critical service — should fire CRITICAL
	state.SetHealth("new-api", ServiceHealth{Status: "unhealthy"})
	state.mu.Lock()
	state.unhealthySince["new-api"] = time.Now().Add(-60 * time.Second)
	state.mu.Unlock()

	cfg := Config{Store: st, State: state}
	checkAlerts(cfg)

	alerts, _ := st.ListAlerts(10)
	if len(alerts) != 1 {
		t.Fatalf("expected 1 alert for unhealthy new-api, got %d", len(alerts))
	}
	if alerts[0].Severity != "CRITICAL" {
		t.Errorf("expected severity CRITICAL for new-api, got %s", alerts[0].Severity)
	}
	if alerts[0].Signal != "service-down" {
		t.Errorf("expected signal service-down, got %s", alerts[0].Signal)
	}
}

func TestCheckAlerts_UnhealthyClewdR_Info(t *testing.T) {
	st := newTestStoreForPoller(t)
	state := NewSystemState()

	// A single ClewdR instance is INFO (Bifrost routes around it)
	state.SetHealth("clewdr-1", ServiceHealth{Status: "unhealthy"})
	state.mu.Lock()
	state.unhealthySince["clewdr-1"] = time.Now().Add(-60 * time.Second)
	state.mu.Unlock()

	cfg := Config{Store: st, State: state}
	checkAlerts(cfg)

	alerts, _ := st.ListAlerts(10)
	if len(alerts) != 1 {
		t.Fatalf("expected 1 alert for unhealthy clewdr-1, got %d", len(alerts))
	}
	if alerts[0].Severity != "INFO" {
		t.Errorf("expected severity INFO for single clewdr instance, got %s", alerts[0].Severity)
	}
}

func TestCheckAlerts_UnhealthyServiceTooRecent(t *testing.T) {
	st := newTestStoreForPoller(t)
	state := NewSystemState()

	// Set unhealthy just now (< 30s threshold)
	state.SetHealth("new-api", ServiceHealth{Status: "unhealthy"})

	cfg := Config{Store: st, State: state}
	checkAlerts(cfg)

	alerts, _ := st.ListAlerts(10)
	if len(alerts) != 0 {
		t.Errorf("expected 0 alerts for recently unhealthy service, got %d", len(alerts))
	}
}

func TestFireAlert_UpdatesState(t *testing.T) {
	st := newTestStoreForPoller(t)
	state := NewSystemState()

	cfg := Config{
		Store: st,
		State: state,
	}

	fireAlert(cfg, store.Alert{
		Type:      "WARNING",
		Severity:  "WARNING",
		Signal:    "config-drift",
		Service:   "test-svc",
		Message:   "state update test",
		Timestamp: time.Now(),
	})

	snap := state.GetSnapshot()
	if len(snap.Alerts) != 1 {
		t.Fatalf("expected 1 alert in state, got %d", len(snap.Alerts))
	}
	if snap.Alerts[0].Severity != "WARNING" {
		t.Errorf("expected WARNING, got %s", snap.Alerts[0].Severity)
	}
	if snap.Alerts[0].Signal != "config-drift" {
		t.Errorf("expected config-drift, got %s", snap.Alerts[0].Signal)
	}
	if snap.Alerts[0].Service != "test-svc" {
		t.Errorf("expected test-svc, got %s", snap.Alerts[0].Service)
	}
	if snap.Alerts[0].Message != "state update test" {
		t.Errorf("expected 'state update test', got %s", snap.Alerts[0].Message)
	}
	if snap.Alerts[0].ID == 0 {
		t.Error("expected alert ID to be set from store")
	}
}

func TestAppendAlert_Cap(t *testing.T) {
	state := NewSystemState()

	for i := 0; i < 110; i++ {
		state.AppendAlert(store.Alert{
			ID:       int64(i + 1),
			Type:     "INFO",
			Severity: "INFO",
			Signal:   "test",
			Service:  "svc",
			Message:  fmt.Sprintf("alert %d", i+1),
		})
	}

	snap := state.GetSnapshot()
	if len(snap.Alerts) != 100 {
		t.Fatalf("expected 100 alerts (capped), got %d", len(snap.Alerts))
	}
	// First alert should be #11 (first 10 dropped)
	if snap.Alerts[0].ID != 11 {
		t.Errorf("expected first alert ID 11, got %d", snap.Alerts[0].ID)
	}
	if snap.Alerts[99].ID != 110 {
		t.Errorf("expected last alert ID 110, got %d", snap.Alerts[99].ID)
	}
}

func TestServiceIsCritical(t *testing.T) {
	critical := []string{"new-api", "bifrost", "nginx", "postgres"}
	for _, name := range critical {
		if !serviceIsCritical(name) {
			t.Errorf("expected %s to be critical", name)
		}
	}
	notCritical := []string{"clewdr-1", "redis", "unknown"}
	for _, name := range notCritical {
		if serviceIsCritical(name) {
			t.Errorf("expected %s to NOT be critical", name)
		}
	}
}
