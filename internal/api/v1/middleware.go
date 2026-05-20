// Package v1 implements sigil-manager's `/api/v1/*` HTTP surface — the SPA's
// only consumer-side API. It composes [fleet.Client], [triage.Repo], and
// [auth.Signer]; nothing here is publicly reachable on `sigil-server`.
package v1

import (
	"context"
	"errors"
	"net/http"

	"github.com/Ju571nK/sigil-manager/internal/auth"
	"github.com/Ju571nK/sigil-manager/internal/httputil"
)

// CookieName is the HTTP-only session cookie holding the signed JWT.
const CookieName = "sigil_session"

// ctxKey is the unexported key under which RequireAuth stuffs the subject
// (admin username) into request context. Handlers retrieve it via [Subject].
type ctxKey struct{}

// Subject returns the admin username established by [RequireAuth]. Returns
// "" if no authenticated request context flowed through RequireAuth — the
// handler should treat that as a programming error, not a normal path.
func Subject(ctx context.Context) string {
	s, _ := ctx.Value(ctxKey{}).(string)
	return s
}

// errorBody mirrors the contract §6.1 wire shape so SPA + sigil-server errors
// look the same to the frontend.
type errorBody struct {
	Error errorPayload `json:"error"`
}

type errorPayload struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

func writeError(w http.ResponseWriter, status int, code, msg string) {
	httputil.WriteJSON(w, status, errorBody{Error: errorPayload{Code: code, Message: msg}})
}

// RequireAuth verifies the session cookie and stuffs the subject into the
// request context for downstream handlers.
//
// Cookie missing or empty → 401 unauthorized.
// Cookie present but expired → 401 unauthorized + "session_expired" code so
//
//	the SPA can distinguish "never logged in" from "session ran out".
//
// Cookie present but invalid (tampered, wrong signature, malformed) → 401
//
//	unauthorized with generic code — never tell the attacker which part of
//	the token failed.
func RequireAuth(signer *auth.Signer) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			c, err := r.Cookie(CookieName)
			if err != nil || c.Value == "" {
				writeError(w, http.StatusUnauthorized, "unauthorized", "session cookie missing")
				return
			}
			subject, err := signer.Verify(c.Value)
			if err != nil {
				if errors.Is(err, auth.ErrExpiredToken) {
					writeError(w, http.StatusUnauthorized, "session_expired", "session expired, please log in again")
					return
				}
				writeError(w, http.StatusUnauthorized, "unauthorized", "invalid session")
				return
			}
			ctx := context.WithValue(r.Context(), ctxKey{}, subject)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}
