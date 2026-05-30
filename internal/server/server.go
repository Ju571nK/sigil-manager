package server

import (
	"log"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"

	"github.com/Ju571nK/sigil-manager/internal/api"
	apiv1 "github.com/Ju571nK/sigil-manager/internal/api/v1"
)

// NewRouter constructs the top-level chi router. Pass nil for v1Server in
// tests / build verification where only `/api/health` and the SPA matter;
// production main.go always supplies a wired [apiv1.Server].
func NewRouter(v1Server *apiv1.Server) http.Handler {
	r := chi.NewRouter()
	r.Use(middleware.RequestID)
	r.Use(middleware.RealIP)
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)
	r.Use(securityHeaders)

	r.Route("/api", func(r chi.Router) {
		r.Get("/health", api.HealthHandler)
		if v1Server != nil {
			r.Mount("/v1", v1Server.Routes())
		}
	})

	spa, err := spaHandler()
	if err != nil {
		log.Fatalf("failed to mount SPA: %v", err)
	}
	r.Handle("/*", spa)

	return r
}
