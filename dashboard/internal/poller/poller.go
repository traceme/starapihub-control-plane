package poller

import (
	"bufio"
	"context"
	"encoding/json"
	"log/slog"
	"math"
	"os"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/starapihub/dashboard/internal/store"
	"github.com/starapihub/dashboard/internal/upstream"
)

// Config holds all dependencies for pollers.
type Config struct {
	NewAPIClient    *upstream.NewAPIClient
	BifrostClient   *upstream.BifrostClient
	ClewdRClient    *upstream.ClewdRClient
	ClewdRURLs      []string
	ClewdRTokens    []string
	NginxLogPath    string
	AlertWebhookURL string
	Store           *store.Store
	State           *SystemState
}

// ServiceHealth represents the health of a single upstream service.
type ServiceHealth struct {
	Status    string    `json:"status"` // healthy, unhealthy, unknown
	URL       string    `json:"url"`
	LastCheck time.Time `json:"last_check"`
	Latency   int64     `json:"latency_ms"`
}

// CookieStatus represents cookie status for one ClewdR instance.
type CookieStatus struct {
	Valid     int       `json:"valid"`
	Exhausted int       `json:"exhausted"`
	Invalid   int       `json:"invalid"`
	Total     int       `json:"total"`
	HighUtil  int       `json:"high_utilization"`
	LastCheck time.Time `json:"last_check"`
}

// LogStats holds aggregated traffic stats.
type LogStats struct {
	RequestRate float64        `json:"request_rate"`
	P50Latency  float64        `json:"p50_latency_ms"`
	P99Latency  float64        `json:"p99_latency_ms"`
	ErrorRate   float64        `json:"error_rate"`
	ByModel     map[string]int `json:"by_model"`
	ByStatus    map[int]int    `json:"by_status"`
	Period      string         `json:"period"`
}

// Snapshot is a point-in-time copy of the entire system state.
type Snapshot struct {
	Health    map[string]ServiceHealth `json:"health"`
	Cookies   map[string]CookieStatus  `json:"cookies"`
	LogStats  LogStats                 `json:"log_stats"`
	Alerts    []store.Alert            `json:"alerts"`
	UpdatedAt time.Time                `json:"updated_at"`
}

// SystemState is the thread-safe shared state read by API handlers.
type SystemState struct {
	mu        sync.RWMutex
	Health    map[string]ServiceHealth
	Cookies   map[string]CookieStatus
	LogStats  LogStats
	Alerts    []store.Alert
	UpdatedAt time.Time

	// track when a service first became unhealthy (for alert thresholds)
	unhealthySince map[string]time.Time
}

func NewSystemState() *SystemState {
	return &SystemState{
		Health:         make(map[string]ServiceHealth),
		Cookies:        make(map[string]CookieStatus),
		LogStats:       LogStats{ByModel: make(map[string]int), ByStatus: make(map[int]int), Period: "60s"},
		unhealthySince: make(map[string]time.Time),
	}
}

func (s *SystemState) SetHealth(name string, h ServiceHealth) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.Health[name] = h
	s.UpdatedAt = time.Now()

	if h.Status == "unhealthy" {
		if _, ok := s.unhealthySince[name]; !ok {
			s.unhealthySince[name] = time.Now()
		}
	} else {
		delete(s.unhealthySince, name)
	}
}

func (s *SystemState) SetCookies(instance string, cs CookieStatus) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.Cookies[instance] = cs
	s.UpdatedAt = time.Now()
}

func (s *SystemState) AppendAlert(a store.Alert) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.Alerts = append(s.Alerts, a)
	const maxAlerts = 100
	if len(s.Alerts) > maxAlerts {
		s.Alerts = s.Alerts[len(s.Alerts)-maxAlerts:]
	}
	s.UpdatedAt = time.Now()
}

func (s *SystemState) SetLogStats(ls LogStats) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.LogStats = ls
	s.UpdatedAt = time.Now()
}

func (s *SystemState) GetSnapshot() Snapshot {
	s.mu.RLock()
	defer s.mu.RUnlock()

	health := make(map[string]ServiceHealth, len(s.Health))
	for k, v := range s.Health {
		health[k] = v
	}
	cookies := make(map[string]CookieStatus, len(s.Cookies))
	for k, v := range s.Cookies {
		cookies[k] = v
	}
	ls := s.LogStats
	ls.ByModel = copyMapStringInt(s.LogStats.ByModel)
	ls.ByStatus = copyMapIntInt(s.LogStats.ByStatus)

	alerts := make([]store.Alert, len(s.Alerts))
	copy(alerts, s.Alerts)

	return Snapshot{
		Health:    health,
		Cookies:   cookies,
		LogStats:  ls,
		Alerts:    alerts,
		UpdatedAt: s.UpdatedAt,
	}
}

func (s *SystemState) GetUnhealthySince(name string) (time.Time, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	t, ok := s.unhealthySince[name]
	return t, ok
}

func (s *SystemState) GetHealth() map[string]ServiceHealth {
	s.mu.RLock()
	defer s.mu.RUnlock()
	h := make(map[string]ServiceHealth, len(s.Health))
	for k, v := range s.Health {
		h[k] = v
	}
	return h
}

