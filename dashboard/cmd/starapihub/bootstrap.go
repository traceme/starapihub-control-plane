package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"github.com/starapihub/dashboard/internal/audit"
	"github.com/starapihub/dashboard/internal/bootstrap"
	"github.com/starapihub/dashboard/internal/registry"
	syncpkg "github.com/starapihub/dashboard/internal/sync"
	"github.com/starapihub/dashboard/internal/upstream"
)

func bootstrapCmd() *cobra.Command {
	var (
		timeout  string
		skipSeed bool
		skipSync bool
		dryRun   bool
		auditLog string
		noAudit  bool
	)

	cmd := &cobra.Command{
		Use:   "bootstrap",
		Short: "Bootstrap a fresh environment from zero to healthy",
		Long:  "Validate prerequisites, wait for services, seed admin account, sync all configuration, and verify health. Idempotent -- safe to re-run.",
		RunE: func(cmd *cobra.Command, args []string) error {
			bootStartTime := time.Now()

			// Parse timeout
			dur, err := time.ParseDuration(timeout)
			if err != nil {
				return fmt.Errorf("invalid --timeout value %q: %w", timeout, err)
			}

			// Read env vars
			newAPIURL := os.Getenv("NEWAPI_URL")
			newAPIToken := os.Getenv("NEWAPI_ADMIN_TOKEN")
			bifrostURL := os.Getenv("BIFROST_URL")
			clewdrURLsStr := os.Getenv("CLEWDR_URLS")
			clewdrToken := os.Getenv("CLEWDR_ADMIN_TOKEN")
			adminUsername := os.Getenv("NEWAPI_ADMIN_USERNAME")
			if adminUsername == "" {
				adminUsername = "root"
			}
			adminPassword := os.Getenv("NEWAPI_ADMIN_PASSWORD")

			var clewdrURLs []string
			if clewdrURLsStr != "" {
				clewdrURLs = strings.Split(clewdrURLsStr, ",")
				for i := range clewdrURLs {
					clewdrURLs[i] = strings.TrimSpace(clewdrURLs[i])
				}
			}

			// Build options
			opts := bootstrap.BootstrapOptions{
				Timeout:          dur,
				SkipSeed:         skipSeed,
				SkipSync:         skipSync,
				DryRun:           dryRun,
				ConfigDir:        configDir,
				Verbose:          verbose,
				Output:           output,
				NewAPIURL:        newAPIURL,
				NewAPIAdminToken: newAPIToken,
				BifrostURL:       bifrostURL,
				ClewdRURLs:       clewdrURLs,
				ClewdRAdminToken: clewdrToken,
				AdminUsername:    adminUsername,
				AdminPassword:    adminPassword,
			}

			b := bootstrap.New(opts)

			// Set up sync dependencies (only if not skip-sync)
			if !skipSync {
				reg, loadErr := registry.LoadAll(configDir)
				if loadErr != nil {
					// Don't fail here -- ValidatePrereqs will catch missing files
					// Just skip sync deps setup
				} else {
					httpClient := &http.Client{Timeout: 30 * time.Second}
					newAPIClient := upstream.NewNewAPIClient(httpClient, newAPIURL)
					bifrostClient := upstream.NewBifrostClient(httpClient, bifrostURL)
					clewdrClient := upstream.NewClewdRClient(httpClient)

					reconcilers := buildReconcilers(reg, newAPIClient, newAPIToken, bifrostClient, clewdrClient, clewdrURLs, clewdrToken, false)
					desiredState := buildDesiredState(reg)
					var liveState map[string]any
					if dryRun {
						// In dry-run mode, skip live state fetching (no API calls needed)
						liveState = make(map[string]any)
					} else {
						liveState = fetchLiveState(newAPIClient, newAPIToken, bifrostClient, clewdrClient, clewdrURLs, clewdrToken, verbose)
					}

					b.SetSyncDeps(bootstrap.SyncDeps{
						Reconcilers:  reconcilers,
						DesiredState: desiredState,
						LiveState:    liveState,
					})
				}
			}

			// Run bootstrap
			ctx, cancel := context.WithTimeout(context.Background(), dur)
			defer cancel()
			report := b.Run(ctx)

			// Audit logging
			if !noAudit && !dryRun {
				auditLogger := audit.NewLogger(auditLog)
				bootDuration := time.Since(bootStartTime)
				// Bootstrap doesn't expose SyncReport directly; write a summary entry
				simpleReport := &syncpkg.SyncReport{}
				if auditErr := auditLogger.Write(simpleReport, "bootstrap", nil, bootDuration); auditErr != nil {
					if verbose {
						fmt.Fprintf(cmd.ErrOrStderr(), "WARNING: audit log write failed: %v\n", auditErr)
					}
				}
			}

			// Format output
			if output == "json" {
				data, _ := json.MarshalIndent(report, "", "  ")
				fmt.Fprintln(cmd.OutOrStdout(), string(data))
			} else {
				// Text report
				fmt.Fprintln(cmd.OutOrStdout(), "Bootstrap Report")
				fmt.Fprintln(cmd.OutOrStdout(), strings.Repeat("=", 50))
				for _, step := range report.Steps {
					icon := "[OK]"
					if step.Status == "failed" {
						icon = "[FAIL]"
					} else if step.Status == "skipped" {
						icon = "[SKIP]"
					}
					fmt.Fprintf(cmd.OutOrStdout(), "  %s %-20s %s", icon, step.Name, step.Message)
					if verbose && step.Duration > 0 {
						fmt.Fprintf(cmd.OutOrStdout(), " (%s)", step.Duration.Round(time.Millisecond))
					}
					fmt.Fprintln(cmd.OutOrStdout())
				}
				fmt.Fprintln(cmd.OutOrStdout(), strings.Repeat("=", 50))
				if report.Success {
					fmt.Fprintln(cmd.OutOrStdout(), "Bootstrap complete.")
				} else {
					fmt.Fprintln(cmd.OutOrStdout(), "Bootstrap failed. See above for details.")
				}
			}

			// Exit code
			if !report.Success {
				cmd.SilenceErrors = true
				cmd.SilenceUsage = true
				return &ExitError{Code: 1}
			}
			return nil
		},
	}

	cmd.Flags().StringVar(&timeout, "timeout", "120s", "Timeout for service health waiting")
	cmd.Flags().BoolVar(&skipSeed, "skip-seed", false, "Skip New-API admin seeding")
	cmd.Flags().BoolVar(&skipSync, "skip-sync", false, "Skip sync and verification (validate + health only)")
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "Show what would be done without making changes")
	cmd.Flags().StringVar(&auditLog, "audit-log", "", "Path to audit log file (default: ~/.starapihub/audit.log)")
	cmd.Flags().BoolVar(&noAudit, "no-audit", false, "Disable audit logging")

	return cmd
}
