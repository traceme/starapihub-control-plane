package main

import (
	"fmt"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"github.com/starapihub/dashboard/internal/audit"
	"github.com/starapihub/dashboard/internal/drift"
	"github.com/starapihub/dashboard/internal/registry"
	"github.com/starapihub/dashboard/internal/sync"
	"github.com/starapihub/dashboard/internal/upstream"
)

// ExitError is a sentinel error that carries a specific exit code.
// Used by diffCmd to signal severity-based exit codes to main().
type ExitError struct {
	Code int
}

func (e *ExitError) Error() string {
	return fmt.Sprintf("exit code %d", e.Code)
}

func syncCmd() *cobra.Command {
	var (
		dryRun   bool
		prune    bool
		failFast bool
		target   string // comma-separated: "channels,providers"
		auditLog string // --audit-log flag
		noAudit  bool   // --no-audit flag
	)

	cmd := &cobra.Command{
		Use:   "sync",
		Short: "Sync desired state to upstream systems",
		Long:  "Reconcile desired-state YAML registries against live upstream systems (New-API, Bifrost, ClewdR). Creates, updates, and optionally deletes resources to match.",
		RunE: func(cmd *cobra.Command, args []string) error {
			syncStartTime := time.Now()

			// 1. Load registry
			reg, err := registry.LoadAll(configDir)
			if err != nil {
				return fmt.Errorf("load registry: %w", err)
			}

			// 2. Read connection config from env vars
			newAPIURL := os.Getenv("NEWAPI_URL")
			if newAPIURL == "" {
				return fmt.Errorf("NEWAPI_URL environment variable required")
			}
			newAPIToken := os.Getenv("NEWAPI_ADMIN_TOKEN")
			if newAPIToken == "" {
				return fmt.Errorf("NEWAPI_ADMIN_TOKEN environment variable required")
			}
			bifrostURL := os.Getenv("BIFROST_URL")
			if bifrostURL == "" {
				return fmt.Errorf("BIFROST_URL environment variable required")
			}
			clewdrURLsStr := os.Getenv("CLEWDR_URLS")
			clewdrToken := os.Getenv("CLEWDR_ADMIN_TOKEN")
			// ClewdR URLs are optional (if no cookies in registry)
			var clewdrURLs []string
			if clewdrURLsStr != "" {
				clewdrURLs = strings.Split(clewdrURLsStr, ",")
				for i := range clewdrURLs {
					clewdrURLs[i] = strings.TrimSpace(clewdrURLs[i])
				}
			}

			// 3. Parse and normalize targets
			var targets []string
			if target != "" {
				raw := strings.Split(target, ",")
				for i := range raw {
					raw[i] = strings.TrimSpace(raw[i])
				}
				var normErr error
				targets, normErr = sync.NormalizeTargets(raw)
				if normErr != nil {
					return normErr
				}
			}

			// 4. Create upstream clients
			httpClient := &http.Client{Timeout: 30 * time.Second}
			newAPIClient := upstream.NewNewAPIClient(httpClient, newAPIURL)
			bifrostClient := upstream.NewBifrostClient(httpClient, bifrostURL)
			clewdrClient := upstream.NewClewdRClient(httpClient)

			// 5. Build sync options
			opts := sync.SyncOptions{
				DryRun:   dryRun,
				Prune:    prune,
				FailFast: failFast,
				Verbose:  verbose,
				Targets:  targets,
				Output:   output,
			}

			// 6. Build reconcilers in dependency order
			reconcilers := buildReconcilers(reg, newAPIClient, newAPIToken, bifrostClient, clewdrClient, clewdrURLs, clewdrToken, prune)

			// 7. Fetch live state and build desired state maps
			desiredState := buildDesiredState(reg)
			liveState := fetchLiveState(newAPIClient, newAPIToken, bifrostClient, clewdrClient, clewdrURLs, clewdrToken, verbose)

			// 8. Create and run orchestrator
			orch := sync.NewSyncOrchestrator(reconcilers, opts, desiredState, liveState)
			report, err := orch.Run()
			if err != nil {
				// Fatal error (exit code 2)
				return err
			}

			// 9. Audit logging
			if !noAudit && !dryRun {
				auditLogger := audit.NewLogger(auditLog)
				auditDuration := time.Since(syncStartTime)
				if auditErr := auditLogger.Write(report, "sync", targets, auditDuration); auditErr != nil {
					if verbose {
						fmt.Fprintf(cmd.ErrOrStderr(), "WARNING: audit log write failed: %v\n", auditErr)
					}
				}
			}

			// 10. Format and print report
			if output == "json" {
				data, _ := sync.FormatJSONReport(report)
				fmt.Fprintln(cmd.OutOrStdout(), string(data))
			} else {
				fmt.Fprintln(cmd.OutOrStdout(), sync.FormatTextReport(report, verbose))
			}

			// 11. Exit code via error return
			if report.Failed > 0 {
				return fmt.Errorf("sync completed with %d failures", report.Failed)
			}
			return nil
		},
	}

	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "Show what would change without applying")
	cmd.Flags().BoolVar(&prune, "prune", false, "Delete resources not in desired state")
	cmd.Flags().BoolVar(&failFast, "fail-fast", false, "Abort on first error")
	cmd.Flags().StringVar(&target, "target", "", "Comma-separated resource types (channel,provider,config,routing-rule,pricing,cookie)")
	cmd.Flags().StringVar(&auditLog, "audit-log", "", "Path to audit log file (default: ~/.starapihub/audit.log)")
	cmd.Flags().BoolVar(&noAudit, "no-audit", false, "Disable audit logging")

	return cmd
}

