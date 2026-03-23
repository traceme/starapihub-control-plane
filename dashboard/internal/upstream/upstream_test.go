package upstream

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestNewHTTPClient(t *testing.T) {
	c := NewHTTPClient()
	if c == nil {
		t.Fatal("expected non-nil client")
	}
	if c.Timeout == 0 {
		t.Error("expected non-zero timeout")
	}
	tr, ok := c.Transport.(*http.Transport)
	if !ok {
		t.Fatal("expected *http.Transport")
	}
	if tr.MaxIdleConns != 50 {
		t.Errorf("expected MaxIdleConns 50, got %d", tr.MaxIdleConns)
	}
	if tr.MaxIdleConnsPerHost != 10 {
		t.Errorf("expected MaxIdleConnsPerHost 10, got %d", tr.MaxIdleConnsPerHost)
	}
}

// --- NewAPIClient tests ---

func TestNewNewAPIClient(t *testing.T) {
	c := NewNewAPIClient(NewHTTPClient(), "http://localhost:3000")
	if c == nil {
		t.Fatal("expected non-nil client")
	}
	if c.BaseURL() != "http://localhost:3000" {
		t.Errorf("expected base URL http://localhost:3000, got %s", c.BaseURL())
	}
}

func TestNewAPIClient_CheckHealth(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/status" {
			t.Errorf("expected path /api/status, got %s", r.URL.Path)
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer ts.Close()

	c := NewNewAPIClient(ts.Client(), ts.URL)
	ok, err := c.CheckHealth()
	if err != nil {
		t.Fatalf("CheckHealth: %v", err)
	}
	if !ok {
		t.Error("expected healthy")
	}
}

func TestNewAPIClient_CheckHealth_Unhealthy(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusServiceUnavailable)
	}))
	defer ts.Close()

	c := NewNewAPIClient(ts.Client(), ts.URL)
	ok, err := c.CheckHealth()
	if err != nil {
		t.Fatalf("CheckHealth: %v", err)
	}
	if ok {
		t.Error("expected unhealthy")
	}
}

func TestNewAPIClient_ListChannels(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "GET" {
			t.Errorf("expected GET, got %s", r.Method)
		}
		if r.Header.Get("Authorization") != "admin-token" {
			t.Errorf("missing or wrong Authorization header: got %q", r.Header.Get("Authorization"))
		}
		if r.Header.Get("New-Api-User") != "1" {
			t.Errorf("missing or wrong New-Api-User header: got %q", r.Header.Get("New-Api-User"))
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"data":[]}`))
	}))
	defer ts.Close()

	c := NewNewAPIClient(ts.Client(), ts.URL)
	c.SetAdminUserID("1")
	body, err := c.ListChannels("admin-token")
	if err != nil {
		t.Fatalf("ListChannels: %v", err)
	}
	if string(body) != `{"data":[]}` {
		t.Errorf("unexpected body: %s", string(body))
	}
}

func TestNewAPIClient_CreateChannel(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" {
			t.Errorf("expected POST, got %s", r.Method)
		}
		if r.Header.Get("Content-Type") != "application/json" {
			t.Error("missing Content-Type")
		}
		body, _ := io.ReadAll(r.Body)
		w.WriteHeader(http.StatusCreated)
		w.Write(body)
	}))
	defer ts.Close()

	c := NewNewAPIClient(ts.Client(), ts.URL)
	payload := json.RawMessage(`{"name":"test"}`)
	resp, err := c.CreateChannel("token", payload)
	if err != nil {
		t.Fatalf("CreateChannel: %v", err)
	}
	if string(resp) != `{"name":"test"}` {
		t.Errorf("unexpected response: %s", string(resp))
	}
}

func TestNewAPIClient_CreateChannel_Error(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte(`{"error":"bad request"}`))
	}))
	defer ts.Close()

	c := NewNewAPIClient(ts.Client(), ts.URL)
	_, err := c.CreateChannel("token", json.RawMessage(`{}`))
	if err == nil {
		t.Fatal("expected error for 400 response")
	}
}

func TestNewAPIClient_DeleteChannel(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "DELETE" {
			t.Errorf("expected DELETE, got %s", r.Method)
		}
		if r.URL.Path != "/api/channel/42" {
			t.Errorf("expected path /api/channel/42, got %s", r.URL.Path)
		}
		w.WriteHeader(http.StatusNoContent)
	}))
	defer ts.Close()

	c := NewNewAPIClient(ts.Client(), ts.URL)
	err := c.DeleteChannel("token", "42")
	if err != nil {
		t.Fatalf("DeleteChannel: %v", err)
	}
}

func TestNewAPIClient_DeleteChannel_Error(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer ts.Close()

	c := NewNewAPIClient(ts.Client(), ts.URL)
	err := c.DeleteChannel("token", "999")
	if err == nil {
		t.Fatal("expected error for 404 response")
	}
}

func TestNewAPIClient_SendChatCompletion(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" {
			t.Errorf("expected POST, got %s", r.Method)
		}
		if r.URL.Path != "/v1/chat/completions" {
			t.Errorf("expected path /v1/chat/completions, got %s", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"id":"chatcmpl-1","choices":[]}`))
	}))
	defer ts.Close()

	c := NewNewAPIClient(ts.Client(), ts.URL)
	resp, err := c.SendChatCompletion("key", json.RawMessage(`{"model":"test"}`))
	if err != nil {
		t.Fatalf("SendChatCompletion: %v", err)
	}
	if len(resp) == 0 {
		t.Error("expected non-empty response")
	}
}

