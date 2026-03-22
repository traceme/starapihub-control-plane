package bootstrap

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/starapihub/dashboard/internal/upstream"
)

// PrereqItem represents a single missing prerequisite.
type PrereqItem struct {
	Name     string // e.g. "NEWAPI_URL"
	Kind     string // "env" or "file"
	Guidance string // actionable fix instruction
}

// PrereqError wraps multiple prerequisite failures with actionable guidance.
type PrereqError struct {
	Missing []PrereqItem
}

func (e *PrereqError) Error() string {
	var sb strings.Builder
	sb.WriteString("missing prerequisites:\n")
	for _, item := range e.Missing {
		sb.WriteString(fmt.Sprintf("  [%s] %s: %s\n", item.Kind, item.Name, item.Guidance))
	}
	return sb.String()
}

// StepResult represents the outcome of a single bootstrap step.
type StepResult struct {
	Name     string        // "validate-prereqs", "wait-services", "seed-admin", "run-sync", "verify-health"
	Status   string        // "ok", "skipped", "failed"
	Message  string        // human-readable summary
	Duration time.Duration
}

// ServiceStatus represents the health status of a single upstream service.
type ServiceStatus struct {
	Name         string        `json:"name"`
	URL          string        `json:"url"`
	Healthy      bool          `json:"healthy"`
	ResponseTime time.Duration `json:"response_time"`
	Error        string        `json:"error,omitempty"`
}

// BootstrapOptions holds all configuration for a bootstrap run.
type BootstrapOptions struct {
	Timeout          time.Duration // default 120s, for service waiting
	SkipSeed         bool          // --skip-seed flag
	SkipSync         bool          // --skip-sync flag
	DryRun           bool          // --dry-run flag
	ConfigDir        string        // policies directory path
	Verbose          bool
	Output           string // "text" or "json"
	NewAPIURL        string
	NewAPIAdminToken string
	BifrostURL       string
	ClewdRURLs       []string
	ClewdRAdminToken string
	AdminUsername    string // default "root"
	AdminPassword    string // default "" (generated if empty)
}

// Bootstrapper orchestrates the bootstrap sequence.
type Bootstrapper struct {
	opts          BootstrapOptions
	newAPIClient  *upstream.NewAPIClient
	bifrostClient *upstream.BifrostClient
	clewdrClient  *upstream.ClewdRClient
}

// New creates a new Bootstrapper with the given options.
func New(opts BootstrapOptions) *Bootstrapper {
	if opts.Timeout == 0 {
		opts.Timeout = 120 * time.Second
	}
	if opts.AdminUsername == "" {
		opts.AdminUsername = "root"
	}

	httpClient := &http.Client{Timeout: 10 * time.Second}

	var newAPIClient *upstream.NewAPIClient
	if opts.NewAPIURL != "" {
		newAPIClient = upstream.NewNewAPIClient(httpClient, opts.NewAPIURL)
	}

	var bifrostClient *upstream.BifrostClient
	if opts.BifrostURL != "" {
		bifrostClient = upstream.NewBifrostClient(httpClient, opts.BifrostURL)
	}

	clewdrClient := upstream.NewClewdRClient(httpClient)

	return &Bootstrapper{
		opts:          opts,
		newAPIClient:  newAPIClient,
		bifrostClient: bifrostClient,
		clewdrClient:  clewdrClient,
	}
}

// envGuidance maps env var names to actionable guidance strings.
var envGuidance = map[string]string{
	"NEWAPI_URL":        "Set NEWAPI_URL to the New-API base URL (e.g. http://newapi:3000). Export it or add to .env file.",
	"NEWAPI_ADMIN_TOKEN": "Set NEWAPI_ADMIN_TOKEN to a valid New-API admin bearer token. Get one from New-API admin UI or POST /api/setup response.",
	"BIFROST_URL":       "Set BIFROST_URL to the Bifrost base URL (e.g. http://bifrost:8080). Export it or add to .env file.",
	"CLEWDR_URLS":       "Set CLEWDR_URLS to comma-separated ClewdR instance URLs (e.g. http://clewdr-1:8484,http://clewdr-2:8484).",
	"CLEWDR_ADMIN_TOKEN": "Set CLEWDR_ADMIN_TOKEN to the ClewdR admin bearer token.",
}

