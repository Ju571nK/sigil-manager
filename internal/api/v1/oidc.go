package v1

import (
	"net/http"
	"time"

	"github.com/Ju571nK/sigil-manager/internal/auth"
	"github.com/Ju571nK/sigil-manager/internal/httputil"
)

const oidcBindingCookie = "sigil_oidc_binding"

func (s *Server) handleAuthMethods(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	httputil.WriteJSON(w, http.StatusOK, map[string]bool{"local": true, "oidc": s.OIDC != nil})
}

func (s *Server) bindingCookie(value string, maxAge int) *http.Cookie {
	return &http.Cookie{Name: oidcBindingCookie, Value: value, Path: "/api/v1/auth/oidc", MaxAge: maxAge, HttpOnly: true, Secure: s.Auth.CookieSecure, SameSite: http.SameSiteLaxMode}
}

func (s *Server) handleOIDCStart(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("Referrer-Policy", "no-referrer")
	if s.OIDC == nil {
		http.NotFound(w, r)
		return
	}
	location, binding, err := s.OIDC.Begin()
	if err != nil {
		writeError(w, http.StatusServiceUnavailable, "login_unavailable", "Sign-in is busy. Try again shortly.")
		return
	}
	http.SetCookie(w, s.bindingCookie(binding, int(auth.LoginAttemptTTL.Seconds())))
	http.Redirect(w, r, location, http.StatusFound)
}

func (s *Server) handleOIDCCallback(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("Referrer-Policy", "no-referrer")
	if s.OIDC == nil {
		http.NotFound(w, r)
		return
	}
	cookie, err := r.Cookie(oidcBindingCookie)
	http.SetCookie(w, s.bindingCookie("", -1))
	fail := func() { http.Redirect(w, r, "/login?oidc_error=1", http.StatusSeeOther) }
	if err != nil {
		fail()
		return
	}
	// Never reflect provider error descriptions, codes or tokens into the page.
	code := r.URL.Query().Get("code")
	if r.URL.Query().Get("error") != "" {
		code = ""
	}
	subject, deadline, err := s.OIDC.Complete(r.Context(), r.URL.Query().Get("state"), cookie.Value, code)
	if err != nil {
		fail()
		return
	}
	token, exp, err := s.Signer.SignUntil(subject, deadline)
	if err != nil {
		fail()
		return
	}
	http.SetCookie(w, &http.Cookie{Name: CookieName, Value: token, Path: "/", Expires: exp, MaxAge: max(1, int(time.Until(exp).Seconds())), HttpOnly: true, Secure: s.Auth.CookieSecure, SameSite: http.SameSiteLaxMode})
	http.Redirect(w, r, "/alerts", http.StatusSeeOther)
}