func diffCmd() *cobra.Command {
	var (
		target     string // comma-separated resource types
		severity   string // minimum severity to display
		exitWarn   bool   // treat warnings as exit 0
		reportFile string // write JSON report to file
	)

	cmd := &cobra.Command{
		Use:   "diff",
		Short: "Show drift between desired and actual state",
		Long:  "Detect and classify drift between desired-state YAML registries and live upstream systems. Produces severity-tagged output suitable for CI pipelines.",
		RunE: func(cmd *cobra.Command, args []string) error {
			// 1. Load registry
			reg, err := registry.LoadAll(configDir)
			if err != nil {
				return fmt.Errorf("load registry: %w", err)
			}

			// 2. Read connection config from env vars
			newAPIURL := os.Getenv("NEWAPI_URL")
			if newAPIURL == "" {
				return fmt.Errorf("NEWAPI_URL environment variable required")
			}
			newAPIToken := os.Getenv("NEWAPI_ADMIN_TOKEN")
			if newAPIToken == "" {
				return fmt.Errorf("NEWAPI_ADMIN_TOKEN environment variable required")
			}
			bifrostURL := os.Getenv("BIFROST_URL")
			if bifrostURL == "" {
				return fmt.Errorf("BIFROST_URL environment variable required")
			}
			clewdrURLsStr := os.Getenv("CLEWDR_URLS")
			clewdrToken := os.Getenv("CLEWDR_ADMIN_TOKEN")
			var clewdrURLs []string
			if clewdrURLsStr != "" {
				clewdrURLs = strings.Split(clewdrURLsStr, ",")
				for i := range clewdrURLs {
					clewdrURLs[i] = strings.TrimSpace(clewdrURLs[i])
				}
			}

			// 3. Parse and normalize targets
			var targets []string
			if target != "" {
				raw := strings.Split(target, ",")
				for i := range raw {
					raw[i] = strings.TrimSpace(raw[i])
				}
				var normErr error
				targets, normErr = sync.NormalizeTargets(raw)
				if normErr != nil {
					return normErr
				}
			}

			// 4. Create upstream clients
			httpClient := &http.Client{Timeout: 30 * time.Second}
			newAPIClient := upstream.NewNewAPIClient(httpClient, newAPIURL)
			bifrostClient := upstream.NewBifrostClient(httpClient, bifrostURL)
			clewdrClient := upstream.NewClewdRClient(httpClient)

			// 5. Build sync options (always dry-run for diff)
			opts := sync.SyncOptions{
				DryRun:  true,
				Verbose: verbose,
				Targets: targets,
				Output:  output,
			}

			// 6. Build reconcilers, desired state, live state
			reconcilers := buildReconcilers(reg, newAPIClient, newAPIToken, bifrostClient, clewdrClient, clewdrURLs, clewdrToken, false)
			desiredState := buildDesiredState(reg)
			liveState := fetchLiveState(newAPIClient, newAPIToken, bifrostClient, clewdrClient, clewdrURLs, clewdrToken, verbose)

			// 7. Run sync in dry-run mode to get SyncReport
			orch := sync.NewSyncOrchestrator(reconcilers, opts, desiredState, liveState)
			report, err := orch.Run()
			if err != nil {
				return err
			}

			// 8. Run drift detection
			detector := drift.NewDriftDetector()
			driftReport := detector.Detect(report)
			driftReport.DesiredStateDir = configDir

			// 9. Apply severity filter
			displayReport := applySeverityFilter(driftReport, severity)
			displayVerbose := resolveVerbose(verbose, severity)

			// 10. Format output
			var formatted string
			if output == "json" {
				data, jsonErr := drift.FormatJSONDriftReport(displayReport, displayVerbose)
				if jsonErr != nil {
					return fmt.Errorf("format JSON: %w", jsonErr)
				}
				formatted = string(data)
			} else {
				formatted = drift.FormatTextDriftReport(displayReport, displayVerbose)
			}
			fmt.Fprintln(cmd.OutOrStdout(), formatted)

			// 11. Write report file if requested (always JSON)
			if reportFile != "" {
				data, jsonErr := drift.FormatJSONDriftReport(driftReport, true)
				if jsonErr != nil {
					return fmt.Errorf("format report file: %w", jsonErr)
				}
				if writeErr := os.WriteFile(reportFile, data, 0644); writeErr != nil {
					return fmt.Errorf("write report file: %w", writeErr)
				}
			}

			// 12. Determine exit code based on severity
			cmd.SilenceErrors = true
			cmd.SilenceUsage = true
			if driftReport.HasBlocking() {
				return &ExitError{Code: 2}
			}
			if driftReport.HasWarning() && !exitWarn {
				return &ExitError{Code: 1}
			}
			return nil
		},
	}

	cmd.Flags().StringVar(&target, "target", "", "Comma-separated resource types (channel,provider,config,routing-rule,pricing,cookie)")
	cmd.Flags().StringVar(&severity, "severity", "warning", "Minimum severity to display: informational, warning, blocking")
	cmd.Flags().BoolVar(&exitWarn, "exit-warn", false, "Treat warnings as exit 0 (lenient mode for CI)")
	cmd.Flags().StringVar(&reportFile, "report-file", "", "Write full JSON drift report to file")

	return cmd
}

