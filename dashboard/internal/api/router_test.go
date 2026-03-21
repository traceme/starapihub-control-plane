package api

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/starapihub/dashboard/internal/poller"
	"github.com/starapihub/dashboard/internal/store"
	"github.com/starapihub/dashboard/internal/upstream"
)

const testToken = "test-secret-token"

func newTestHandler(t *testing.T) *Handler {
	t.Helper()
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "test.db")
	st, err := store.New(dbPath)
	if err != nil {
		t.Fatalf("store.New: %v", err)
	}
	t.Cleanup(func() { st.Close() })

	state := poller.NewSystemState()
	httpClient := upstream.NewHTTPClient()

	return NewHandler(
		state, st,
		upstream.NewNewAPIClient(httpClient, "http://localhost:3000"),
		upstream.NewBifrostClient(httpClient, "http://localhost:8080"),
		upstream.NewClewdRClient(httpClient),
		nil, nil, testToken,
	)
}

func TestAuthMiddleware_ValidToken(t *testing.T) {
	called := false
	inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		w.WriteHeader(http.StatusOK)
	})

	handler := authMiddleware(testToken, inner)

	req := httptest.NewRequest("GET", "/api/health", nil)
	req.Header.Set("Authorization", "Bearer "+testToken)
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", rr.Code)
	}
	if !called {
		t.Error("expected inner handler to be called")
	}
}

func TestAuthMiddleware_MissingHeader(t *testing.T) {
	inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Error("inner handler should not be called")
	})

	handler := authMiddleware(testToken, inner)

	req := httptest.NewRequest("GET", "/api/health", nil)
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d", rr.Code)
	}
}

func TestAuthMiddleware_InvalidFormat(t *testing.T) {
	inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Error("inner handler should not be called")
	})

	handler := authMiddleware(testToken, inner)

	req := httptest.NewRequest("GET", "/api/health", nil)
	req.Header.Set("Authorization", "Basic dXNlcjpwYXNz")
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d", rr.Code)
	}
}

func TestAuthMiddleware_WrongToken(t *testing.T) {
	inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Error("inner handler should not be called")
	})

	handler := authMiddleware(testToken, inner)

	req := httptest.NewRequest("GET", "/api/health", nil)
	req.Header.Set("Authorization", "Bearer wrong-token")
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusForbidden {
		t.Errorf("expected 403, got %d", rr.Code)
	}
}

func TestAuthMiddleware_TimingSafe(t *testing.T) {
	// Verify the middleware uses constant-time comparison by checking
	// that tokens of different lengths both result in 403 (not timing difference).
	// This is a behavioral test - the actual timing safety comes from crypto/subtle.
	inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Error("inner handler should not be called")
	})
	handler := authMiddleware(testToken, inner)

	tokens := []string{"x", "wrong-token", "test-secret-token-extra-long-string"}
	for _, tok := range tokens {
		req := httptest.NewRequest("GET", "/api/health", nil)
		req.Header.Set("Authorization", "Bearer "+tok)
		rr := httptest.NewRecorder()
		handler.ServeHTTP(rr, req)
		if rr.Code != http.StatusForbidden {
			t.Errorf("token %q: expected 403, got %d", tok, rr.Code)
		}
	}
}

func TestHandleHealth(t *testing.T) {
	h := newTestHandler(t)

	// Set some state
	h.state.SetHealth("new-api", poller.ServiceHealth{
		Status:    "healthy",
		URL:       "http://localhost:3000",
		LastCheck: time.Now(),
		Latency:   15,
	})

	router := NewRouter(h, testToken, nil)

	req := httptest.NewRequest("GET", "/api/health", nil)
	req.Header.Set("Authorization", "Bearer "+testToken)
	rr := httptest.NewRecorder()

	router.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rr.Code)
	}

	var resp map[string]interface{}
	if err := json.NewDecoder(rr.Body).Decode(&resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	services, ok := resp["services"].(map[string]interface{})
	if !ok {
		t.Fatal("expected services in response")
	}
	if _, ok := services["new-api"]; !ok {
		t.Error("expected new-api in services")
	}
}

func TestHandleSSE_InitialEvent(t *testing.T) {
	h := newTestHandler(t)
	h.state.SetHealth("svc", poller.ServiceHealth{Status: "healthy"})

	// Use a context that is already cancelled so the SSE loop exits immediately
	// after sending the initial snapshot.
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // cancel immediately

	req := httptest.NewRequest("GET", "/api/sse", nil)
	req = req.WithContext(ctx)

	rr := httptest.NewRecorder()
	h.HandleSSE(rr, req)

	body := rr.Body.String()
	if len(body) == 0 {
		t.Error("expected SSE data in response body")
	}
	// Should start with "data: "
	if len(body) >= 6 && body[:6] != "data: " {
		end := 20
		if len(body) < end {
			end = len(body)
		}
		t.Errorf("expected SSE format, got prefix %q", body[:end])
	}
	// Verify it contains valid JSON after "data: "
	if len(body) > 6 {
		// Find the end of the first data line
		dataEnd := strings.Index(body, "\n\n")
		if dataEnd > 6 {
			jsonStr := body[6:dataEnd]
			var snap poller.Snapshot
			if err := json.Unmarshal([]byte(jsonStr), &snap); err != nil {
				t.Errorf("failed to parse SSE JSON: %v", err)
			}
			if _, ok := snap.Health["svc"]; !ok {
				t.Error("expected svc in snapshot health")
			}
		}
	}
}

func TestNewRouter_NoFrontend(t *testing.T) {
	h := newTestHandler(t)
	router := NewRouter(h, testToken, nil)

	req := httptest.NewRequest("GET", "/", nil)
	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, req)

	if rr.Code != http.StatusNotFound {
		t.Errorf("expected 404 without frontend, got %d", rr.Code)
	}
}
