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

	router := server.NewRouter()
	log.Printf("sigil-manager listening on %s", addr)
	if err := http.ListenAndServe(addr, router); err != nil {
		log.Fatalf("server error: %v", err)
	}
}
