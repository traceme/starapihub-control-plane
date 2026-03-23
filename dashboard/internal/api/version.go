package api

import (
	"encoding/json"
	"net/http"
	"os"

	"github.com/starapihub/dashboard/internal/buildinfo"
)

// HandleVersion returns the appliance version and build metadata.
// This endpoint does not require authentication — version info
// helps operators verify the running instance.
func (h *Handler) HandleVersion(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(buildinfo.Info(os.Getenv))
}
