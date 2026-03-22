package main

import (
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"
	"github.com/starapihub/dashboard/internal/trace"
)

func traceCmd() *cobra.Command {
	var (
		logDir string
	)

	cmd := &cobra.Command{
		Use:   "trace <request-id>",
		Short: "Trace a request across all layers",
		Long:  "Search container logs (or saved log files) for a request ID across Nginx, New-API, Bifrost, and ClewdR layers. Extracts per-layer metadata including user, model, and provider.",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			requestID := args[0]

			opts := trace.TraceOptions{
				RequestID:      requestID,
				ContainerNames: []string{"cp-nginx", "cp-new-api", "cp-bifrost"},
				LogDir:         logDir,
				Verbose:        verbose,
			}

			// Add ClewdR containers from env if available
			clewdrURLsStr := os.Getenv("CLEWDR_URLS")
			if clewdrURLsStr != "" {
				urls := strings.Split(clewdrURLsStr, ",")
				for i := range urls {
					_ = urls[i] // just need the count
					opts.ContainerNames = append(opts.ContainerNames, fmt.Sprintf("cp-clewdr-%d", i+1))
				}
			}

			tracer := trace.NewTracer(opts)
			result, err := tracer.Run()
			if err != nil {
				return fmt.Errorf("trace: %w", err)
			}

			// Format output
			if output == "json" {
				data, jsonErr := trace.FormatJSONTrace(result)
				if jsonErr != nil {
					return fmt.Errorf("format JSON: %w", jsonErr)
				}
				fmt.Fprintln(cmd.OutOrStdout(), string(data))
			} else {
				fmt.Fprintln(cmd.OutOrStdout(), trace.FormatTextTrace(result, verbose))
			}

			// Exit code: 0 if any matches found, 1 if no matches
			if len(result.Layers) == 0 {
				fmt.Fprintln(cmd.ErrOrStderr(), "No matches found for request ID:", requestID)
				cmd.SilenceErrors = true
				cmd.SilenceUsage = true
				return &ExitError{Code: 1}
			}
			return nil
		},
	}

	cmd.Flags().StringVar(&logDir, "log-dir", "", "Read logs from directory instead of docker containers")

	return cmd
}
