package auth

import (
	"context"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/coreos/go-oidc/v3/oidc"
	"golang.org/x/oauth2"
)

// OIDCConfig configures one standard discovery-based provider, without vendor branches.
type OIDCConfig struct {
	Issuer, ClientID, ClientSecret, RedirectURL string
	AllowedSubjects                             []string
}

// Validate rejects incomplete or insecure provider configuration.
func (c OIDCConfig) Validate() error {
	if c.Issuer == "" && c.ClientID == "" && c.ClientSecret == "" && c.RedirectURL == "" && len(c.AllowedSubjects) == 0 {
		return nil
	}
	if c.Issuer == "" || c.ClientID == "" || c.ClientSecret == "" || c.RedirectURL == "" || len(c.AllowedSubjects) == 0 {
		return errors.New("OIDC requires OIDC_ISSUER_URL, OIDC_CLIENT_ID, OIDC_CLIENT_SECRET, OIDC_REDIRECT_URL and nonempty OIDC_ALLOWED_SUBJECTS")
	}
	for _, raw := range []string{c.Issuer, c.RedirectURL} {
		u, err := url.Parse(raw)
		if err != nil || u.Scheme != "https" || u.Host == "" || u.User != nil || u.RawQuery != "" || u.Fragment != "" {
			return errors.New("OIDC issuer and redirect must be absolute HTTPS URLs without credentials, query or fragment")
		}
	}
	redirect, _ := url.Parse(c.RedirectURL)
	if redirect.Path != "/api/v1/auth/oidc/callback" {
		return errors.New("OIDC_REDIRECT_URL path must be /api/v1/auth/oidc/callback")
	}
	for _, sub := range c.AllowedSubjects {
		if strings.TrimSpace(sub) == "" {
			return errors.New("OIDC_ALLOWED_SUBJECTS cannot contain empty subjects")
		}
	}
	return nil
}

type loginAttempt struct {
	binding, nonce, verifier string
	expires                  time.Time
}

// OIDC keeps short-lived, single-use login state server-side. A process restart
// invalidates outstanding logins. Instances need sticky routing for the callback.
type OIDC struct {
	config   OIDCConfig
	oauth    oauth2.Config
	verifier *oidc.IDTokenVerifier
	client   *http.Client
	mu       sync.Mutex
	attempts map[string]loginAttempt
	now      func() time.Time
}

// LoginAttemptTTL bounds how long an authorization callback can be completed.
const LoginAttemptTTL = 5 * time.Minute
const maxLoginAttempts = 1024

// ErrOIDCLogin intentionally hides provider errors and claims from the caller.
var ErrOIDCLogin = errors.New("OIDC login failed")

// NewOIDC discovers and validates the configured provider; disabled config returns nil.
func NewOIDC(ctx context.Context, c OIDCConfig) (*OIDC, error) {
	if err := c.Validate(); err != nil {
		return nil, err
	}
	if c.Issuer == "" {
		return nil, nil
	}
	client := &http.Client{Timeout: 10 * time.Second, Transport: httpsTransport{http.DefaultTransport}, CheckRedirect: func(_ *http.Request, _ []*http.Request) error { return http.ErrUseLastResponse }}
	return discoverOIDC(ctx, c, client)
}

// Discovery/JWKS/token traffic must not downgrade to cleartext, even if a
// misconfigured discovery document advertises insecure endpoints.
type httpsTransport struct{ base http.RoundTripper }

func (t httpsTransport) RoundTrip(r *http.Request) (*http.Response, error) {
	if r.URL.Scheme != "https" || r.URL.User != nil {
		return nil, errors.New("OIDC endpoint requires HTTPS")
	}
	return t.base.RoundTrip(r)
}

