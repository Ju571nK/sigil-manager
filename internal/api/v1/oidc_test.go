package v1

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/Ju571nK/sigil-manager/internal/auth"
	"github.com/Ju571nK/sigil-manager/internal/testutil"
)

func TestOIDCRoutesSessionAndFailure(t *testing.T) {
	p := testutil.NewOIDCProvider(t)
	// Trust only the local fixture's TLS certificate while constructing the client.
	previous := http.DefaultTransport
	http.DefaultTransport = p.Server.Client().Transport
	provider, err := auth.NewOIDC(context.Background(), auth.OIDCConfig{Issuer: p.Server.URL, ClientID: "console", ClientSecret: "test-secret", RedirectURL: "https://console.example/api/v1/auth/oidc/callback", AllowedSubjects: []string{"employee-1"}})
	http.DefaultTransport = previous
	require.NoError(t, err)
	signer, err := auth.NewSigner(testJWTSecret, 12*time.Hour)
	require.NoError(t, err)
	s := &Server{OIDC: provider, Signer: signer, Auth: AuthConfig{CookieSecure: true}}
	router := s.Routes()
	start := httptest.NewRecorder()
	router.ServeHTTP(start, httptest.NewRequest("GET", "/auth/oidc/start", nil))
	require.Equal(t, 302, start.Code)
	require.Equal(t, "no-store", start.Header().Get("Cache-Control"))
	binding := start.Result().Cookies()[0]
	require.True(t, binding.Secure)
	require.True(t, binding.HttpOnly)
	require.Equal(t, http.SameSiteLaxMode, binding.SameSite)
	client := p.Server.Client()
	client.CheckRedirect = func(_ *http.Request, _ []*http.Request) error { return http.ErrUseLastResponse }
	response, err := client.Get(start.Header().Get("Location"))
	require.NoError(t, err)
	_ = response.Body.Close()
	callback, err := url.Parse(response.Header.Get("Location"))
	require.NoError(t, err)
	invoke := func(cookie *http.Cookie) *httptest.ResponseRecorder {
		r := httptest.NewRequest("GET", "/auth/oidc/callback?"+callback.RawQuery, nil)
		if cookie != nil {
			r.AddCookie(cookie)
		}
		out := httptest.NewRecorder()
		router.ServeHTTP(out, r)
		return out
	}
	missing := invoke(nil)
	require.Equal(t, "/login?oidc_error=1", missing.Header().Get("Location"))
	success := invoke(binding)
	require.Equal(t, "/alerts", success.Header().Get("Location"))
	var session *http.Cookie
	for _, cookie := range success.Result().Cookies() {
		if cookie.Name == CookieName {
			session = cookie
		}
	}
	require.NotNil(t, session)
	require.True(t, session.HttpOnly)
	require.True(t, session.Secure)
	require.LessOrEqual(t, session.MaxAge, 3600)
	subject, err := signer.Verify(session.Value)
	require.NoError(t, err)
	require.Contains(t, subject, "oidc:")
	req := httptest.NewRequest("GET", "/auth/me", nil)
	req.AddCookie(session)
	me := httptest.NewRecorder()
	router.ServeHTTP(me, req)
	require.Equal(t, 200, me.Code)
	require.Contains(t, me.Body.String(), subject)
	replay := invoke(binding)
	require.Equal(t, "/login?oidc_error=1", replay.Header().Get("Location"))
}

func TestOIDCDisabled(t *testing.T) {
	h := newHarness(t)
	status, body, _ := h.do("GET", "/auth/methods", nil, nil)
	require.Equal(t, 200, status)
	require.JSONEq(t, `{"local":true,"oidc":false}`, string(body))
	status, _, _ = h.do("GET", "/auth/oidc/start", nil, nil)
	require.Equal(t, 404, status)
}
