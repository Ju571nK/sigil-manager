package server

import "net/http"

// contentSecurityPolicy locks the SPA to same-origin scripts/styles/connections
// and forbids framing (clickjacking). The Vite build emits only external
// same-origin assets + the favicon, so 'self' covers scripts; 'unsafe-inline'
// on style-src is needed for React inline style attributes (style={{…}}) and
// is low-risk (it does not enable inline <script>). connect-src 'self' matches
// the SPA's same-origin /api/v1 calls.
const contentSecurityPolicy = "default-src 'self'; " +
	"script-src 'self'; " +
	"style-src 'self' 'unsafe-inline'; " +
	"img-src 'self' data:; " +
	"font-src 'self'; " +
	"connect-src 'self'; " +
	"frame-ancestors 'none'; " +
	"base-uri 'self'; " +
	"form-action 'self'; " +
	"object-src 'none'"

// securityHeaders sets defense-in-depth response headers on every response
// (SPA documents and API JSON alike). HSTS is intentionally omitted — TLS
// termination (and thus the decision to assert HSTS) belongs to the reverse
// proxy in front of this console, not the app, which may legitimately serve
// plain http on a trusted LAN.
func securityHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		h := w.Header()
		h.Set("Content-Security-Policy", contentSecurityPolicy)
		h.Set("X-Frame-Options", "DENY") // legacy clickjacking guard alongside CSP frame-ancestors
		h.Set("X-Content-Type-Options", "nosniff")
		h.Set("Referrer-Policy", "no-referrer")
		next.ServeHTTP(w, r)
	})
}
