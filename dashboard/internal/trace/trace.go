package trace

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

// Layer names used for log source identification.
const (
	LayerNginx   = "nginx"
	LayerNewAPI  = "new-api"
	LayerBifrost = "bifrost"
	LayerClewdR  = "clewdr"
)

// allLayers is the ordered list of layers to search.
var allLayers = []string{LayerNginx, LayerNewAPI, LayerBifrost, LayerClewdR}

// LayerMatch represents a single log line match in one layer.
type LayerMatch struct {
	Layer     string            `json:"layer"`
	Timestamp string            `json:"timestamp,omitempty"`
	Line      string            `json:"line"`
	Metadata  map[string]string `json:"metadata,omitempty"`
}

// TraceResult aggregates matches across all layers.
type TraceResult struct {
	RequestID string       `json:"request_id"`
	Layers    []LayerMatch `json:"layers"`
}

// TraceOptions configures the trace operation.
type TraceOptions struct {
	RequestID      string
	ContainerNames []string // docker container names to search
	LogDir         string   // if set, read from files instead of docker logs
	Verbose        bool
}

// Tracer greps logs for a request ID across layers.
type Tracer struct {
	opts    TraceOptions
	execCmd func(name string, args ...string) ([]byte, error)
}

// NewTracer creates a Tracer with the given options.
func NewTracer(opts TraceOptions) *Tracer {
	return &Tracer{
		opts: opts,
		execCmd: func(name string, args ...string) ([]byte, error) {
			return exec.Command(name, args...).CombinedOutput()
		},
	}
}

// SetExecCmd allows injecting a mock command executor for testing.
func (t *Tracer) SetExecCmd(fn func(name string, args ...string) ([]byte, error)) {
	t.execCmd = fn
}

// Run executes the trace across all configured layers.
func (t *Tracer) Run() (*TraceResult, error) {
	result := &TraceResult{
		RequestID: t.opts.RequestID,
	}

	if t.opts.LogDir != "" {
		// File-based tracing
		for _, layer := range allLayers {
			logPath := filepath.Join(t.opts.LogDir, layer+".log")
			matches, err := t.searchFile(logPath, layer)
			if err != nil {
				// Missing file is not an error -- degrade gracefully
				continue
			}
			result.Layers = append(result.Layers, matches...)
		}
	} else {
		// Docker-based tracing
		containerToLayer := t.buildContainerMap()
		for container, layer := range containerToLayer {
			output, err := t.execCmd("docker", "logs", container)
			if err != nil {
				// Container not running or not found -- degrade gracefully
				continue
			}
			matches := t.searchLines(string(output), layer)
			result.Layers = append(result.Layers, matches...)
		}
	}

	return result, nil
}

// buildContainerMap maps container names to their layer names.
func (t *Tracer) buildContainerMap() map[string]string {
	m := make(map[string]string)
	for _, name := range t.opts.ContainerNames {
		switch {
		case strings.Contains(name, "nginx"):
			m[name] = LayerNginx
		case strings.Contains(name, "new-api"):
			m[name] = LayerNewAPI
		case strings.Contains(name, "bifrost"):
			m[name] = LayerBifrost
		case strings.Contains(name, "clewdr"):
			m[name] = LayerClewdR
		}
	}
	return m
}

// searchFile reads a log file and returns matches.
func (t *Tracer) searchFile(path, layer string) ([]LayerMatch, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	var matches []LayerMatch
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := scanner.Text()
		if strings.Contains(line, t.opts.RequestID) {
			match := LayerMatch{
				Layer:    layer,
				Line:     line,
				Metadata: make(map[string]string),
			}
			extractMetadata(&match, layer)
			matches = append(matches, match)
		}
	}
	return matches, scanner.Err()
}

// searchLines parses raw log text and returns matches.
func (t *Tracer) searchLines(text, layer string) []LayerMatch {
	var matches []LayerMatch
	for _, line := range strings.Split(text, "\n") {
		if line == "" {
			continue
		}
		if strings.Contains(line, t.opts.RequestID) {
			match := LayerMatch{
				Layer:    layer,
				Line:     line,
				Metadata: make(map[string]string),
			}
			extractMetadata(&match, layer)
			matches = append(matches, match)
		}
	}
	return matches
}

// --- Metadata extraction per layer ---

