package main

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"github.com/starapihub/dashboard/internal/bootstrap"
)

func healthCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "health",
		Short: "Check health of all upstream systems",
		Long:  "Check health of New-API, Bifrost, and each ClewdR instance. Reports per-service status with response times.",
		RunE: func(cmd *cobra.Command, args []string) error {
			// Read env vars (same pattern as sync.go)
			newAPIURL := os.Getenv("NEWAPI_URL")
			bifrostURL := os.Getenv("BIFROST_URL")
			clewdrURLsStr := os.Getenv("CLEWDR_URLS")
			clewdrToken := os.Getenv("CLEWDR_ADMIN_TOKEN")

			// Parse ClewdR URLs
			var clewdrURLs []string
			if clewdrURLsStr != "" {
				clewdrURLs = strings.Split(clewdrURLsStr, ",")
				for i := range clewdrURLs {
					clewdrURLs[i] = strings.TrimSpace(clewdrURLs[i])
				}
			}

			// Build BootstrapOptions for CheckAllHealth reuse
			opts := bootstrap.BootstrapOptions{
				NewAPIURL:        newAPIURL,
				BifrostURL:       bifrostURL,
				ClewdRURLs:       clewdrURLs,
				ClewdRAdminToken: clewdrToken,
				Verbose:          verbose,
				Output:           output,
			}
			b := bootstrap.New(opts)

			// Run health checks
			statuses := b.CheckAllHealth()

			// Handle case where no services are configured
			if len(statuses) == 0 {
				fmt.Fprintln(cmd.OutOrStdout(), "No upstream services configured. Set NEWAPI_URL, BIFROST_URL, and/or CLEWDR_URLS environment variables.")
				cmd.SilenceErrors = true
				cmd.SilenceUsage = true
				return &ExitError{Code: 1}
			}

			// Format output
			if output == "json" {
				data, _ := json.MarshalIndent(statuses, "", "  ")
				fmt.Fprintln(cmd.OutOrStdout(), string(data))
			} else {
				// Text format: table with service, URL, status, response time
				for _, s := range statuses {
					icon := "[OK]"
					if !s.Healthy {
						icon = "[FAIL]"
					}
					line := fmt.Sprintf("  %s %-12s %s", icon, s.Name, s.URL)
					if verbose && s.Healthy {
						line += fmt.Sprintf(" (%s)", s.ResponseTime.Round(time.Millisecond))
					}
					if s.Error != "" {
						line += fmt.Sprintf(" -- %s", s.Error)
					}
					fmt.Fprintln(cmd.OutOrStdout(), line)
				}
			}

			// Exit codes: 0=all healthy, 1=some unhealthy
			someUnhealthy := false
			for _, s := range statuses {
				if !s.Healthy {
					someUnhealthy = true
					break
				}
			}
			if someUnhealthy {
				cmd.SilenceErrors = true
				cmd.SilenceUsage = true
				return &ExitError{Code: 1}
			}
			return nil
		},
	}
	return cmd
}