func TestNewAPIClient_SendChatCompletion_Error(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusTooManyRequests)
		w.Write([]byte(`{"error":"rate limited"}`))
	}))
	defer ts.Close()

	c := NewNewAPIClient(ts.Client(), ts.URL)
	_, err := c.SendChatCompletion("key", json.RawMessage(`{}`))
	if err == nil {
		t.Fatal("expected error for 429 response")
	}
}

// --- BifrostClient tests ---

func TestNewBifrostClient(t *testing.T) {
	c := NewBifrostClient(NewHTTPClient(), "http://localhost:8080")
	if c == nil {
		t.Fatal("expected non-nil client")
	}
	if c.BaseURL() != "http://localhost:8080" {
		t.Errorf("expected base URL http://localhost:8080, got %s", c.BaseURL())
	}
}

func TestBifrostClient_CheckHealth(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/health" {
			t.Errorf("expected /health, got %s", r.URL.Path)
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer ts.Close()

	c := NewBifrostClient(ts.Client(), ts.URL)
	ok, err := c.CheckHealth()
	if err != nil {
		t.Fatalf("CheckHealth: %v", err)
	}
	if !ok {
		t.Error("expected healthy")
	}
}

func TestBifrostClient_CheckHealth_Unhealthy(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusServiceUnavailable)
	}))
	defer ts.Close()

	c := NewBifrostClient(ts.Client(), ts.URL)
	ok, err := c.CheckHealth()
	if err != nil {
		t.Fatalf("CheckHealth: %v", err)
	}
	if ok {
		t.Error("expected unhealthy")
	}
}

func TestBifrostClient_ListProviders(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/providers" {
			t.Errorf("expected /api/providers, got %s", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`[{"id":"p1"}]`))
	}))
	defer ts.Close()

	c := NewBifrostClient(ts.Client(), ts.URL)
	body, err := c.ListProviders()
	if err != nil {
		t.Fatalf("ListProviders: %v", err)
	}
	if string(body) != `[{"id":"p1"}]` {
		t.Errorf("unexpected body: %s", string(body))
	}
}

func TestBifrostClient_CreateProvider(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" {
			t.Errorf("expected POST, got %s", r.Method)
		}
		w.WriteHeader(http.StatusCreated)
		w.Write([]byte(`{"id":"new"}`))
	}))
	defer ts.Close()

	c := NewBifrostClient(ts.Client(), ts.URL)
	resp, err := c.CreateProvider(json.RawMessage(`{"name":"test"}`))
	if err != nil {
		t.Fatalf("CreateProvider: %v", err)
	}
	if string(resp) != `{"id":"new"}` {
		t.Errorf("unexpected response: %s", string(resp))
	}
}