func discoverOIDC(ctx context.Context, c OIDCConfig, client *http.Client) (*OIDC, error) {
	ctx = oidc.ClientContext(ctx, client)
	provider, err := oidc.NewProvider(ctx, c.Issuer)
	if err != nil {
		return nil, errors.New("OIDC discovery failed; check issuer, TLS and connectivity")
	}
	endpoints := provider.Endpoint()
	for _, raw := range []string{endpoints.AuthURL, endpoints.TokenURL} {
		u, err := url.Parse(raw)
		if err != nil || u.Scheme != "https" || u.Host == "" || u.User != nil || u.Fragment != "" {
			return nil, errors.New("OIDC discovery requires HTTPS authorization and token endpoints")
		}
	}
	return &OIDC{config: c, oauth: oauth2.Config{ClientID: c.ClientID, ClientSecret: c.ClientSecret, RedirectURL: c.RedirectURL, Endpoint: endpoints, Scopes: []string{oidc.ScopeOpenID}}, verifier: provider.Verifier(&oidc.Config{ClientID: c.ClientID}), client: client, attempts: make(map[string]loginAttempt), now: time.Now}, nil
}

// Begin allocates a browser-bound attempt and returns its authorization URL.
func (o *OIDC) Begin() (location, binding string, err error) {
	state, binding, nonce, verifier := oauth2.GenerateVerifier(), oauth2.GenerateVerifier(), oauth2.GenerateVerifier(), oauth2.GenerateVerifier()
	o.mu.Lock()
	defer o.mu.Unlock()
	now := o.now()
	for key, attempt := range o.attempts {
		if !attempt.expires.After(now) {
			delete(o.attempts, key)
		}
	}
	if len(o.attempts) >= maxLoginAttempts {
		return "", "", errors.New("too many pending OIDC logins")
	}
	o.attempts[state] = loginAttempt{binding: binding, nonce: nonce, verifier: verifier, expires: now.Add(LoginAttemptTTL)}
	return o.oauth.AuthCodeURL(state, oidc.Nonce(nonce), oauth2.S256ChallengeOption(verifier), oauth2.SetAuthURLParam("response_mode", "query")), binding, nil
}

func (o *OIDC) consume(state, binding string) (loginAttempt, error) {
	o.mu.Lock()
	defer o.mu.Unlock()
	a, ok := o.attempts[state]
	if !ok || binding == "" || subtle.ConstantTimeCompare([]byte(a.binding), []byte(binding)) != 1 {
		return loginAttempt{}, ErrOIDCLogin
	}
	delete(o.attempts, state)
	if !a.expires.After(o.now()) {
		return loginAttempt{}, ErrOIDCLogin
	}
	return a, nil
}

// Complete verifies both the provider token and the browser-bound transaction.
// Only exact provider subjects explicitly admitted by the operator receive access.
func (o *OIDC) Complete(ctx context.Context, state, binding, code string) (string, time.Time, error) {
	a, err := o.consume(state, binding)
	if err != nil || code == "" {
		return "", time.Time{}, ErrOIDCLogin
	}
	ctx = oidc.ClientContext(ctx, o.client)
	token, err := o.oauth.Exchange(ctx, code, oauth2.VerifierOption(a.verifier))
	if err != nil {
		return "", time.Time{}, ErrOIDCLogin
	}
	raw, ok := token.Extra("id_token").(string)
	if !ok {
		return "", time.Time{}, ErrOIDCLogin
	}
	id, err := o.verifier.Verify(ctx, raw)
	if err != nil {
		return "", time.Time{}, ErrOIDCLogin
	}
	if id.Subject == "" || subtle.ConstantTimeCompare([]byte(id.Nonce), []byte(a.nonce)) != 1 || !slices.Contains(o.config.AllowedSubjects, id.Subject) {
		return "", time.Time{}, ErrOIDCLogin
	}
	// Check azp when supplied and require it for multi-audience ID tokens.
	var claims struct {
		AuthorizedParty string `json:"azp"`
	}
	if err := id.Claims(&claims); err != nil || (len(id.Audience) > 1 && claims.AuthorizedParty == "") || (claims.AuthorizedParty != "" && claims.AuthorizedParty != o.config.ClientID) {
		return "", time.Time{}, ErrOIDCLogin
	}
	if id.AccessTokenHash != "" && id.VerifyAccessToken(token.AccessToken) != nil {
		return "", time.Time{}, ErrOIDCLogin
	}
	// Length-bounded and unambiguous issuer+subject identity; never email/name.
	identity, _ := json.Marshal([]string{id.Issuer, id.Subject})
	digest := sha256.Sum256(identity)
	return fmt.Sprintf("oidc:%s", hex.EncodeToString(digest[:])), id.Expiry, nil
}
