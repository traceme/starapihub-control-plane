package api

import (
	"crypto/subtle"
	"io/fs"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/starapihub/dashboard/internal/poller"
	"github.com/starapihub/dashboard/internal/store"
	"github.com/starapihub/dashboard/internal/upstream"
)

// Handler holds all dependencies for API handlers.
type Handler struct {
	state          *poller.SystemState
	store          *store.Store
	newAPI         *upstream.NewAPIClient
	bifrost        *upstream.BifrostClient
	clewdr         *upstream.ClewdRClient
	clewdrURLs     []string
	clewdrTokens   []string
	dashboardToken string
}

func NewHandler(
	state *poller.SystemState,
	store *store.Store,
	newAPI *upstream.NewAPIClient,
	bifrost *upstream.BifrostClient,
	clewdr *upstream.ClewdRClient,
	clewdrURLs []string,
	clewdrTokens []string,
	dashboardToken string,
) *Handler {
	return &Handler{
		state:          state,
		store:          store,
		newAPI:         newAPI,
		bifrost:        bifrost,
		clewdr:         clewdr,
		clewdrURLs:     clewdrURLs,
		clewdrTokens:   clewdrTokens,
		dashboardToken: dashboardToken,
	}
}

// NewRouter sets up the HTTP router with middleware and routes.
func NewRouter(h *Handler, token string, frontendFS fs.FS) http.Handler {
	mux := http.NewServeMux()

	// API routes (auth required)
	mux.Handle("GET /api/health", authMiddleware(token, http.HandlerFunc(h.HandleHealth)))
	mux.Handle("GET /api/cookies", authMiddleware(token, http.HandlerFunc(h.HandleCookies)))
	mux.Handle("GET /api/models", authMiddleware(token, http.HandlerFunc(h.HandleListModels)))
	mux.Handle("POST /api/models", authMiddleware(token, http.HandlerFunc(h.HandleCreateModel)))
	mux.Handle("PUT /api/models/{id}", authMiddleware(token, http.HandlerFunc(h.HandleUpdateModel)))
	mux.Handle("DELETE /api/models/{id}", authMiddleware(token, http.HandlerFunc(h.HandleDeleteModel)))
	mux.Handle("GET /api/logs", authMiddleware(token, http.HandlerFunc(h.HandleListLogs)))
	mux.Handle("GET /api/logs/{requestId}", authMiddleware(token, http.HandlerFunc(h.HandleGetLog)))
	mux.Handle("GET /api/alerts", authMiddleware(token, http.HandlerFunc(h.HandleListAlerts)))
	mux.Handle("POST /api/alerts/{id}/ack", authMiddleware(token, http.HandlerFunc(h.HandleAckAlert)))
	mux.Handle("POST /api/wizard/provider", authMiddleware(token, http.HandlerFunc(h.HandleWizardProvider)))
	mux.Handle("POST /api/wizard/model", authMiddleware(token, http.HandlerFunc(h.HandleWizardModel)))
	mux.Handle("POST /api/wizard/test", authMiddleware(token, http.HandlerFunc(h.HandleWizardTest)))
	mux.Handle("GET /api/wizard/status", authMiddleware(token, http.HandlerFunc(h.HandleWizardStatus)))
	mux.Handle("GET /api/sse", authMiddleware(token, http.HandlerFunc(h.HandleSSE)))

	// Version (no auth — helps operators verify the running instance)
	mux.Handle("GET /api/version", http.HandlerFunc(h.HandleVersion))

	// Ops: sync/diff/audit/bootstrap status (reads from CLI audit log)
	mux.Handle("GET /api/ops/sync", authMiddleware(token, http.HandlerFunc(h.HandleSyncStatus)))
	mux.Handle("GET /api/ops/diff", authMiddleware(token, http.HandlerFunc(h.HandleDiffStatus)))
	mux.Handle("GET /api/ops/audit", authMiddleware(token, http.HandlerFunc(h.HandleAuditLog)))
	mux.Handle("GET /api/ops/bootstrap", authMiddleware(token, http.HandlerFunc(h.HandleBootstrapStatus)))

	// Static frontend files
	if frontendFS != nil {
		fileServer := http.FileServer(http.FS(frontendFS))
		mux.HandleFunc("GET /", func(w http.ResponseWriter, r *http.Request) {
			// SPA fallback: serve index.html for non-file paths
			path := r.URL.Path
			if path != "/" && !strings.Contains(path, ".") {
				r.URL.Path = "/"
			}
			fileServer.ServeHTTP(w, r)
		})
	} else {
		slog.Warn("no frontend filesystem provided, serving 404 for /")
		mux.HandleFunc("GET /", func(w http.ResponseWriter, r *http.Request) {
			http.Error(w, "frontend not built", http.StatusNotFound)
		})
	}

	return loggingMiddleware(mux)
}

func authMiddleware(token string, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		auth := r.Header.Get("Authorization")
		if auth == "" {
			w.Header().Set("Content-Type", "application/json")
			http.Error(w, `{"error":"missing authorization header"}`, http.StatusUnauthorized)
			return
		}
		if !strings.HasPrefix(auth, "Bearer ") {
			w.Header().Set("Content-Type", "application/json")
			http.Error(w, `{"error":"invalid authorization format"}`, http.StatusUnauthorized)
			return
		}
		provided := strings.TrimPrefix(auth, "Bearer ")
		if subtle.ConstantTimeCompare([]byte(provided), []byte(token)) != 1 {
			w.Header().Set("Content-Type", "application/json")
			http.Error(w, `{"error":"invalid token"}`, http.StatusForbidden)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func loggingMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		rw := &responseWriter{ResponseWriter: w, statusCode: http.StatusOK}
		next.ServeHTTP(rw, r)
		slog.Info("request",
			"method", r.Method,
			"path", r.URL.Path,
			"status", rw.statusCode,
			"duration_ms", time.Since(start).Milliseconds(),
		)
	})
}

type responseWriter struct {
	http.ResponseWriter
	statusCode int
}

func (rw *responseWriter) WriteHeader(code int) {
	rw.statusCode = code
	rw.ResponseWriter.WriteHeader(code)
}

// Flush implements http.Flusher by delegating to the underlying ResponseWriter.
func (rw *responseWriter) Flush() {
	if f, ok := rw.ResponseWriter.(http.Flusher); ok {
		f.Flush()
	}
}

// Unwrap allows http.ResponseController to find the underlying ResponseWriter.
func (rw *responseWriter) Unwrap() http.ResponseWriter {
	return rw.ResponseWriter
}