// ValidatePrereqs checks env vars are populated and policy files exist.
func (b *Bootstrapper) ValidatePrereqs() *StepResult {
	start := time.Now()

	var missing []PrereqItem

	// Check env vars (by checking opts fields, which are populated from env vars)
	checks := []struct {
		value string
		name  string
	}{
		{b.opts.NewAPIURL, "NEWAPI_URL"},
		{b.opts.NewAPIAdminToken, "NEWAPI_ADMIN_TOKEN"},
		{b.opts.BifrostURL, "BIFROST_URL"},
		{strings.Join(b.opts.ClewdRURLs, ","), "CLEWDR_URLS"},
		{b.opts.ClewdRAdminToken, "CLEWDR_ADMIN_TOKEN"},
	}
	for _, c := range checks {
		if c.value == "" {
			missing = append(missing, PrereqItem{
				Name:     c.name,
				Kind:     "env",
				Guidance: envGuidance[c.name],
			})
		}
	}

	// Check policy files
	fileChecks := []struct {
		name     string
		guidance string
	}{
		{"channels.yaml", "Create channels.yaml in the policies directory. See control-plane/policies/channels.yaml for an example."},
		{"providers.yaml", "Create providers.yaml in the policies directory. See control-plane/policies/providers.yaml for an example."},
	}
	for _, fc := range fileChecks {
		path := filepath.Join(b.opts.ConfigDir, fc.name)
		if _, err := os.Stat(path); err != nil {
			missing = append(missing, PrereqItem{
				Name:     fc.name,
				Kind:     "file",
				Guidance: fc.guidance,
			})
		}
	}

	duration := time.Since(start)

	if len(missing) > 0 {
		pe := &PrereqError{Missing: missing}
		return &StepResult{
			Name:     "validate-prereqs",
			Status:   "failed",
			Message:  pe.Error(),
			Duration: duration,
		}
	}

	return &StepResult{
		Name:     "validate-prereqs",
		Status:   "ok",
		Message:  "All prerequisites validated",
		Duration: duration,
	}
}

// WaitForServices polls health endpoints with exponential backoff.
func (b *Bootstrapper) WaitForServices(ctx context.Context) *StepResult {
	start := time.Now()

	ctx, cancel := context.WithTimeout(ctx, b.opts.Timeout)
	defer cancel()

	// backoff sequence: 1s, 2s, 4s, 8s, 16s, 30s (capped)
	backoff := time.Second

	for {
		statuses := b.CheckAllHealth()

		allHealthy := true
		for _, s := range statuses {
			if !s.Healthy {
				allHealthy = false
				break
			}
		}

		if allHealthy {
			return &StepResult{
				Name:     "wait-services",
				Status:   "ok",
				Message:  formatServiceStatuses(statuses),
				Duration: time.Since(start),
			}
		}

		// Check if context (timeout) is exceeded
		select {
		case <-ctx.Done():
			return &StepResult{
				Name:     "wait-services",
				Status:   "failed",
				Message:  "timeout waiting for services:\n" + formatServiceStatuses(statuses),
				Duration: time.Since(start),
			}
		default:
		}

		// Wait with backoff
		timer := time.NewTimer(backoff)
		select {
		case <-ctx.Done():
			timer.Stop()
			statuses = b.CheckAllHealth()
			return &StepResult{
				Name:     "wait-services",
				Status:   "failed",
				Message:  "timeout waiting for services:\n" + formatServiceStatuses(statuses),
				Duration: time.Since(start),
			}
		case <-timer.C:
		}

		// Exponential backoff with cap
		backoff *= 2
		if backoff > 30*time.Second {
			backoff = 30 * time.Second
		}
	}
}