// Regex patterns for metadata extraction.
var (
	// Nginx patterns
	nginxTimestampRe   = regexp.MustCompile(`\[(\d{2}/\w+/\d{4}:\d{2}:\d{2}:\d{2})\s`)
	nginxStatusRe      = regexp.MustCompile(`"\s(\d{3})\s`)
	nginxUpstreamRe    = regexp.MustCompile(`upstream_time=(\S+)`)

	// New-API patterns
	newAPIUserIDRe  = regexp.MustCompile(`userId[=:]?\s*(\d+)`)
	newAPIModelRe   = regexp.MustCompile(`model[="]?\s*"?([^"\s,]+)`)
	newAPIChannelRe = regexp.MustCompile(`channel[=:]?\s*(\d+)`)

	// Bifrost patterns (JSON structured)
	bifrostProviderRe = regexp.MustCompile(`"provider"\s*:\s*"([^"]+)"`)
	bifrostModelRe    = regexp.MustCompile(`"model"\s*:\s*"([^"]+)"`)
	bifrostLatencyRe  = regexp.MustCompile(`"latency"\s*:\s*"?([^",}\s]+)`)

	// ClewdR patterns
	clewdrStatusRe = regexp.MustCompile(`"status"\s*:\s*(\d+)`)
	clewdrCookieRe = regexp.MustCompile(`cookie[=:]?\s*"?([^"\s,]+)`)

	// Generic JSON timestamp
	jsonTimestampRe = regexp.MustCompile(`"timestamp"\s*:\s*"([^"]+)"`)
)

func extractMetadata(m *LayerMatch, layer string) {
	line := m.Line

	switch layer {
	case LayerNginx:
		if ts := nginxTimestampRe.FindStringSubmatch(line); len(ts) > 1 {
			m.Timestamp = ts[1]
		}
		if st := nginxStatusRe.FindStringSubmatch(line); len(st) > 1 {
			m.Metadata["status"] = st[1]
		}
		if ut := nginxUpstreamRe.FindStringSubmatch(line); len(ut) > 1 {
			m.Metadata["upstream_time"] = ut[1]
		}

	case LayerNewAPI:
		if uid := newAPIUserIDRe.FindStringSubmatch(line); len(uid) > 1 {
			m.Metadata["user_id"] = uid[1]
		}
		if mo := newAPIModelRe.FindStringSubmatch(line); len(mo) > 1 {
			m.Metadata["model"] = mo[1]
		}
		if ch := newAPIChannelRe.FindStringSubmatch(line); len(ch) > 1 {
			m.Metadata["channel"] = ch[1]
		}
		// Try JSON timestamp first, then generic
		if ts := jsonTimestampRe.FindStringSubmatch(line); len(ts) > 1 {
			m.Timestamp = ts[1]
		}

	case LayerBifrost:
		if p := bifrostProviderRe.FindStringSubmatch(line); len(p) > 1 {
			m.Metadata["provider"] = p[1]
		}
		if mo := bifrostModelRe.FindStringSubmatch(line); len(mo) > 1 {
			m.Metadata["model"] = mo[1]
		}
		if lat := bifrostLatencyRe.FindStringSubmatch(line); len(lat) > 1 {
			m.Metadata["latency"] = lat[1]
		}
		if ts := jsonTimestampRe.FindStringSubmatch(line); len(ts) > 1 {
			m.Timestamp = ts[1]
		}

	case LayerClewdR:
		if st := clewdrStatusRe.FindStringSubmatch(line); len(st) > 1 {
			m.Metadata["status"] = st[1]
		}
		if ck := clewdrCookieRe.FindStringSubmatch(line); len(ck) > 1 {
			m.Metadata["cookie"] = ck[1]
		}
		if ts := jsonTimestampRe.FindStringSubmatch(line); len(ts) > 1 {
			m.Timestamp = ts[1]
		}
	}
}

// --- Formatting ---

// FormatTextTrace produces a human-readable table from a TraceResult.
// If verbose is true, the matched log line is also displayed.
func FormatTextTrace(result *TraceResult, verbose bool) string {
	var b strings.Builder

	fmt.Fprintf(&b, "Trace: %s\n\n", result.RequestID)

	if len(result.Layers) == 0 {
		b.WriteString("No matches found.\n")
		return b.String()
	}

	fmt.Fprintf(&b, "%-12s %-24s %s\n", "LAYER", "TIMESTAMP", "METADATA")
	fmt.Fprintf(&b, "%s\n", strings.Repeat("-", 72))

	for _, m := range result.Layers {
		ts := m.Timestamp
		if ts == "" {
			ts = "-"
		}
		meta := formatMetadata(m.Metadata)
		fmt.Fprintf(&b, "%-12s %-24s %s\n", m.Layer, ts, meta)
		if verbose && m.Line != "" {
			fmt.Fprintf(&b, "  >> %s\n", m.Line)
		}
	}

	return b.String()
}

// FormatJSONTrace produces indented JSON output from a TraceResult.
func FormatJSONTrace(result *TraceResult) ([]byte, error) {
	return json.MarshalIndent(result, "", "  ")
}

// formatMetadata sorts and formats metadata key=value pairs.
func formatMetadata(meta map[string]string) string {
	if len(meta) == 0 {
		return "-"
	}
	keys := make([]string, 0, len(meta))
	for k := range meta {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	parts := make([]string, 0, len(keys))
	for _, k := range keys {
		parts = append(parts, k+"="+meta[k])
	}
	return strings.Join(parts, " ")
}
