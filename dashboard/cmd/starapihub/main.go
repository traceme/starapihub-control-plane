package main

import (
	"errors"
	"os"

	"github.com/spf13/cobra"
)

var (
	configDir string
	verbose   bool
	output    string
)

func buildRootCmd() *cobra.Command {
	rootCmd := &cobra.Command{
		Use:   "starapihub",
		Short: "StarAPIHub control plane CLI",
		Long:  "Manage desired state, validate registries, sync upstream systems, and detect drift.",
	}

	// Persistent flags available to all subcommands
	rootCmd.PersistentFlags().StringVar(&configDir, "config-dir", "policies", "Path to policy registry directory")
	rootCmd.PersistentFlags().BoolVar(&verbose, "verbose", false, "Enable verbose output")
	rootCmd.PersistentFlags().StringVar(&output, "output", "text", "Output format (text|json)")

	rootCmd.AddCommand(validateCmd())
	rootCmd.AddCommand(syncCmd())
	rootCmd.AddCommand(diffCmd())
	rootCmd.AddCommand(bootstrapCmd())
	rootCmd.AddCommand(healthCmd())
	rootCmd.AddCommand(traceCmd())
	rootCmd.AddCommand(cookieStatusCmd())

	return rootCmd
}

func main() {
	if err := buildRootCmd().Execute(); err != nil {
		var exitErr *ExitError
		if errors.As(err, &exitErr) {
			os.Exit(exitErr.Code)
		}
		os.Exit(2)
	}
}
