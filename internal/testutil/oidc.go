// Package testutil provides a local TLS OIDC fixture for protocol integration tests.
package testutil

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"math/big"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/stretchr/testify/require"
)

// OIDCProvider implements discovery, PKCE authorization and signed ID tokens.
type OIDCProvider struct {
	Server *httptest.Server
	Key    *rsa.PrivateKey
	// Mutate is configured before requests begin, for negative token tests.
	Mutate           func(jwt.MapClaims)
	InvalidSignature bool
	mu               sync.Mutex
	codes            map[string]map[string]string
}

// NewOIDCProvider creates a TLS fixture and registers cleanup.
func NewOIDCProvider(t *testing.T) *OIDCProvider {
	t.Helper()
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)
	p := &OIDCProvider{Key: key, codes: make(map[string]map[string]string)}
	p.Server = httptest.NewTLSServer(http.HandlerFunc(p.serve))
	t.Cleanup(p.Server.Close)
	return p
}

func (p *OIDCProvider) serve(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	switch r.URL.Path {
	case "/.well-known/openid-configuration":
		_ = json.NewEncoder(w).Encode(map[string]any{"issuer": p.Server.URL, "authorization_endpoint": p.Server.URL + "/authorize", "token_endpoint": p.Server.URL + "/token", "jwks_uri": p.Server.URL + "/keys", "id_token_signing_alg_values_supported": []string{"RS256"}})
	case "/keys":
		_ = json.NewEncoder(w).Encode(map[string]any{"keys": []any{map[string]string{"kty": "RSA", "kid": "test-key", "alg": "RS256", "use": "sig", "n": base64.RawURLEncoding.EncodeToString(p.Key.N.Bytes()), "e": base64.RawURLEncoding.EncodeToString(big.NewInt(int64(p.Key.E)).Bytes())}}})
	case "/authorize":
		q := r.URL.Query()
		if q.Get("code_challenge_method") != "S256" || q.Get("nonce") == "" {
			http.Error(w, "PKCE/nonce required", 400)
			return
		}
		code := q.Get("state")
		p.mu.Lock()
		p.codes[code] = map[string]string{"nonce": q.Get("nonce"), "challenge": q.Get("code_challenge"), "redirect": q.Get("redirect_uri")}
		p.mu.Unlock()
		http.Redirect(w, r, q.Get("redirect_uri")+"?code="+code+"&state="+q.Get("state"), http.StatusFound)
	case "/token":
		_ = r.ParseForm()
		client, secret, ok := r.BasicAuth()
		if !ok {
			client, secret = r.Form.Get("client_id"), r.Form.Get("client_secret")
		}
		if client != "console" || secret != "test-secret" {
			http.Error(w, "invalid client", http.StatusUnauthorized)
			return
		}
		p.mu.Lock()
		a, ok := p.codes[r.Form.Get("code")]
		delete(p.codes, r.Form.Get("code"))
		p.mu.Unlock()
		digest := sha256.Sum256([]byte(r.Form.Get("code_verifier")))
		if !ok || a["challenge"] != base64.RawURLEncoding.EncodeToString(digest[:]) || a["redirect"] != r.Form.Get("redirect_uri") {
			http.Error(w, "invalid grant", 400)
			return
		}
		claims := jwt.MapClaims{"iss": p.Server.URL, "sub": "employee-1", "aud": "console", "exp": time.Now().Add(time.Hour).Unix(), "iat": time.Now().Unix(), "nonce": a["nonce"]}
		if p.Mutate != nil {
			p.Mutate(claims)
		}
		token := jwt.NewWithClaims(jwt.SigningMethodRS256, claims)
		token.Header["kid"] = "test-key"
		signingKey := p.Key
		if p.InvalidSignature {
			signingKey, _ = rsa.GenerateKey(rand.Reader, 2048)
		}
		signed, _ := token.SignedString(signingKey)
		_ = json.NewEncoder(w).Encode(map[string]any{"access_token": "opaque", "token_type": "Bearer", "id_token": signed})
	default:
		http.NotFound(w, r)
	}
}
