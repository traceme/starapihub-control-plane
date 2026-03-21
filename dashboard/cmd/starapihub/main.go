package main

import (
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
	rootCmd.AddCommand(stubCmd("sync", "Sync desired state to upstream systems"))
	rootCmd.AddCommand(stubCmd("diff", "Show drift between desired and actual state"))
	rootCmd.AddCommand(stubCmd("bootstrap", "Bootstrap a fresh environment"))
	rootCmd.AddCommand(stubCmd("health", "Check health of all upstream systems"))

	return rootCmd
}

func main() {
	if err := buildRootCmd().Execute(); err != nil {
		os.Exit(2)
	}
}
