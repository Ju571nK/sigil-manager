package api

import (
	"net/http"
	"time"

	"github.com/Ju571nK/sigil-manager/internal/httputil"
)

// Version is the build version string, injected at link time via -ldflags.
var Version = "dev"

type healthResponse struct {
	Status    string `json:"status"`
	Version   string `json:"version"`
	Timestamp string `json:"timestamp"`
}

// HealthHandler returns a JSON health-check response with status, version, and timestamp.
func HealthHandler(w http.ResponseWriter, r *http.Request) {
	httputil.WriteJSON(w, http.StatusOK, healthResponse{
		Status:    "ok",
		Version:   Version,
		Timestamp: time.Now().UTC().Format(time.RFC3339),
	})
}
