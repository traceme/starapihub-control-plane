package sync

import (
	"fmt"

	"github.com/starapihub/dashboard/internal/upstream"
)

// ClewdRCookieClient is the interface the reconciler needs from the upstream ClewdR client.
type ClewdRCookieClient interface {
	GetCookiesTyped(url, adminToken string) (*upstream.CookieResponseTyped, error)
	PostCookie(url, adminToken string, cookie string) error
}

// CookieReconciler reconciles ClewdR cookies using push-only semantics.
// It adds missing cookies but never deletes existing ones.
type CookieReconciler struct {
	client      ClewdRCookieClient
	instanceURL string
	adminToken  string
}

// NewCookieReconciler creates a CookieReconciler for the given ClewdR instance.
func NewCookieReconciler(client ClewdRCookieClient, instanceURL, adminToken string) *CookieReconciler {
	return &CookieReconciler{
		client:      client,
		instanceURL: instanceURL,
		adminToken:  adminToken,
	}
}

// Name returns the resource type name.
func (r *CookieReconciler) Name() string {
	return "cookie"
}

// Plan compares desired cookies against live cookies and returns create actions for missing ones.
// desired must be []string (cookie values). live must be *upstream.CookieResponseTyped.
// Push-only: never generates delete actions.
func (r *CookieReconciler) Plan(desired, live any) ([]Action, error) {
	desiredCookies, ok := desired.([]string)
	if !ok {
		return nil, fmt.Errorf("CookieReconciler.Plan: desired must be []string, got %T", desired)
	}
	liveCookies, ok := live.(*upstream.CookieResponseTyped)
	if !ok {
		return nil, fmt.Errorf("CookieReconciler.Plan: live must be *upstream.CookieResponseTyped, got %T", live)
	}

	// Build set of all live cookies (valid + exhausted + invalid)
	liveSet := make(map[string]struct{})
	if liveCookies != nil {
		for _, c := range liveCookies.Valid {
			liveSet[c.Cookie] = struct{}{}
		}
		for _, c := range liveCookies.Exhausted {
			liveSet[c.Cookie] = struct{}{}
		}
		for _, c := range liveCookies.Invalid {
			liveSet[c.Cookie] = struct{}{}
		}
	}

	var actions []Action
	for _, cookie := range desiredCookies {
		if _, exists := liveSet[cookie]; !exists {
			resourceID := cookie
			if len(resourceID) > 8 {
				resourceID = resourceID[:8]
			}
			actions = append(actions, Action{
				Type:         ActionCreate,
				ResourceType: "cookie",
				ResourceID:   resourceID,
				Desired:      cookie,
				Live:         nil,
			})
		}
	}
	return actions, nil
}

// Apply executes a single cookie create action by posting the cookie to ClewdR.
func (r *CookieReconciler) Apply(action Action) (*Result, error) {
	cookie, ok := action.Desired.(string)
	if !ok {
		return &Result{Action: action, Status: StatusFailed, Error: fmt.Errorf("desired is not a string")}, nil
	}

	err := r.client.PostCookie(r.instanceURL, r.adminToken, cookie)
	if err != nil {
		return &Result{Action: action, Status: StatusFailed, Error: err}, nil
	}
	return &Result{Action: action, Status: StatusOK}, nil
}

// Verify reads back the cookie list after Apply and confirms the cookie is present.
func (r *CookieReconciler) Verify(action Action, result *Result) error {
	cookie, ok := action.Desired.(string)
	if !ok {
		return fmt.Errorf("desired is not a string")
	}

	cookies, err := r.client.GetCookiesTyped(r.instanceURL, r.adminToken)
	if err != nil {
		result.Status = StatusUnverified
		result.DriftMsg = fmt.Sprintf("failed to read back cookies: %v", err)
		return nil
	}
	result.ReadBack = cookies

	// Check if cookie appears in valid or exhausted lists
	for _, c := range cookies.Valid {
		if c.Cookie == cookie {
			return nil
		}
	}
	for _, c := range cookies.Exhausted {
		if c.Cookie == cookie {
			return nil
		}
	}

	// Cookie not found after posting -- drift
	result.Status = StatusAppliedWithDrift
	result.DriftMsg = fmt.Sprintf("cookie %s not found in read-back after post", action.ResourceID)
	return nil
}

// Compile-time interface check.
var _ Reconciler = (*CookieReconciler)(nil)
