package poller

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"sync"
	"time"

	"github.com/starapihub/dashboard/internal/store"
)

// alertMu protects the check-then-insert in fireAlert to prevent duplicate alerts.
var alertMu sync.Mutex

// StartAlertChecker runs after each poll cycle to check for alert conditions.
func StartAlertChecker(ctx context.Context, cfg Config) {
	go func() {
		ticker := time.NewTicker(15 * time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				checkAlerts(cfg)
			}
		}
	}()
}

func checkAlerts(cfg Config) {
	// Check ClewdR cookie alerts
	cookies := cfg.State.GetCookies()
	for instance, cs := range cookies {
		if cs.Valid == 0 && cs.Total > 0 {
			fireAlert(cfg, store.Alert{
				Type:      "CRITICAL",
				Service:   instance,
				Message:   fmt.Sprintf("%s has 0 valid cookies out of %d total", instance, cs.Total),
				Timestamp: time.Now(),
			})
		}
		if cs.HighUtil > 0 {
			fireAlert(cfg, store.Alert{
				Type:      "WARNING",
				Service:   instance,
				Message:   fmt.Sprintf("%s has %d cookies with >80%% utilization", instance, cs.HighUtil),
				Timestamp: time.Now(),
			})
		}
	}

	// Check service health alerts
	health := cfg.State.GetHealth()
	for name, h := range health {
		if h.Status == "unhealthy" {
			since, ok := cfg.State.GetUnhealthySince(name)
			if ok && time.Since(since) > 30*time.Second {
				fireAlert(cfg, store.Alert{
					Type:      "WARNING",
					Service:   name,
					Message:   fmt.Sprintf("%s has been unhealthy for %s", name, time.Since(since).Round(time.Second)),
					Timestamp: time.Now(),
				})
			}
		}
	}
}

func fireAlert(cfg Config, alert store.Alert) {
	// Serialize check-then-insert to prevent duplicate alerts from concurrent callers.
	alertMu.Lock()
	defer alertMu.Unlock()

	// Deduplicate: don't fire the same alert type+service within 5 minutes
	recent, err := cfg.Store.HasRecentAlert(alert.Type, alert.Service, 5*time.Minute)
	if err != nil {
		slog.Error("check recent alert", "error", err)
		return
	}
	if recent {
		return
	}

	id, err := cfg.Store.InsertAlert(alert)
	if err != nil {
		slog.Error("insert alert", "error", err)
		return
	}

	slog.Warn("alert fired", "id", id, "type", alert.Type, "service", alert.Service, "message", alert.Message)

	// Send webhook if configured
	if cfg.AlertWebhookURL != "" {
		go sendWebhook(cfg.AlertWebhookURL, alert)
	}
}

func sendWebhook(url string, alert store.Alert) {
	payload, err := json.Marshal(alert)
	if err != nil {
		slog.Error("marshal alert for webhook", "error", err)
		return
	}

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Post(url, "application/json", bytes.NewReader(payload))
	if err != nil {
		slog.Error("send alert webhook", "error", err, "url", url)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 300 {
		slog.Warn("alert webhook non-2xx response", "status", resp.StatusCode, "url", url)
	}
}
