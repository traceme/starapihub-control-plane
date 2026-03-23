package main

import (
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"
	"github.com/starapihub/dashboard/internal/upgrade"
)

func upgradeCheckCmd() *cobra.Command {
	var (
		repoRoot string
		relayURL string
	)

	cmd := &cobra.Command{
		Use:   "upgrade-check",
		Short: "Verify an upgrade succeeded across all verification gates",
		Long: `Run 5 verification gates after an upstream upgrade:

  Gate 1 (Deployment):    Health check all services
  Gate 2 (Sync):          Verify registry loads (sync compatibility)
  Gate 3 (Request Path):  Probe relay endpoint availability
  Gate 4 (Auditability):  Verify X-Request-ID correlation propagation
  Gate 5 (Patch Intent):  Verify all active patches are still applied

Returns exit 0 if all gates pass, exit 1 if any gate fails.
Use --output json for CI-friendly structured output.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			// Build env-sourced URLs (same pattern as health.go and sync.go)
			newAPIURL := os.Getenv("NEWAPI_URL")
			newAPIToken := os.Getenv("NEWAPI_ADMIN_TOKEN")
			bifrostURL := os.Getenv("BIFROST_URL")
			clewdrURLsStr := os.Getenv("CLEWDR_URLS")
			clewdrToken := os.Getenv("CLEWDR_ADMIN_TOKEN")

			var clewdrURLs []string
			if clewdrURLsStr != "" {
				clewdrURLs = strings.Split(clewdrURLsStr, ",")
				for i := range clewdrURLs {
					clewdrURLs[i] = strings.TrimSpace(clewdrURLs[i])
				}
			}

			// If relayURL not set via flag, default to NEWAPI_URL
			if relayURL == "" {
				relayURL = newAPIURL
			}

			opts := upgrade.GateOptions{
				NewAPIURL:        newAPIURL,
				NewAPIAdminToken: newAPIToken,
				BifrostURL:       bifrostURL,
				ClewdRURLs:       clewdrURLs,
				ClewdRAdminToken: clewdrToken,
				ConfigDir:        configDir,
				RepoRoot:         repoRoot,
				RelayURL:         relayURL,
				Verbose:          verbose,
			}

			report := upgrade.RunAllGates(opts)

			// Format and print
			if output == "json" {
				jsonOut, err := upgrade.FormatJSON(report)
				if err != nil {
					return fmt.Errorf("format JSON: %w", err)
				}
				fmt.Fprintln(cmd.OutOrStdout(), jsonOut)
			} else {
				fmt.Fprint(cmd.OutOrStdout(), upgrade.FormatText(report, verbose))
			}

			if !report.AllPass {
				cmd.SilenceErrors = true
				cmd.SilenceUsage = true
				return &ExitError{Code: 1}
			}
			return nil
		},
	}

	cmd.Flags().StringVar(&repoRoot, "repo-root", ".", "Path to repository root (for patch verification and version extraction)")
	cmd.Flags().StringVar(&relayURL, "relay-url", "", "URL to test relay endpoint (defaults to NEWAPI_URL)")
	return cmd
}
