package main

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"github.com/starapihub/dashboard/internal/buildinfo"
)

func versionCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "version",
		Short: "Print the appliance version and build info",
		RunE: func(cmd *cobra.Command, args []string) error {
			info := buildinfo.Info(os.Getenv)
			w := cmd.OutOrStdout()

			if output == "json" {
				return json.NewEncoder(w).Encode(info)
			}

			fmt.Fprintf(w, "starapihub %s\n", info["version"])
			fmt.Fprintf(w, "  build:   %s\n", info["build_date"])
			fmt.Fprintf(w, "  go:      %s\n", info["go_version"])
			fmt.Fprintf(w, "  mode:    %s\n", info["mode"])
			return nil
		},
	}
}
