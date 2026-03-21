package main

import (
	"context"
	"io/fs"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/starapihub/dashboard/internal/api"
	"github.com/starapihub/dashboard/internal/poller"
	"github.com/starapihub/dashboard/internal/store"
	"github.com/starapihub/dashboard/internal/upstream"
)

func main() {
	slog.SetDefault(slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo})))

	token := os.Getenv("DASHBOARD_TOKEN")
	if token == "" {
		slog.Error("DASHBOARD_TOKEN is required")
		os.Exit(1)
	}

	port := envOrDefault("DASHBOARD_PORT", "8090")
	newAPIURL := envOrDefault("NEWAPI_URL", "http://new-api:3000")
	bifrostURL := envOrDefault("BIFROST_URL", "http://bifrost:8080")
	clewdrURLsRaw := envOrDefault("CLEWDR_URLS", "http://clewdr-1:8484,http://clewdr-2:8484,http://clewdr-3:8484")
	clewdrTokensRaw := os.Getenv("CLEWDR_ADMIN_TOKENS")
	nginxLogPath := envOrDefault("NGINX_LOG_PATH", "/var/log/nginx/access.log")
	alertWebhookURL := os.Getenv("ALERT_WEBHOOK_URL")
	dbPath := envOrDefault("DB_PATH", "./data/dashboard.db")

	clewdrURLs := splitCSV(clewdrURLsRaw)
	clewdrTokens := splitCSV(clewdrTokensRaw)

	// Initialize SQLite store
	db, err := store.New(dbPath)
	if err != nil {
		slog.Error("failed to initialize store", "error", err)
		os.Exit(1)
	}
	defer db.Close()

	// Initialize upstream clients
	httpClient := upstream.NewHTTPClient()
	newAPIClient := upstream.NewNewAPIClient(httpClient, newAPIURL)
	bifrostClient := upstream.NewBifrostClient(httpClient, bifrostURL)
	clewdrClient := upstream.NewClewdRClient(httpClient)

	// Initialize system state
	state := poller.NewSystemState()

	// Build poller config
	pollCfg := poller.Config{
		NewAPIClient:    newAPIClient,
		BifrostClient:   bifrostClient,
		ClewdRClient:    clewdrClient,
		ClewdRURLs:      clewdrURLs,
		ClewdRTokens:    clewdrTokens,
		NginxLogPath:    nginxLogPath,
		AlertWebhookURL: alertWebhookURL,
		Store:           db,
		State:           state,
	}

	// Start pollers
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	poller.StartHealthPoller(ctx, pollCfg)
	poller.StartCookiePoller(ctx, pollCfg)
	poller.StartLogTailer(ctx, pollCfg)
	poller.StartAlertChecker(ctx, pollCfg)

	// Start cleanup goroutine
	go func() {
		ticker := time.NewTicker(1 * time.Hour)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				if err := db.CleanupOldLogs(24 * time.Hour); err != nil {
					slog.Error("failed to cleanup old logs", "error", err)
				}
			}
		}
	}()

	// Build HTTP handler
	handler := api.NewHandler(state, db, newAPIClient, bifrostClient, clewdrClient, clewdrURLs, clewdrTokens, token)

	// Prepare embedded frontend filesystem
	var staticFS fs.FS
	if sub, fsErr := fs.Sub(frontendFS, "frontend/dist"); fsErr != nil {
		slog.Warn("frontend embed not available", "error", fsErr)
	} else {
		staticFS = sub
	}

	router := api.NewRouter(handler, token, staticFS)

	srv := &http.Server{
		Addr:         ":" + port,
		Handler:      router,
		ReadTimeout:  30 * time.Second,
		WriteTimeout: 60 * time.Second,
		IdleTimeout:  120 * time.Second,
	}

	// Graceful shutdown
	go func() {
		sigCh := make(chan os.Signal, 1)
		signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
		<-sigCh
		slog.Info("shutting down")
		cancel()
		shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer shutdownCancel()
		if err := srv.Shutdown(shutdownCtx); err != nil {
			slog.Error("shutdown error", "error", err)
		}
	}()

	slog.Info("starting dashboard server", "port", port)
	if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		slog.Error("server error", "error", err)
		os.Exit(1)
	}
	slog.Info("server stopped")
}

func envOrDefault(key, defaultVal string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return defaultVal
}

func splitCSV(s string) []string {
	if s == "" {
		return nil
	}
	parts := strings.Split(s, ",")
	result := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			result = append(result, p)
		}
	}
	return result
}

