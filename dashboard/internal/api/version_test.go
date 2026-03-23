package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/starapihub/dashboard/internal/buildinfo"
)

func TestHandleVersion_ReturnsBuildinfoMetadata(t *testing.T) {
	// Set known build values
	oldVersion := buildinfo.Version
	oldBuild := buildinfo.BuildDate
	defer func() {
		buildinfo.Version = oldVersion
		buildinfo.BuildDate = oldBuild
	}()

	buildinfo.Version = "0.2.0-test"
	buildinfo.BuildDate = "2026-03-22T12:00:00Z"
	t.Setenv("STARAPIHUB_MODE", "")

	h := newTestHandler(t)
	router := NewRouter(h, testToken, nil)

	// /api/version does NOT require auth
	req := httptest.NewRequest("GET", "/api/version", nil)
	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}

	var resp map[string]string
	if err := json.NewDecoder(rr.Body).Decode(&resp); err != nil {
		t.Fatalf("decode: %v", err)
	}

	if resp["version"] != "0.2.0-test" {
		t.Errorf("expected version=0.2.0-test, got %s", resp["version"])
	}
	if resp["build_date"] != "2026-03-22T12:00:00Z" {
		t.Errorf("expected build_date=2026-03-22T12:00:00Z, got %s", resp["build_date"])
	}
	if resp["go_version"] == "" {
		t.Error("expected non-empty go_version")
	}
	// Without STARAPIHUB_MODE set, mode should be "unknown"
	if resp["mode"] != "unknown" {
		t.Errorf("expected mode=unknown, got %s", resp["mode"])
	}
}

func TestHandleVersion_NoAuthRequired(t *testing.T) {
	h := newTestHandler(t)
	router := NewRouter(h, testToken, nil)

	// No Authorization header — should still return 200
	req := httptest.NewRequest("GET", "/api/version", nil)
	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("expected 200 without auth, got %d", rr.Code)
	}
}