func (s *SystemState) GetCookies() map[string]CookieStatus {
	s.mu.RLock()
	defer s.mu.RUnlock()
	c := make(map[string]CookieStatus, len(s.Cookies))
	for k, v := range s.Cookies {
		c[k] = v
	}
	return c
}

func copyMapStringInt(m map[string]int) map[string]int {
	c := make(map[string]int, len(m))
	for k, v := range m {
		c[k] = v
	}
	return c
}

func copyMapIntInt(m map[int]int) map[int]int {
	c := make(map[int]int, len(m))
	for k, v := range m {
		c[k] = v
	}
	return c
}

// StartHealthPoller polls New-API, Bifrost health every 10s.
func StartHealthPoller(ctx context.Context, cfg Config) {
	go func() {
		// Poll once immediately
		pollHealth(cfg)

		ticker := time.NewTicker(10 * time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				pollHealth(cfg)
			}
		}
	}()
}

func pollHealth(cfg Config) {
	// New-API
	checkService(cfg, "new-api", func() (bool, error) {
		return cfg.NewAPIClient.CheckHealth()
	}, cfg.NewAPIClient.BaseURL())

	// Bifrost
	checkService(cfg, "bifrost", func() (bool, error) {
		return cfg.BifrostClient.CheckHealth()
	}, cfg.BifrostClient.BaseURL())

	// ClewdR instances
	for _, url := range cfg.ClewdRURLs {
		name := clewdrName(url)
		u := url
		checkService(cfg, name, func() (bool, error) {
			return cfg.ClewdRClient.CheckHealth(u)
		}, u)
	}
}

func checkService(cfg Config, name string, check func() (bool, error), url string) {
	start := time.Now()
	ok, err := check()
	latency := time.Since(start).Milliseconds()

	status := "healthy"
	if err != nil || !ok {
		status = "unhealthy"
		if err != nil {
			slog.Warn("health check failed", "service", name, "error", err)
		}
	}

	cfg.State.SetHealth(name, ServiceHealth{
		Status:    status,
		URL:       url,
		LastCheck: time.Now(),
		Latency:   latency,
	})
}

func clewdrName(url string) string {
	// Extract a short name from URL, e.g. "clewdr-1" from "http://clewdr-1:8484"
	url = strings.TrimPrefix(url, "http://")
	url = strings.TrimPrefix(url, "https://")
	parts := strings.Split(url, ":")
	return parts[0]
}

// StartCookiePoller polls ClewdR /api/cookies every 60s.
func StartCookiePoller(ctx context.Context, cfg Config) {
	go func() {
		pollCookies(cfg)

		ticker := time.NewTicker(60 * time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				pollCookies(cfg)
			}
		}
	}()
}

type cookieUtilization struct {
	SessionUtilization int `json:"session_utilization"`
}

func pollCookies(cfg Config) {
	for i, url := range cfg.ClewdRURLs {
		token := ""
		if i < len(cfg.ClewdRTokens) {
			token = cfg.ClewdRTokens[i]
		}
		name := clewdrName(url)

		cr, err := cfg.ClewdRClient.GetCookies(url, token)
		if err != nil {
			slog.Warn("cookie poll failed", "instance", name, "error", err)
			continue
		}

		highUtil := 0
		for _, raw := range cr.Valid {
			var cu cookieUtilization
			if err := json.Unmarshal(raw, &cu); err == nil {
				if cu.SessionUtilization > 80 {
					highUtil++
				}
			}
		}

		cs := CookieStatus{
			Valid:     len(cr.Valid),
			Exhausted: len(cr.Exhausted),
			Invalid:   len(cr.Invalid),
			Total:     len(cr.Valid) + len(cr.Exhausted) + len(cr.Invalid),
			HighUtil:  highUtil,
			LastCheck: time.Now(),
		}
		cfg.State.SetCookies(name, cs)
	}
}

// StartLogTailer tails the nginx access log every 5s.
func StartLogTailer(ctx context.Context, cfg Config) {
	go func() {
		var lastOffset int64
		var lastInode uint64
		var recentEntries []store.LogEntry

		// nginx log regex:
		// $remote_addr [$time_local] "$request" $status req_id=$req_id upstream=$upstream_response_time
		logRegex := regexp.MustCompile(
			`^(\S+)\s+\[([^\]]+)\]\s+"(\S+)\s+(\S+)\s+\S+"\s+(\d+)\s+req_id=(\S+)\s+upstream=(\S*)`,
		)

		ticker := time.NewTicker(5 * time.Second)
		defer ticker.Stop()

		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				entries, newOffset, newInode := tailLog(cfg.NginxLogPath, lastOffset, lastInode, logRegex)
				if newOffset > lastOffset || newInode != lastInode {
					lastOffset = newOffset
					lastInode = newInode
				}

				if len(entries) > 0 {
					if err := cfg.Store.InsertLogEntries(entries); err != nil {
						slog.Error("insert log entries", "error", err)
					}
				}

				// Keep a sliding window for stats (capped at 10,000 entries)
				now := time.Now()
				cutoff := now.Add(-60 * time.Second)
				recentEntries = append(recentEntries, entries...)

				// Trim old entries from window
				trimmed := recentEntries[:0]
				for _, e := range recentEntries {
					if e.Timestamp.After(cutoff) {
						trimmed = append(trimmed, e)
					}
				}
				recentEntries = trimmed

				// Hard cap to prevent unbounded memory growth
				const maxRecentEntries = 10000
				if len(recentEntries) > maxRecentEntries {
					recentEntries = recentEntries[len(recentEntries)-maxRecentEntries:]
				}

				stats := computeStats(recentEntries)
				cfg.State.SetLogStats(stats)
			}
		}
	}()
}

