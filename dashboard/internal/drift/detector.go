package drift

import (
	"strings"
	"time"

	"github.com/starapihub/dashboard/internal/sync"
)

// DriftDetector classifies sync engine dry-run output into severity-tagged drift entries.
type DriftDetector struct{}

// NewDriftDetector creates a new DriftDetector instance.
func NewDriftDetector() *DriftDetector {
	return &DriftDetector{}
}

// diffSegment represents a single parsed field change from a Diff string.
type diffSegment struct {
	changeType string // "update", "create", "delete"
	path       string // dotted field path
	from       string // old value
	to         string // new value
}

// Detect takes a SyncReport (typically from a dry-run) and produces a
// severity-classified DriftReport.
func (d *DriftDetector) Detect(report *sync.SyncReport) *DriftReport {
	dr := &DriftReport{
		Entries:   []DriftEntry{},
		Timestamp: time.Now(),
	}

	// Track unique resources for summary
	allResources := make(map[string]struct{})
	driftedResources := make(map[string]struct{})

	for _, result := range report.Results {
		action := result.Action
		resKey := action.ResourceType + ":" + action.ResourceID
		allResources[resKey] = struct{}{}

		switch action.Type {
		case sync.ActionNoChange:
			// No drift -- skip
			continue

		case sync.ActionCreate:
			entry := DriftEntry{
				ResourceType: action.ResourceType,
				ResourceID:   action.ResourceID,
				ActionType:   string(action.Type),
				Field:        "(missing)",
				DesiredValue: "(to be created)",
				LiveValue:    "(absent)",
				Severity:     DefaultSeverityForAction(action.ResourceType, string(action.Type)),
			}
			dr.Entries = append(dr.Entries, entry)
			driftedResources[resKey] = struct{}{}

		case sync.ActionDelete:
			entry := DriftEntry{
				ResourceType: action.ResourceType,
				ResourceID:   action.ResourceID,
				ActionType:   string(action.Type),
				Field:        "(extra)",
				DesiredValue: "(absent)",
				LiveValue:    "(to be deleted)",
				Severity:     DefaultSeverityForAction(action.ResourceType, string(action.Type)),
			}
			dr.Entries = append(dr.Entries, entry)
			driftedResources[resKey] = struct{}{}

		case sync.ActionUpdate:
			segments := d.parseDiffSegments(action.Diff)
			if len(segments) == 0 {
				// Unparseable or empty diff -- single entry with default severity
				entry := DriftEntry{
					ResourceType: action.ResourceType,
					ResourceID:   action.ResourceID,
					ActionType:   string(action.Type),
					Field:        "(unknown)",
					DesiredValue: "",
					LiveValue:    "",
					Severity:     DefaultSeverityForAction(action.ResourceType, string(action.Type)),
				}
				dr.Entries = append(dr.Entries, entry)
				driftedResources[resKey] = struct{}{}
			} else {
				for _, seg := range segments {
					entry := DriftEntry{
						ResourceType: action.ResourceType,
						ResourceID:   action.ResourceID,
						ActionType:   string(action.Type),
						Field:        seg.path,
						DesiredValue: seg.to,
						LiveValue:    seg.from,
						Severity:     LookupFieldSeverity(action.ResourceType, seg.path),
					}
					dr.Entries = append(dr.Entries, entry)
				}
				driftedResources[resKey] = struct{}{}
			}
		}
	}

	// Compute summary
	dr.Summary = d.computeSummary(dr.Entries, len(allResources), len(driftedResources))
	return dr
}

// computeSummary aggregates entry counts and determines the verdict.
func (d *DriftDetector) computeSummary(entries []DriftEntry, total, drifted int) DriftSummary {
	summary := DriftSummary{
		TotalResources:   total,
		DriftedResources: drifted,
		Verdict:          VerdictClean,
	}

	for _, e := range entries {
		switch e.Severity {
		case SeverityInformational:
			summary.InformationalCount++
		case SeverityWarning:
			summary.WarningCount++
		case SeverityBlocking:
			summary.BlockingCount++
		}
	}

	if summary.BlockingCount > 0 {
		summary.Verdict = VerdictBlocking
	} else if summary.WarningCount > 0 {
		summary.Verdict = VerdictWarning
	} else if summary.InformationalCount > 0 {
		// Only informational entries -- still clean (no actionable drift)
		summary.Verdict = VerdictClean
	}

	return summary
}

// parseDiffSegments parses a Diff string into individual field change segments.
// Supports multiple formats from different reconcilers:
//
//   - Provider format: "update Keys.0.Models: [a] -> [a,b]; update Other: x -> y"
//   - Routing format: "routing rule ID:\n  field: old -> new\n  field: changed\n"
//   - Pricing format: "model: added=val, model: val->val"
//   - Config format: "changed fields: field1, field2"
//   - Channel format: "channel X differs from desired state" (no field-level parsing)
func (d *DriftDetector) parseDiffSegments(diff string) []diffSegment {
	diff = strings.TrimSpace(diff)
	if diff == "" {
		return nil
	}

	// Try provider/r3labs format first: "TYPE PATH: FROM -> TO; ..."
	if segments := d.parseProviderFormat(diff); len(segments) > 0 {
		return segments
	}

	// Try routing rule format: "routing rule ID:\n  field: val -> val\n"
	if segments := d.parseRoutingFormat(diff); len(segments) > 0 {
		return segments
	}

	// Try pricing format: "field: old->new, field: added=val"
	if segments := d.parsePricingFormat(diff); len(segments) > 0 {
		return segments
	}

	// Try config format: "changed fields: field1, field2"
	if segments := d.parseConfigFormat(diff); len(segments) > 0 {
		return segments
	}

	// Unparseable
	return nil
}

