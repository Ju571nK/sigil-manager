package v1

import (
	"crypto/subtle"
	"encoding/json"
	"net/http"
	"time"

	"github.com/Ju571nK/sigil-manager/internal/auth"
	"github.com/Ju571nK/sigil-manager/internal/httputil"
)

// loginRequest is the body of POST /api/v1/auth/login.
type loginRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

// loginResponse is returned on successful login. The cookie carries the
// actual session; this body just lets the SPA flip its auth state without
// a follow-up /me call.
type loginResponse struct {
	Username  string    `json:"username"`
	ExpiresAt time.Time `json:"expires_at"`
}

// meResponse is the body of GET /api/v1/auth/me.
type meResponse struct {
	Username  string    `json:"username"`
	ExpiresAt time.Time `json:"expires_at"`
}

// loginFailDelay is a fixed wait inserted on every failed-credentials path
// so an attacker can't distinguish "unknown user" from "wrong password" via
// response timing. Wall-clock attacks on bcrypt are already covered by the
// cost factor; this is defense against the username-side leak.
const loginFailDelay = 250 * time.Millisecond

// handleLogin authenticates against the configured single admin and sets
// the session cookie on success.
func (s *Server) handleLogin(w http.ResponseWriter, r *http.Request) {
	var req loginRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_query", "malformed JSON body")
		return
	}

	// Both checks always run — never short-circuit on a username mismatch,
	// or response timing leaks user existence.
	usernameOK := subtle.ConstantTimeCompare([]byte(req.Username), []byte(s.Auth.AdminUsername)) == 1
	passwordOK := auth.Verify(req.Password, s.Auth.AdminPasswordBcrypt)
	if !usernameOK || !passwordOK {
		time.Sleep(loginFailDelay)
		writeError(w, http.StatusUnauthorized, "invalid_credentials", "username or password is wrong")
		return
	}

	token, exp, err := s.Signer.Sign(s.Auth.AdminUsername)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal", "failed to issue session")
		return
	}

	http.SetCookie(w, &http.Cookie{
		Name:     CookieName,
		Value:    token,
		Path:     "/",
		Expires:  exp,
		MaxAge:   int(s.Signer.TTL().Seconds()),
		HttpOnly: true,
		Secure:   s.Auth.CookieSecure,
		SameSite: http.SameSiteLaxMode,
	})
	httputil.WriteJSON(w, http.StatusOK, loginResponse{
		Username:  s.Auth.AdminUsername,
		ExpiresAt: exp,
	})
}

// handleLogout clears the session cookie. Idempotent: calling it without a
// cookie is fine (the middleware lets it through only with a valid cookie,
// but the action is the same — emit a deletion cookie).
func (s *Server) handleLogout(w http.ResponseWriter, _ *http.Request) {
	http.SetCookie(w, &http.Cookie{
		Name:     CookieName,
		Value:    "",
		Path:     "/",
		Expires:  time.Unix(0, 0),
		MaxAge:   -1,
		HttpOnly: true,
		Secure:   s.Auth.CookieSecure,
		SameSite: http.SameSiteLaxMode,
	})
	httputil.WriteJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

// handleMe returns the current admin + session expiry. The SPA polls this
// (or reads it once on app boot) to learn whether the cookie is still good.
//
// `expires_at` is derived from the JWT's `exp` claim, not the cookie's
// Max-Age — they agree but JWT is the authoritative source.
func (s *Server) handleMe(w http.ResponseWriter, r *http.Request) {
	c, err := r.Cookie(CookieName)
	if err != nil {
		// RequireAuth already returned 401 if no cookie; reaching here means
		// the cookie was deleted mid-request. Treat as 401.
		writeError(w, http.StatusUnauthorized, "unauthorized", "session cookie missing")
		return
	}
	subject, exp, err := s.Signer.VerifyWithExp(c.Value)
	if err != nil {
		writeError(w, http.StatusUnauthorized, "unauthorized", "invalid session")
		return
	}
	httputil.WriteJSON(w, http.StatusOK, meResponse{Username: subject, ExpiresAt: exp})
}
