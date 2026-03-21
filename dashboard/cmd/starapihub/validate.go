package main

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"
	"github.com/starapihub/dashboard/internal/registry"
)

func validateCmd() *cobra.Command {
	var schemasDir string

	cmd := &cobra.Command{
		Use:   "validate",
		Short: "Validate all YAML registry files against JSON Schema",
		Long:  "Checks all YAML files in the policies directory against their JSON Schema definitions. Reports schema violations with file and field context.",
		RunE: func(cmd *cobra.Command, args []string) error {
			// Resolve absolute paths
			absPolicies, err := filepath.Abs(configDir)
			if err != nil {
				return fmt.Errorf("resolve policies dir: %w", err)
			}
			absSchemas, err := filepath.Abs(schemasDir)
			if err != nil {
				return fmt.Errorf("resolve schemas dir: %w", err)
			}

			// Check directories exist
			if _, err := os.Stat(absPolicies); os.IsNotExist(err) {
				return fmt.Errorf("policies directory not found: %s", absPolicies)
			}
			if _, err := os.Stat(absSchemas); os.IsNotExist(err) {
				return fmt.Errorf("schemas directory not found: %s", absSchemas)
			}

			if verbose {
				fmt.Fprintf(cmd.OutOrStdout(), "Validating policies in: %s\n", absPolicies)
				fmt.Fprintf(cmd.OutOrStdout(), "Using schemas from: %s\n", absSchemas)
			}

			err = registry.ValidateRegistry(absPolicies, absSchemas)
			if err != nil {
				fmt.Fprintf(cmd.ErrOrStderr(), "Validation FAILED:\n%v\n", err)
				// Return an error so cobra reports non-zero exit and tests can detect failure
				return fmt.Errorf("validation failed")
			}

			fmt.Fprintf(cmd.OutOrStdout(), "All registry files passed validation.\n")
			return nil
		},
	}

	cmd.Flags().StringVar(&schemasDir, "schemas-dir", "schemas", "Path to JSON Schema directory")

	return cmd
}
