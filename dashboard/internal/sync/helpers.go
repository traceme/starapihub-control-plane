package sync

import (
	"fmt"
	"os"
)

// ResolveEnvVar reads an environment variable by name. Returns error if empty/unset.
func ResolveEnvVar(envName string) (string, error) {
	val := os.Getenv(envName)
	if val == "" {
		return "", fmt.Errorf("environment variable %s is not set or empty", envName)
	}
	return val, nil
}
