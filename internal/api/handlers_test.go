package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestHealthHandler_ReturnsOK(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/api/health", nil)
	rec := httptest.NewRecorder()

	HealthHandler(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Equal(t, "application/json", rec.Header().Get("Content-Type"))

	var body map[string]any
	err := json.Unmarshal(rec.Body.Bytes(), &body)
	assert.NoError(t, err)
	assert.Equal(t, "ok", body["status"])
	assert.Contains(t, body, "version")
	assert.Contains(t, body, "timestamp")
}