// applySeverityFilter returns a filtered copy of the drift report if needed.
func applySeverityFilter(report *drift.DriftReport, severity string) *drift.DriftReport {
	switch drift.DriftSeverity(severity) {
	case drift.SeverityBlocking:
		filtered := *report
		filtered.Entries = report.FilterBySeverity(drift.SeverityBlocking)
		return &filtered
	default:
		return report
	}
}

// resolveVerbose determines the verbose flag to pass to formatters based on severity.
func resolveVerbose(verbose bool, severity string) bool {
	switch drift.DriftSeverity(severity) {
	case drift.SeverityInformational:
		return true
	case drift.SeverityBlocking:
		return true
	default:
		return verbose
	}
}

// buildReconcilers creates all 6 reconcilers in dependency order.
func buildReconcilers(
	reg *registry.Registry,
	newAPIClient *upstream.NewAPIClient,
	newAPIToken string,
	bifrostClient *upstream.BifrostClient,
	clewdrClient *upstream.ClewdRClient,
	clewdrURLs []string,
	clewdrToken string,
	prune bool,
) []sync.Reconciler {
	var reconcilers []sync.Reconciler

	// 1. Cookie reconcilers (one per ClewdR instance)
	for _, url := range clewdrURLs {
		reconcilers = append(reconcilers, sync.NewCookieReconciler(clewdrClient, url, clewdrToken))
	}
	// If no ClewdR URLs but cookies target requested, add a placeholder
	// so target filtering can match "cookie"
	if len(clewdrURLs) == 0 {
		// No cookie reconciler if no instances
	}

	// 2. Provider reconciler
	reconcilers = append(reconcilers, sync.NewProviderReconciler(bifrostClient, prune))

	// 3. Config reconciler
	reconcilers = append(reconcilers, sync.NewConfigReconciler(bifrostClient))

	// 4. Routing rule reconciler
	reconcilers = append(reconcilers, sync.NewRoutingRuleReconciler(bifrostClient, prune))

	// 5. Channel reconciler
	keyResolver := func(envName string) string {
		return os.Getenv(envName)
	}
	reconcilers = append(reconcilers, sync.NewChannelReconciler(newAPIClient, newAPIToken, prune, keyResolver))

	// 6. Pricing reconciler
	reconcilers = append(reconcilers, sync.NewPricingReconciler(newAPIClient, newAPIToken))

	return reconcilers
}

