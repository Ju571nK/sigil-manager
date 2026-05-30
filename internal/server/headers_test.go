package server

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
)

// NewRouter(nil) still serves /api/health + the SPA, and the securityHeaders
// middleware runs on every response — so a health probe is enough to assert
// the headers are on the wire.
func TestSecurityHeaders_PresentOnEveryResponse(t *testing.T) {
	srv := httptest.NewServer(NewRouter(nil))
	defer srv.Close()

	resp, err := http.Get(srv.URL + "/api/health")
	assert.NoError(t, err)
	defer func() { _ = resp.Body.Close() }()

	h := resp.Header
	assert.Equal(t, "DENY", h.Get("X-Frame-Options"))
	assert.Equal(t, "nosniff", h.Get("X-Content-Type-Options"))
	assert.Equal(t, "no-referrer", h.Get("Referrer-Policy"))

	csp := h.Get("Content-Security-Policy")
	assert.Contains(t, csp, "default-src 'self'")
	assert.Contains(t, csp, "frame-ancestors 'none'")
	assert.Contains(t, csp, "object-src 'none'")
	// HSTS is deliberately NOT asserted here — it belongs to the TLS terminator.
	assert.Empty(t, h.Get("Strict-Transport-Security"))
}