// tailLog reads new lines from the log file since the last offset.
// Detects log rotation via inode change and resets offset accordingly.
func tailLog(path string, offset int64, lastInode uint64, re *regexp.Regexp) ([]store.LogEntry, int64, uint64) {
	f, err := os.Open(path)
	if err != nil {
		// Not an error during startup when file doesn't exist
		return nil, offset, lastInode
	}
	defer f.Close()

	info, err := f.Stat()
	if err != nil {
		return nil, offset, lastInode
	}

	// Detect inode change (log rotation via rename + create)
	var currentInode uint64
	if stat, ok := info.Sys().(*syscall.Stat_t); ok {
		currentInode = stat.Ino
	}
	if lastInode != 0 && currentInode != lastInode {
		slog.Info("log file rotated (inode changed)", "old_inode", lastInode, "new_inode", currentInode)
		offset = 0
	}

	// If the file was truncated (log rotation via truncate), reset offset
	if info.Size() < offset {
		slog.Info("log file truncated", "old_offset", offset, "new_size", info.Size())
		offset = 0
	}

	if info.Size() == offset {
		return nil, offset, currentInode
	}

	if _, err := f.Seek(offset, 0); err != nil {
		return nil, offset, currentInode
	}

	var entries []store.LogEntry
	scanner := bufio.NewScanner(f)
	// 1MB max line size
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)

	for scanner.Scan() {
		line := scanner.Text()
		entry, ok := parseLine(line, re)
		if ok {
			entries = append(entries, entry)
		}
	}

	// After using bufio.Scanner, the file position may be past what was scanned
	// due to read-ahead buffering. Use file size as the offset since we read to EOF.
	return entries, info.Size(), currentInode
}

func parseLine(line string, re *regexp.Regexp) (store.LogEntry, bool) {
	matches := re.FindStringSubmatch(line)
	if matches == nil {
		return store.LogEntry{}, false
	}

	// matches: [full, remote_addr, time_local, method, path, status, req_id, upstream_time]
	status, _ := strconv.Atoi(matches[5])
	upstreamTime, _ := strconv.ParseFloat(matches[7], 64)

	// Parse time_local like "02/Jan/2006:15:04:05 +0000"
	ts, err := time.Parse("02/Jan/2006:15:04:05 -0700", matches[2])
	if err != nil {
		ts = time.Now()
	}

	// Detect model from path
	model := detectModel(matches[4])

	return store.LogEntry{
		Timestamp:    ts,
		Method:       matches[3],
		Path:         matches[4],
		Status:       status,
		LatencyMs:    upstreamTime * 1000, // upstream_time is in seconds
		RequestID:    matches[6],
		Model:        model,
		UpstreamTime: upstreamTime,
	}, true
}

func detectModel(path string) string {
	// Try to extract model from path segments like /v1/chat/completions
	// or /v1/models/claude-3-opus/...
	parts := strings.Split(path, "/")
	for i, p := range parts {
		if p == "models" && i+1 < len(parts) {
			return parts[i+1]
		}
	}
	// For /v1/chat/completions, we can't detect model from path alone
	return ""
}

func computeStats(entries []store.LogEntry) LogStats {
	stats := LogStats{
		ByModel:  make(map[string]int),
		ByStatus: make(map[int]int),
		Period:   "60s",
	}

	if len(entries) == 0 {
		return stats
	}

	var latencies []float64
	errorCount := 0

	for _, e := range entries {
		stats.ByStatus[e.Status]++
		if e.Model != "" {
			stats.ByModel[e.Model]++
		}
		latencies = append(latencies, e.LatencyMs)
		if e.Status >= 400 {
			errorCount++
		}
	}

	total := float64(len(entries))
	stats.RequestRate = total / 60.0
	stats.ErrorRate = float64(errorCount) / total

	sort.Float64s(latencies)
	stats.P50Latency = percentile(latencies, 0.50)
	stats.P99Latency = percentile(latencies, 0.99)

	return stats
}

func percentile(sorted []float64, p float64) float64 {
	if len(sorted) == 0 {
		return 0
	}
	idx := p * float64(len(sorted)-1)
	lower := int(math.Floor(idx))
	upper := int(math.Ceil(idx))
	if lower == upper || upper >= len(sorted) {
		return sorted[lower]
	}
	frac := idx - float64(lower)
	return sorted[lower]*(1-frac) + sorted[upper]*frac
}