// buildDesiredState extracts desired state from the registry keyed by reconciler name.
func buildDesiredState(reg *registry.Registry) map[string]any {
	ds := make(map[string]any)

	// Cookies: managed via ClewdR admin UI, not stored in YAML registry.
	// Session tokens are authentication secrets that should not live alongside
	// infrastructure config. The CookieReconciler is push-only by design
	// (Phase 3 decision) and is a no-op with empty desired state.
	ds["cookie"] = []string{}

	if reg.Providers != nil {
		ds["provider"] = reg.Providers.Providers
	}

	// Config: extract BifrostClientConfig from providers file (if global config section exists)
	if reg.Providers != nil && reg.Providers.Config != nil {
		ds["config"] = reg.Providers.Config
	} else {
		ds["config"] = (*registry.BifrostClientConfig)(nil)
	}

	if reg.RoutingRules != nil {
		ds["routing-rule"] = reg.RoutingRules.Rules
	}

	if reg.Channels != nil {
		ds["channel"] = reg.Channels.Channels
	}

	if reg.Pricing != nil {
		ds["pricing"] = reg.Pricing.Pricing
	}

	return ds
}

// fetchLiveState queries all upstreams and builds a live state map keyed by reconciler name.
func fetchLiveState(
	newAPIClient *upstream.NewAPIClient,
	newAPIToken string,
	bifrostClient *upstream.BifrostClient,
	clewdrClient *upstream.ClewdRClient,
	clewdrURLs []string,
	clewdrToken string,
	verbose bool,
) map[string]any {
	ls := make(map[string]any)

	// Cookie live state: aggregated from all ClewdR instances
	// For simplicity, pass empty for now -- orchestrator handles per-reconciler
	ls["cookie"] = (*upstream.CookieResponseTyped)(nil)

	// Providers
	providers, err := bifrostClient.ListProvidersTyped()
	if err != nil {
		if verbose {
			fmt.Fprintf(os.Stderr, "WARNING: failed to fetch providers: %v\n", err)
		}
		ls["provider"] = map[string]upstream.BifrostProviderResponse{}
	} else {
		ls["provider"] = providers
	}

	// Config
	cfg, err := bifrostClient.GetConfigTyped()
	if err != nil {
		if verbose {
			fmt.Fprintf(os.Stderr, "WARNING: failed to fetch config: %v\n", err)
		}
		ls["config"] = (*upstream.BifrostConfigResponse)(nil)
	} else {
		ls["config"] = cfg
	}

	// Routing rules
	rules, err := bifrostClient.ListRoutingRulesTyped()
	if err != nil {
		if verbose {
			fmt.Fprintf(os.Stderr, "WARNING: failed to fetch routing rules: %v\n", err)
		}
		ls["routing-rule"] = []upstream.BifrostRoutingRuleResponse{}
	} else {
		ls["routing-rule"] = rules
	}

	// Channels
	channels, err := newAPIClient.ListChannelsTyped(newAPIToken)
	if err != nil {
		if verbose {
			fmt.Fprintf(os.Stderr, "WARNING: failed to fetch channels: %v\n", err)
		}
		ls["channel"] = []upstream.ChannelResponse{}
	} else {
		ls["channel"] = channels
	}

	// Pricing (options)
	options, err := newAPIClient.GetOptionsTyped(newAPIToken)
	if err != nil {
		if verbose {
			fmt.Fprintf(os.Stderr, "WARNING: failed to fetch options: %v\n", err)
		}
		ls["pricing"] = []upstream.OptionEntry{}
	} else {
		ls["pricing"] = options
	}

	return ls
}
