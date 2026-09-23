package auth

import (
	"context"
	"net/http"
	"net/url"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/stretchr/testify/require"

	"github.com/Ju571nK/sigil-manager/internal/testutil"
)

func TestOIDCFlow(t *testing.T) {
	for _, tc := range []struct {
		name   string
		mutate func(jwt.MapClaims)
		denied bool
	}{
		{name: "standard provider"},
		{name: "unknown subject", mutate: func(c jwt.MapClaims) { c["sub"] = "outsider" }, denied: true},
		{name: "wrong nonce", mutate: func(c jwt.MapClaims) { c["nonce"] = "wrong" }, denied: true},
		{name: "wrong issuer", mutate: func(c jwt.MapClaims) { c["iss"] = "https://other.example" }, denied: true},
		{name: "wrong audience", mutate: func(c jwt.MapClaims) { c["aud"] = "other" }, denied: true},
		{name: "expired", mutate: func(c jwt.MapClaims) { c["exp"] = time.Now().Add(-time.Hour).Unix() }, denied: true},
		{name: "wrong authorized party", mutate: func(c jwt.MapClaims) { c["azp"] = "other" }, denied: true},
		{name: "multiple audiences without azp", mutate: func(c jwt.MapClaims) { c["aud"] = []string{"console", "other"} }, denied: true},
		{name: "multiple audiences with azp", mutate: func(c jwt.MapClaims) { c["aud"] = []string{"console", "other"}; c["azp"] = "console" }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			p := testutil.NewOIDCProvider(t)
			p.Mutate = tc.mutate
			client := p.Server.Client()
			client.CheckRedirect = func(_ *http.Request, _ []*http.Request) error { return http.ErrUseLastResponse }
			c := OIDCConfig{Issuer: p.Server.URL, ClientID: "console", ClientSecret: "test-secret", RedirectURL: "https://console.example/api/v1/auth/oidc/callback", AllowedSubjects: []string{"employee-1"}}
			o, err := discoverOIDC(context.Background(), c, client)
			require.NoError(t, err)
			location, binding, err := o.Begin()
			require.NoError(t, err)
			resp, err := client.Get(location)
			require.NoError(t, err)
			_ = resp.Body.Close()
			callback, err := url.Parse(resp.Header.Get("Location"))
			require.NoError(t, err)
			q := callback.Query()
			_, _, err = o.Complete(context.Background(), q.Get("state"), "wrong-browser", q.Get("code"))
			require.Error(t, err)
			subject, expiry, err := o.Complete(context.Background(), q.Get("state"), binding, q.Get("code"))
			if tc.denied {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
				require.Contains(t, subject, "oidc:")
				require.WithinDuration(t, time.Now().Add(time.Hour), expiry, time.Second*3)
			}
			_, _, err = o.Complete(context.Background(), q.Get("state"), binding, q.Get("code"))
			require.Error(t, err, "replay must fail")
		})
	}
}

func TestOIDCAttemptExpiryAndCapacity(t *testing.T) {
	p := testutil.NewOIDCProvider(t)
	o, err := discoverOIDC(context.Background(), OIDCConfig{Issuer: p.Server.URL}, p.Server.Client())
	require.NoError(t, err)
	location, binding, err := o.Begin()
	require.NoError(t, err)
	u, _ := url.Parse(location)
	o.now = func() time.Time { return time.Now().Add(LoginAttemptTTL + time.Second) }
	_, err = o.consume(u.Query().Get("state"), binding)
	require.Error(t, err)
	for range maxLoginAttempts {
		_, _, err = o.Begin()
		require.NoError(t, err)
	}
	_, _, err = o.Begin()
	require.Error(t, err)
	o.now = func() time.Time { return time.Now().Add(2*LoginAttemptTTL + time.Minute) }
	_, _, err = o.Begin()
	require.NoError(t, err)
}

func TestOIDCConfig(t *testing.T) {
	valid := OIDCConfig{Issuer: "https://id.example/realm", ClientID: "console", ClientSecret: "secret", RedirectURL: "https://console.example/api/v1/auth/oidc/callback", AllowedSubjects: []string{"subject"}}
	require.NoError(t, valid.Validate())
	require.NoError(t, (OIDCConfig{}).Validate())
	for _, change := range []func(*OIDCConfig){func(c *OIDCConfig) { c.Issuer = "http://id.example" }, func(c *OIDCConfig) { c.AllowedSubjects = nil }, func(c *OIDCConfig) { c.AllowedSubjects = []string{""} }, func(c *OIDCConfig) { c.RedirectURL = "https://console.example/other" }, func(c *OIDCConfig) { c.Issuer = "https://user:pass@id.example" }, func(c *OIDCConfig) { c.ClientSecret = "" }} {
		c := valid
		change(&c)
		require.Error(t, c.Validate())
	}
}

func TestSignUntilCapsSession(t *testing.T) {
	signer, err := NewSigner("0123456789abcdefghijklmnopqrstuv", 12*time.Hour)
	require.NoError(t, err)
	deadline := time.Now().Add(time.Minute).Truncate(time.Second)
	token, exp, err := signer.SignUntil("oidc:subject", deadline)
	require.NoError(t, err)
	require.Equal(t, deadline, exp)
	_, exp, err = signer.VerifyWithExp(token)
	require.NoError(t, err)
	require.Equal(t, deadline, exp)
	_, _, err = signer.SignUntil("oidc:subject", time.Now().Add(-time.Second))
	require.Error(t, err)
}

func TestOIDCRejectsInvalidSignature(t *testing.T) {
	p := testutil.NewOIDCProvider(t)
	p.InvalidSignature = true
	client := p.Server.Client()
	client.CheckRedirect = func(_ *http.Request, _ []*http.Request) error { return http.ErrUseLastResponse }
	o, err := discoverOIDC(context.Background(), OIDCConfig{Issuer: p.Server.URL, ClientID: "console", ClientSecret: "test-secret", RedirectURL: "https://console.example/api/v1/auth/oidc/callback", AllowedSubjects: []string{"employee-1"}}, client)
	require.NoError(t, err)
	location, binding, err := o.Begin()
	require.NoError(t, err)
	resp, err := client.Get(location)
	require.NoError(t, err)
	_ = resp.Body.Close()
	u, _ := url.Parse(resp.Header.Get("Location"))
	_, _, err = o.Complete(context.Background(), u.Query().Get("state"), binding, u.Query().Get("code"))
	require.Error(t, err)
}

func TestOIDCRejectsCleartextEndpoints(t *testing.T) {
	r, _ := http.NewRequest("GET", "http://identity.example/keys", nil)
	_, err := (httpsTransport{http.DefaultTransport}).RoundTrip(r)
	require.ErrorContains(t, err, "HTTPS")
}
