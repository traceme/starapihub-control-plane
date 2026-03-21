package poller

import (
	"encoding/json"
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
		Service:   "test-svc",
		Message:   "test alert",
		Timestamp: time.Now(),
	}

	// Fire first alert
	fireAlert(cfg, alert)

	// Fire duplicate - should be suppressed
	fireAlert(cfg, alert)

	alerts, err := st.ListAlerts(10)
	if err != nil {
		t.Fatalf("ListAlerts: %v", err)
	}
	if len(alerts) != 1 {
		t.Errorf("expected 1 alert (dedup), got %d", len(alerts))
	}
}

func TestFireAlert_DifferentTypes(t *testing.T) {
	st := newTestStoreForPoller(t)
	state := NewSystemState()

	cfg := Config{
		Store: st,
		State: state,
	}

	fireAlert(cfg, store.Alert{Type: "WARNING", Service: "svc", Message: "warn", Timestamp: time.Now()})
	fireAlert(cfg, store.Alert{Type: "CRITICAL", Service: "svc", Message: "crit", Timestamp: time.Now()})

	alerts, _ := st.ListAlerts(10)
	if len(alerts) != 2 {
		t.Errorf("expected 2 alerts for different types, got %d", len(alerts))
	}
}

func TestFireAlert_Webhook(t *testing.T) {
	st := newTestStoreForPoller(t)
	state := NewSystemState()

	var received bool
	var mu sync.Mutex
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		defer mu.Unlock()
		received = true
		var alert store.Alert
		json.NewDecoder(r.Body).Decode(&alert)
		if alert.Type != "CRITICAL" {
			t.Errorf("webhook got type %s, want CRITICAL", alert.Type)
		}
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
		Service:   "svc",
		Message:   "webhook test",
		Timestamp: time.Now(),
	})

	// Wait a bit for the async webhook goroutine
	time.Sleep(100 * time.Millisecond)

	mu.Lock()
	defer mu.Unlock()
	if !received {
		t.Error("expected webhook to be called")
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
	if alerts[0].Type != "CRITICAL" {
		t.Errorf("expected CRITICAL, got %s", alerts[0].Type)
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
	if alerts[0].Type != "WARNING" {
		t.Errorf("expected WARNING, got %s", alerts[0].Type)
	}
}

func TestCheckAlerts_UnhealthyService(t *testing.T) {
	st := newTestStoreForPoller(t)
	state := NewSystemState()

	// Set service unhealthy and backdate unhealthySince
	state.SetHealth("new-api", ServiceHealth{Status: "unhealthy"})
	state.mu.Lock()
	state.unhealthySince["new-api"] = time.Now().Add(-60 * time.Second) // 60s ago
	state.mu.Unlock()

	cfg := Config{Store: st, State: state}
	checkAlerts(cfg)

	alerts, _ := st.ListAlerts(10)
	if len(alerts) != 1 {
		t.Fatalf("expected 1 alert for unhealthy service, got %d", len(alerts))
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
