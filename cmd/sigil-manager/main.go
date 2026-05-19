package main

import (
	"log"
	"net/http"
	"os"

	"github.com/Ju571nK/sigil-manager/internal/server"
)

func main() {
	addr := os.Getenv("SIGIL_MANAGER_ADDR")
	if addr == "" {
		addr = ":8080"
	}

	// TODO(T8): construct internal/api/v1.Server from internal/config + fleet +
	// triage + auth and pass it here instead of nil. With nil, /api/v1/* is
	// not mounted — /api/health and the SPA still work.
	router := server.NewRouter(nil)
	log.Printf("sigil-manager listening on %s", addr)
	if err := http.ListenAndServe(addr, router); err != nil {
		log.Fatalf("server error: %v", err)
	}
}
