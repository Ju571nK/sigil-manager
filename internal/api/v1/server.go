package v1

import (
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/Ju571nK/sigil-manager/internal/auth"
	"github.com/Ju571nK/sigil-manager/internal/fleet"
	"github.com/Ju571nK/sigil-manager/internal/triage"
)

// AuthConfig is the subset of [config.Config] this package needs. Decoupled
// from config so tests can wire arbitrary admin creds without env munging.
type AuthConfig struct {
	AdminUsername       string
	AdminPasswordBcrypt string

	// CookieSecure controls the `Secure` flag on the session cookie. true
	// in production (TLS terminator in front), false for local dev where
	// the SPA hits http://localhost.
	CookieSecure bool
}

// Server owns the `/api/v1` handlers and their dependencies. All routes
// except `POST /api/v1/auth/login` are gated by [RequireAuth].
type Server struct {
	Fleet  fleet.Client
	Triage *triage.Repo
	Signer *auth.Signer
	Auth   AuthConfig
}

// Routes returns a chi sub-router mounting every `/api/v1/*` handler.
// Mount it from the parent server via `r.Mount("/api/v1", v1Server.Routes())`.
//
// Route table mirrors plan T7's:
//
//	POST   /auth/login                       (public)
//	POST   /auth/logout                      (cookie)
//	GET    /auth/me                          (cookie)
//	GET    /fleet/meta                       (cookie)
//	GET    /fleet/healthz                    (cookie)
//	GET    /fleet/events                     (cookie)
//	GET    /fleet/events/{event_id}          (cookie)
//	GET    /fleet/risk                       (cookie)
//	GET    /fleet/compliance                 (cookie)
//	GET    /fleet/hosts/{host_id}            (cookie)
//	POST   /triage/upsert                    (cookie)
//	POST   /triage/note                      (cookie)
//	GET    /triage/{host_id}/{event_id}      (cookie)
func (s *Server) Routes() chi.Router {
	r := chi.NewRouter()

	// Public. Login is per-IP rate-limited (10/min for remote clients; loopback
	// exempt) to slow online brute force on top of the bcrypt + fail-delay.
	loginLimiter := newRateLimiter(10, time.Minute)
	r.With(loginLimiter.middleware).Post("/auth/login", s.handleLogin)

	// Authenticated.
	r.Group(func(r chi.Router) {
		r.Use(RequireAuth(s.Signer))

		r.Post("/auth/logout", s.handleLogout)
		r.Get("/auth/me", s.handleMe)

		r.Get("/fleet/meta", s.handleFleetMeta)
		r.Get("/fleet/healthz", s.handleFleetHealthz)
		r.Get("/fleet/events", s.handleFleetEvents)
		r.Get("/fleet/events/{event_id}", s.handleFleetEventByID)
		r.Get("/fleet/risk", s.handleFleetRisk)
		r.Get("/fleet/compliance", s.handleFleetCompliance)
		r.Get("/fleet/hosts/{host_id}", s.handleFleetHostByID)

		r.Post("/triage/upsert", s.handleTriageUpsert)
		r.Post("/triage/note", s.handleTriageNote)
		r.Get("/triage/{host_id}/{event_id}", s.handleTriageGet)
	})

	return r
}