// SeedAdmin calls POST /api/setup to create admin user.
func (b *Bootstrapper) SeedAdmin() *StepResult {
	start := time.Now()

	if b.newAPIClient == nil {
		return &StepResult{
			Name:     "seed-admin",
			Status:   "failed",
			Message:  "New-API client not configured (NEWAPI_URL not set)",
			Duration: time.Since(start),
		}
	}

	username := b.opts.AdminUsername
	password := b.opts.AdminPassword

	body, err := b.newAPIClient.SetupAdmin(username, password)
	if err != nil {
		return &StepResult{
			Name:     "seed-admin",
			Status:   "failed",
			Message:  fmt.Sprintf("failed to seed admin: %v", err),
			Duration: time.Since(start),
		}
	}

	// Check if response indicates admin already exists
	var resp struct {
		Success bool   `json:"success"`
		Message string `json:"message"`
	}
	if err := json.Unmarshal(body, &resp); err == nil {
		if !resp.Success && strings.Contains(strings.ToLower(resp.Message), "already") {
			return &StepResult{
				Name:     "seed-admin",
				Status:   "skipped",
				Message:  "Admin account already exists, skipping",
				Duration: time.Since(start),
			}
		}
	}

	return &StepResult{
		Name:     "seed-admin",
		Status:   "ok",
		Message:  fmt.Sprintf("Admin account '%s' created", username),
		Duration: time.Since(start),
	}
}

// CheckAllHealth runs health checks on all services and returns per-service status.
func (b *Bootstrapper) CheckAllHealth() []ServiceStatus {
	var statuses []ServiceStatus
	var mu sync.Mutex
	var wg sync.WaitGroup

	// New-API
	if b.newAPIClient != nil {
		wg.Add(1)
		go func() {
			defer wg.Done()
			s := checkNewAPI(b.newAPIClient, b.opts.NewAPIURL)
			mu.Lock()
			statuses = append(statuses, s)
			mu.Unlock()
		}()
	}

	// Bifrost
	if b.bifrostClient != nil {
		wg.Add(1)
		go func() {
			defer wg.Done()
			s := checkBifrost(b.bifrostClient, b.opts.BifrostURL)
			mu.Lock()
			statuses = append(statuses, s)
			mu.Unlock()
		}()
	}

	// ClewdR instances
	for i, url := range b.opts.ClewdRURLs {
		wg.Add(1)
		go func(idx int, u string) {
			defer wg.Done()
			s := checkClewdR(b.clewdrClient, u, idx)
			mu.Lock()
			statuses = append(statuses, s)
			mu.Unlock()
		}(i, url)
	}

	wg.Wait()

	// Sort by name for deterministic output
	sortStatuses(statuses)
	return statuses
}

func checkNewAPI(client *upstream.NewAPIClient, url string) ServiceStatus {
	start := time.Now()
	healthy, err := client.CheckHealth()
	elapsed := time.Since(start)

	s := ServiceStatus{
		Name:         "new-api",
		URL:          url,
		Healthy:      healthy,
		ResponseTime: elapsed,
	}
	if err != nil {
		s.Error = err.Error()
	}
	return s
}

func checkBifrost(client *upstream.BifrostClient, url string) ServiceStatus {
	start := time.Now()
	healthy, err := client.CheckHealth()
	elapsed := time.Since(start)

	s := ServiceStatus{
		Name:         "bifrost",
		URL:          url,
		Healthy:      healthy,
		ResponseTime: elapsed,
	}
	if err != nil {
		s.Error = err.Error()
	}
	return s
}

func checkClewdR(client *upstream.ClewdRClient, url string, idx int) ServiceStatus {
	name := "clewdr"
	if idx > 0 {
		name = fmt.Sprintf("clewdr-%d", idx+1)
	}

	start := time.Now()
	healthy, err := client.CheckHealth(url)
	elapsed := time.Since(start)

	s := ServiceStatus{
		Name:         name,
		URL:          url,
		Healthy:      healthy,
		ResponseTime: elapsed,
	}
	if err != nil {
		s.Error = err.Error()
	}
	return s
}

func formatServiceStatuses(statuses []ServiceStatus) string {
	var sb strings.Builder
	for _, s := range statuses {
		status := "healthy"
		if !s.Healthy {
			status = "unhealthy"
		}
		sb.WriteString(fmt.Sprintf("  %s (%s): %s", s.Name, s.URL, status))
		if s.Error != "" {
			sb.WriteString(fmt.Sprintf(" -- %s", s.Error))
		}
		sb.WriteString("\n")
	}
	return sb.String()
}

func sortStatuses(statuses []ServiceStatus) {
	// Simple insertion sort for small slices
	for i := 1; i < len(statuses); i++ {
		key := statuses[i]
		j := i - 1
		for j >= 0 && statuses[j].Name > key.Name {
			statuses[j+1] = statuses[j]
			j--
		}
		statuses[j+1] = key
	}
}
