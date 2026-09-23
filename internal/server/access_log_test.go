package server

import (
	"bytes"
	"log"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestAccessLogOmitsOIDCCredentials(t *testing.T) {
	var output bytes.Buffer
	previous := log.Writer()
	log.SetOutput(&output)
	defer log.SetOutput(previous)
	handler := accessLog(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "secret-code", r.URL.Query().Get("code"))
		w.WriteHeader(http.StatusSeeOther)
	}))
	handler.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest("GET", "/api/v1/auth/oidc/callback?code=secret-code&state=secret-state", nil))
	require.Contains(t, output.String(), "callback")
	require.NotContains(t, output.String(), "secret-code")
	require.NotContains(t, output.String(), "secret-state")
}