// parseProviderFormat handles "update Path: from -> to; update Path2: from2 -> to2"
func (d *DriftDetector) parseProviderFormat(diff string) []diffSegment {
	parts := strings.Split(diff, "; ")
	var segments []diffSegment

	for _, part := range parts {
		part = strings.TrimSpace(part)
		// Expected: "TYPE PATH: FROM -> TO"
		// Find first space to separate type from rest
		spaceIdx := strings.Index(part, " ")
		if spaceIdx < 0 {
			continue
		}
		changeType := part[:spaceIdx]
		if changeType != "update" && changeType != "create" && changeType != "delete" {
			return nil // Not provider format
		}
		rest := part[spaceIdx+1:]

		// Split on ": " to separate path from values
		colonIdx := strings.Index(rest, ": ")
		if colonIdx < 0 {
			continue
		}
		path := rest[:colonIdx]
		values := rest[colonIdx+2:]

		// Split on " -> " to get from/to
		arrowIdx := strings.Index(values, " -> ")
		if arrowIdx < 0 {
			segments = append(segments, diffSegment{
				changeType: changeType,
				path:       path,
				from:       values,
				to:         "",
			})
			continue
		}
		from := values[:arrowIdx]
		to := values[arrowIdx+4:]

		segments = append(segments, diffSegment{
			changeType: changeType,
			path:       path,
			from:       from,
			to:         to,
		})
	}

	return segments
}

// parseRoutingFormat handles "routing rule ID:\n  field: val -> val\n"
func (d *DriftDetector) parseRoutingFormat(diff string) []diffSegment {
	if !strings.HasPrefix(diff, "routing rule ") {
		return nil
	}

	lines := strings.Split(diff, "\n")
	var segments []diffSegment

	for _, line := range lines[1:] { // Skip header line
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		// "field: old -> new" or "field: changed"
		colonIdx := strings.Index(line, ": ")
		if colonIdx < 0 {
			continue
		}
		field := line[:colonIdx]
		values := line[colonIdx+2:]

		arrowIdx := strings.Index(values, " -> ")
		if arrowIdx >= 0 {
			segments = append(segments, diffSegment{
				changeType: "update",
				path:       field,
				from:       values[:arrowIdx],
				to:         values[arrowIdx+4:],
			})
		} else {
			segments = append(segments, diffSegment{
				changeType: "update",
				path:       field,
				from:       "",
				to:         values,
			})
		}
	}

	return segments
}

// parsePricingFormat handles "model: added=val, model: old->new"
func (d *DriftDetector) parsePricingFormat(diff string) []diffSegment {
	parts := strings.Split(diff, ", ")
	var segments []diffSegment

	for _, part := range parts {
		part = strings.TrimSpace(part)
		colonIdx := strings.Index(part, ": ")
		if colonIdx < 0 {
			return nil // Not pricing format
		}
		// For pricing, the field is the pricing ratio type, which we derive from context
		// The model name is the resource ID, so the "field" is the ratio description
		values := part[colonIdx+2:]

		if strings.Contains(values, "->") {
			arrowIdx := strings.Index(values, "->")
			segments = append(segments, diffSegment{
				changeType: "update",
				path:       part[:colonIdx],
				from:       strings.TrimSpace(values[:arrowIdx]),
				to:         strings.TrimSpace(values[arrowIdx+2:]),
			})
		} else if strings.HasPrefix(values, "added=") {
			segments = append(segments, diffSegment{
				changeType: "create",
				path:       part[:colonIdx],
				from:       "",
				to:         strings.TrimPrefix(values, "added="),
			})
		} else {
			return nil // Unknown pricing format
		}
	}

	return segments
}

// parseConfigFormat handles "changed fields: field1, field2"
func (d *DriftDetector) parseConfigFormat(diff string) []diffSegment {
	const prefix = "changed fields: "
	if !strings.HasPrefix(diff, prefix) {
		return nil
	}

	fieldList := diff[len(prefix):]
	fields := strings.Split(fieldList, ", ")
	var segments []diffSegment

	for _, f := range fields {
		f = strings.TrimSpace(f)
		if f == "" {
			continue
		}
		segments = append(segments, diffSegment{
			changeType: "update",
			path:       f,
			from:       "",
			to:         "",
		})
	}

	return segments
}

// FilterBySeverity returns entries with severity >= min.
func (r *DriftReport) FilterBySeverity(min DriftSeverity) []DriftEntry {
	minRank := severityOrder[min]
	var filtered []DriftEntry
	for _, e := range r.Entries {
		if severityOrder[e.Severity] >= minRank {
			filtered = append(filtered, e)
		}
	}
	return filtered
}

// HasBlocking returns true if there are any blocking-severity entries.
func (r *DriftReport) HasBlocking() bool {
	return r.Summary.BlockingCount > 0
}

// HasWarning returns true if there are any warning-severity entries.
func (r *DriftReport) HasWarning() bool {
	return r.Summary.WarningCount > 0
}
