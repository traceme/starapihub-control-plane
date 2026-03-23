package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"github.com/starapihub/dashboard/internal/upstream"
)

// CookieInstanceStatus holds the cookie counts for a single ClewdR instance.
type CookieInstanceStatus struct {
	Instance  string `json:"instance"`
	Valid     int    `json:"valid"`
	Exhausted int    `json:"exhausted"`
	Invalid   int    `json:"invalid"`
	Total     int    `json:"total"`
}

func cookieStatusCmd() *cobra.Command {
	var minValid int

	cmd := &cobra.Command{
		Use:   "cookie-status",
		Short: "Check ClewdR cookie inventory across all instances",
		Long:  "Query each ClewdR instance for cookie counts (valid, exhausted, invalid) and exit non-zero when valid cookies drop below a configurable threshold.",
		RunE: func(cmd *cobra.Command, args []string) error {
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

			if len(clewdrURLs) == 0 {
				fmt.Fprintln(cmd.OutOrStdout(), "No ClewdR instances configured. Set CLEWDR_URLS environment variable.")
				cmd.SilenceErrors = true
				cmd.SilenceUsage = true
				return &ExitError{Code: 1}
			}

			httpClient := &http.Client{Timeout: 10 * time.Second}
			client := upstream.NewClewdRClient(httpClient)

			var results []CookieInstanceStatus
			var totalValid int

			for i, url := range clewdrURLs {
				name := "clewdr"
				if len(clewdrURLs) > 1 {
					name = fmt.Sprintf("clewdr-%d", i+1)
				}

				cookies, err := client.GetCookies(url, clewdrToken)
				if err != nil {
					// Report error but continue checking other instances
					result := CookieInstanceStatus{
						Instance: name,
					}
					results = append(results, result)
					if output != "json" {
						fmt.Fprintf(cmd.OutOrStdout(), "  %-14s error: %v\n", name, err)
					}
					continue
				}

				result := CookieInstanceStatus{
					Instance:  name,
					Valid:     len(cookies.Valid),
					Exhausted: len(cookies.Exhausted),
					Invalid:   len(cookies.Invalid),
					Total:     len(cookies.Valid) + len(cookies.Exhausted) + len(cookies.Invalid),
				}
				results = append(results, result)
				totalValid += result.Valid
			}

			// Format output
			if output == "json" {
				data, _ := json.MarshalIndent(results, "", "  ")
				fmt.Fprintln(cmd.OutOrStdout(), string(data))
			} else {
				fmt.Fprintln(cmd.OutOrStdout(), "ClewdR Cookie Status:")
				for _, r := range results {
					fmt.Fprintf(cmd.OutOrStdout(), "  %-14s valid: %d   exhausted: %d   invalid: %d   total: %d\n",
						r.Instance, r.Valid, r.Exhausted, r.Invalid, r.Total)
				}
				totalExhausted := 0
				totalInvalid := 0
				for _, r := range results {
					totalExhausted += r.Exhausted
					totalInvalid += r.Invalid
				}
				fmt.Fprintf(cmd.OutOrStdout(), "Summary: %d valid, %d exhausted, %d invalid across %d instances\n",
					totalValid, totalExhausted, totalInvalid, len(results))
			}

			// Exit code based on threshold
			if totalValid < minValid {
				cmd.SilenceErrors = true
				cmd.SilenceUsage = true
				return &ExitError{Code: 1}
			}

			return nil
		},
	}

	cmd.Flags().IntVar(&minValid, "min-valid", 2, "Minimum number of valid cookies before non-zero exit")

	return cmd
}
