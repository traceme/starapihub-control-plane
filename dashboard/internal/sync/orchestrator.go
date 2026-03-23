package sync

import (
	"fmt"
	"log"
	"strings"
)

// SyncOptions configures orchestrator behavior.
type SyncOptions struct {
	DryRun   bool
	Prune    bool
	FailFast bool
	Verbose  bool
	Targets  []string // empty = all, or subset of: "cookie", "provider", "config", "routing-rule", "channel", "pricing"
	Output   string   // "text" or "json"
}

// ValidTargets is the set of recognized reconciler names.
var ValidTargets = map[string]string{
	"cookie":       "cookie",
	"cookies":      "cookie",
	"provider":     "provider",
	"providers":    "provider",
	"config":       "config",
	"routing-rule": "routing-rule",
	"routing-rules": "routing-rule",
	"channel":      "channel",
	"channels":     "channel",
	"pricing":      "pricing",
}

// NormalizeTargets maps user-facing target names (including plurals) to canonical
// reconciler names. Returns an error listing any unrecognized targets.
func NormalizeTargets(targets []string) ([]string, error) {
	if len(targets) == 0 {
		return nil, nil
	}
	var normalized []string
	var unknown []string
	seen := make(map[string]bool)
	for _, t := range targets {
		canonical, ok := ValidTargets[t]
		if !ok {
			unknown = append(unknown, t)
			continue
		}
		if !seen[canonical] {
			normalized = append(normalized, canonical)
			seen[canonical] = true
		}
	}
	if len(unknown) > 0 {
		valid := []string{"cookie", "provider", "config", "routing-rule", "channel", "pricing"}
		return nil, fmt.Errorf("unknown target(s): %s (valid: %s)", strings.Join(unknown, ", "), strings.Join(valid, ", "))
	}
	return normalized, nil
}

// SyncOrchestrator runs reconcilers in dependency order and produces a SyncReport.
type SyncOrchestrator struct {
	reconcilers  []Reconciler
	options      SyncOptions
	liveState    map[string]any // keyed by reconciler name
	desiredState map[string]any // keyed by reconciler name
}

// NewSyncOrchestrator creates an orchestrator with the given reconcilers (already in dependency order),
// options, and pre-fetched live/desired state maps keyed by reconciler name.
func NewSyncOrchestrator(reconcilers []Reconciler, opts SyncOptions, desiredState, liveState map[string]any) *SyncOrchestrator {
	// Filter by targets if specified
	var filtered []Reconciler
	if len(opts.Targets) > 0 {
		targetSet := make(map[string]struct{}, len(opts.Targets))
		for _, t := range opts.Targets {
			targetSet[t] = struct{}{}
		}
		for _, r := range reconcilers {
			if _, ok := targetSet[r.Name()]; ok {
				filtered = append(filtered, r)
			}
		}
	} else {
		filtered = reconcilers
	}

	return &SyncOrchestrator{
		reconcilers:  filtered,
		options:      opts,
		liveState:    liveState,
		desiredState: desiredState,
	}
}

// Run executes all reconcilers in order and returns a SyncReport.
// Returns a fatal error only for unrecoverable situations (exit code 2).
// Individual reconciler failures are captured in the report (exit code 1).
func (o *SyncOrchestrator) Run() (*SyncReport, error) {
	report := &SyncReport{}

	for _, rec := range o.reconcilers {
		name := rec.Name()

		// Fetch desired and live state for this reconciler
		desired := o.desiredState[name]
		live := o.liveState[name]

		// Plan
		actions, err := rec.Plan(desired, live)
		if err != nil {
			if o.options.Verbose {
				log.Printf("[%s] Plan error: %v", name, err)
			}
			// Record as a failed result
			report.Results = append(report.Results, Result{
				Action: Action{ResourceType: name, ResourceID: "plan-error"},
				Status: StatusFailed,
				Error:  fmt.Errorf("plan %s: %w", name, err),
			})
			report.TotalActions++
			report.Failed++
			if o.options.FailFast {
				return report, nil
			}
			continue
		}

		if len(actions) == 0 {
			continue
		}

		// Dry-run: record all actions as skipped
		if o.options.DryRun {
			for _, action := range actions {
				report.Results = append(report.Results, Result{
					Action: action,
					Status: StatusSkipped,
				})
			}
			report.TotalActions += len(actions)
			report.Skipped += len(actions)
			continue
		}

		// Apply and verify each action
		for _, action := range actions {
			report.TotalActions++

			result, err := rec.Apply(action)
			if err != nil {
				// Apply returned an error (not a result with StatusFailed)
				result = &Result{Action: action, Status: StatusFailed, Error: err}
			}

			if result.Status == StatusFailed {
				report.Results = append(report.Results, *result)
				report.Failed++
				if o.options.FailFast {
					return report, nil
				}
				continue
			}

			// Verify
			if err := rec.Verify(action, result); err != nil {
				result.Status = StatusUnverified
				result.DriftMsg = fmt.Sprintf("verify error: %v", err)
			}

			// Tally by status
			switch result.Status {
			case StatusOK:
				report.Succeeded++
			case StatusAppliedWithDrift:
				report.DriftWarnings++
			case StatusUnverified:
				report.DriftWarnings++
			case StatusFailed:
				report.Failed++
			case StatusSkipped:
				report.Skipped++
			}

			report.Results = append(report.Results, *result)
		}
	}

	return report, nil
}

// ExitCode returns the appropriate exit code for a SyncReport.
// 0 = all succeeded, 1 = some failed, 2 = reserved for fatal errors.
func ExitCode(report *SyncReport) int {
	if report.Failed > 0 {
		return 1
	}
	return 0
}