func TestBifrostClient_CreateProvider_Error(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte(`bad`))
	}))
	defer ts.Close()

	c := NewBifrostClient(ts.Client(), ts.URL)
	_, err := c.CreateProvider(json.RawMessage(`{}`))
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestBifrostClient_DeleteProvider(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "DELETE" {
			t.Errorf("expected DELETE, got %s", r.Method)
		}
		if r.URL.Path != "/api/providers/p1" {
			t.Errorf("expected /api/providers/p1, got %s", r.URL.Path)
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer ts.Close()

	c := NewBifrostClient(ts.Client(), ts.URL)
	err := c.DeleteProvider("p1")
	if err != nil {
		t.Fatalf("DeleteProvider: %v", err)
	}
}

func TestBifrostClient_DeleteProvider_Error(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer ts.Close()

	c := NewBifrostClient(ts.Client(), ts.URL)
	err := c.DeleteProvider("nonexistent")
	if err == nil {
		t.Fatal("expected error")
	}
}

// --- ClewdRClient tests ---

func TestNewClewdRClient(t *testing.T) {
	c := NewClewdRClient(NewHTTPClient())
	if c == nil {
		t.Fatal("expected non-nil client")
	}
}

func TestClewdRClient_CheckHealth(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/" {
			t.Errorf("expected /, got %s", r.URL.Path)
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer ts.Close()

	c := NewClewdRClient(ts.Client())
	ok, err := c.CheckHealth(ts.URL)
	if err != nil {
		t.Fatalf("CheckHealth: %v", err)
	}
	if !ok {
		t.Error("expected healthy")
	}
}

func TestClewdRClient_CheckHealth_Redirect(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusFound)
	}))
	defer ts.Close()

	// Need a client that does NOT follow redirects
	noRedirectClient := &http.Client{
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}
	c := NewClewdRClient(noRedirectClient)
	ok, err := c.CheckHealth(ts.URL)
	if err != nil {
		t.Fatalf("CheckHealth: %v", err)
	}
	if !ok {
		t.Error("expected healthy (302 is acceptable)")
	}
}

func TestClewdRClient_CheckHealth_Unhealthy(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer ts.Close()

	c := NewClewdRClient(ts.Client())
	ok, err := c.CheckHealth(ts.URL)
	if err != nil {
		t.Fatalf("CheckHealth: %v", err)
	}
	if ok {
		t.Error("expected unhealthy for 500")
	}
}

func TestClewdRClient_GetCookies(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/cookies" {
			t.Errorf("expected /api/cookies, got %s", r.URL.Path)
		}
		if r.Header.Get("Authorization") != "Bearer secret" {
			t.Error("missing or wrong auth header")
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"valid":[{"id":1}],"exhausted":[{"id":2}],"invalid":[]}`))
	}))
	defer ts.Close()

	c := NewClewdRClient(ts.Client())
	cr, err := c.GetCookies(ts.URL, "secret")
	if err != nil {
		t.Fatalf("GetCookies: %v", err)
	}
	if len(cr.Valid) != 1 {
		t.Errorf("expected 1 valid cookie, got %d", len(cr.Valid))
	}
	if len(cr.Exhausted) != 1 {
		t.Errorf("expected 1 exhausted cookie, got %d", len(cr.Exhausted))
	}
	if len(cr.Invalid) != 0 {
		t.Errorf("expected 0 invalid cookies, got %d", len(cr.Invalid))
	}
}

func TestClewdRClient_GetCookies_NoToken(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "" {
			t.Error("expected no auth header when token is empty")
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"valid":[],"exhausted":[],"invalid":[]}`))
	}))
	defer ts.Close()

	c := NewClewdRClient(ts.Client())
	cr, err := c.GetCookies(ts.URL, "")
	if err != nil {
		t.Fatalf("GetCookies: %v", err)
	}
	if cr == nil {
		t.Fatal("expected non-nil response")
	}
}

func TestClewdRClient_GetCookies_Error(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusForbidden)
		w.Write([]byte(`forbidden`))
	}))
	defer ts.Close()

	c := NewClewdRClient(ts.Client())
	_, err := c.GetCookies(ts.URL, "bad-token")
	if err == nil {
		t.Fatal("expected error for 403 response")
	}
}

func TestClewdRClient_GetCookies_BadJSON(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`not json`))
	}))
	defer ts.Close()

	c := NewClewdRClient(ts.Client())
	_, err := c.GetCookies(ts.URL, "")
	if err == nil {
		t.Fatal("expected error for bad JSON")
	}
}
