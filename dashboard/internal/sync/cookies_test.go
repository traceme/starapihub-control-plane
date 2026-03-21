package sync

import (
	"fmt"
	"os"
	"testing"

	"github.com/starapihub/dashboard/internal/upstream"
)

// --- Mock ClewdR client ---

type mockClewdRClient struct {
	cookies    *upstream.CookieResponseTyped
	getErr     error
	postErr    error
	postCalled []string
}

func (m *mockClewdRClient) GetCookiesTyped(url, adminToken string) (*upstream.CookieResponseTyped, error) {
	if m.getErr != nil {
		return nil, m.getErr
	}
	return m.cookies, nil
}

func (m *mockClewdRClient) PostCookie(url, adminToken string, cookie string) error {
	m.postCalled = append(m.postCalled, cookie)
	return m.postErr
}

// --- CookieReconciler Tests ---

func TestCookiePlan_3Desired1Present_Returns2Creates(t *testing.T) {
	client := &mockClewdRClient{
		cookies: &upstream.CookieResponseTyped{
			Valid: []upstream.CookieStatusTyped{{Cookie: "cookie-aaa"}},
		},
	}
	r := NewCookieReconciler(client, "http://localhost:8484", "token")

	actions, err := r.Plan(
		[]string{"cookie-aaa", "cookie-bbb", "cookie-ccc"},
		client.cookies,
	)
	if err != nil {
		t.Fatalf("Plan error: %v", err)
	}
	if len(actions) != 2 {
		t.Fatalf("expected 2 actions, got %d", len(actions))
	}
	for _, a := range actions {
		if a.Type != ActionCreate {
			t.Errorf("expected create action, got %s", a.Type)
		}
		if a.ResourceType != "cookie" {
			t.Errorf("expected resource type cookie, got %s", a.ResourceType)
		}
	}
}

func TestCookiePlan_0Desired_Returns0Actions(t *testing.T) {
	client := &mockClewdRClient{
		cookies: &upstream.CookieResponseTyped{
			Valid: []upstream.CookieStatusTyped{{Cookie: "cookie-aaa"}},
		},
	}
	r := NewCookieReconciler(client, "http://localhost:8484", "token")

	actions, err := r.Plan([]string{}, client.cookies)
	if err != nil {
		t.Fatalf("Plan error: %v", err)
	}
	if len(actions) != 0 {
		t.Fatalf("expected 0 actions for empty desired, got %d", len(actions))
	}
}

func TestCookiePlan_AllPresent_Returns0Actions(t *testing.T) {
	client := &mockClewdRClient{
		cookies: &upstream.CookieResponseTyped{
			Valid:     []upstream.CookieStatusTyped{{Cookie: "cookie-aaa"}},
			Exhausted: []upstream.CookieStatusTyped{{Cookie: "cookie-bbb"}},
			Invalid:   []upstream.CookieStatusTyped{{Cookie: "cookie-ccc"}},
		},
	}
	r := NewCookieReconciler(client, "http://localhost:8484", "token")

	actions, err := r.Plan(
		[]string{"cookie-aaa", "cookie-bbb", "cookie-ccc"},
		client.cookies,
	)
	if err != nil {
		t.Fatalf("Plan error: %v", err)
	}
	if len(actions) != 0 {
		t.Fatalf("expected 0 actions (idempotent), got %d", len(actions))
	}
}

func TestCookieApply_CallsPostCookie(t *testing.T) {
	client := &mockClewdRClient{}
	r := NewCookieReconciler(client, "http://localhost:8484", "token")

	action := Action{
		Type:         ActionCreate,
		ResourceType: "cookie",
		ResourceID:   "cookie-b",
		Desired:      "cookie-bbb-full-value",
	}
	result, err := r.Apply(action)
	if err != nil {
		t.Fatalf("Apply error: %v", err)
	}
	if result.Status != StatusOK {
		t.Errorf("expected StatusOK, got %s", result.Status)
	}
	if len(client.postCalled) != 1 || client.postCalled[0] != "cookie-bbb-full-value" {
		t.Errorf("expected PostCookie called with cookie-bbb-full-value, got %v", client.postCalled)
	}
}

func TestCookieVerify_CookiePresent_StatusOK(t *testing.T) {
	client := &mockClewdRClient{
		cookies: &upstream.CookieResponseTyped{
			Valid: []upstream.CookieStatusTyped{{Cookie: "cookie-new"}},
		},
	}
	r := NewCookieReconciler(client, "http://localhost:8484", "token")

	action := Action{
		Type:         ActionCreate,
		ResourceType: "cookie",
		ResourceID:   "cookie-n",
		Desired:      "cookie-new",
	}
	result := &Result{Action: action, Status: StatusOK}
	err := r.Verify(action, result)
	if err != nil {
		t.Fatalf("Verify error: %v", err)
	}
	if result.Status != StatusOK {
		t.Errorf("expected StatusOK after verify, got %s", result.Status)
	}
}

func TestCookieVerify_CookieNotFound_AppliedWithDrift(t *testing.T) {
	client := &mockClewdRClient{
		cookies: &upstream.CookieResponseTyped{
			Valid: []upstream.CookieStatusTyped{{Cookie: "other-cookie"}},
		},
	}
	r := NewCookieReconciler(client, "http://localhost:8484", "token")

	action := Action{
		Type:         ActionCreate,
		ResourceType: "cookie",
		ResourceID:   "cookie-n",
		Desired:      "cookie-new",
	}
	result := &Result{Action: action, Status: StatusOK}
	err := r.Verify(action, result)
	if err != nil {
		t.Fatalf("Verify error: %v", err)
	}
	if result.Status != StatusAppliedWithDrift {
		t.Errorf("expected StatusAppliedWithDrift, got %s", result.Status)
	}
	if result.DriftMsg == "" {
		t.Error("expected non-empty DriftMsg")
	}
}

func TestResolveEnvVar_Set_ReturnsValue(t *testing.T) {
	os.Setenv("TEST_SYNC_VAR", "hello-world")
	defer os.Unsetenv("TEST_SYNC_VAR")

	val, err := ResolveEnvVar("TEST_SYNC_VAR")
	if err != nil {
		t.Fatalf("ResolveEnvVar error: %v", err)
	}
	if val != "hello-world" {
		t.Errorf("expected hello-world, got %s", val)
	}
}

func TestResolveEnvVar_NotSet_ReturnsError(t *testing.T) {
	os.Unsetenv("TEST_SYNC_VAR_MISSING")

	_, err := ResolveEnvVar("TEST_SYNC_VAR_MISSING")
	if err == nil {
		t.Fatal("expected error for unset env var")
	}
	_ = fmt.Sprintf("%v", err) // ensure error message is printable
}
