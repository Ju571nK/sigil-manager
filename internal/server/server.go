package server

import (
	"log"
	"net/http"

	"github.com/Ju571nK/sigil-manager/internal/api"
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
)

func NewRouter() http.Handler {
	r := chi.NewRouter()
	r.Use(middleware.RequestID)
	r.Use(middleware.RealIP)
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)

	r.Route("/api", func(r chi.Router) {
		r.Get("/health", api.HealthHandler)
	})

	spa, err := spaHandler()
	if err != nil {
		log.Fatalf("failed to mount SPA: %v", err)
	}
	r.Handle("/*", spa)

	return r
}
