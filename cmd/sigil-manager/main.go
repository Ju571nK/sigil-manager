// Command sigil-manager runs the consumer-side console: HTTPS-fronted Go
// binary that mounts the embedded React SPA plus the `/api/v1/*` HTTP
// surface backed by [fleet.Client] (reads from sigil-server) and
// [triage.Repo] (local SQLite for ack/assign/notes).
//
// All runtime configuration is environment-driven; see `.env.example` at
// the repo root for the full list. The process refuses to start if any
// required var is unset (see internal/config).
package main

import (
	"context"
	"errors"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	v1 "github.com/Ju571nK/sigil-manager/internal/api/v1"
	"github.com/Ju571nK/sigil-manager/internal/auth"
	"github.com/Ju571nK/sigil-manager/internal/config"
	"github.com/Ju571nK/sigil-manager/internal/fleet"
	"github.com/Ju571nK/sigil-manager/internal/server"
	"github.com/Ju571nK/sigil-manager/internal/triage"
)

const shutdownGrace = 5 * time.Second

func main() {
	if err := run(); err != nil {
		log.Fatalf("sigil-manager: %v", err)
	}
}

// run holds the actual boot sequence so deferred cleanups (triage Close,
// signal-notify cancel) run on exit. main() only calls os.Exit on error,
// which would skip defers.
func run() error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}

	repo, err := triage.Open(cfg.TriageDBPath)
	if err != nil {
		return err
	}
	defer func() { _ = repo.Close() }()

	signer, err := auth.NewSigner(string(cfg.JWTSecret), cfg.JWTTTL)
	if err != nil {
		return err
	}

	fleetClient := fleet.NewFromConfig(cfg)

	v1Server := &v1.Server{
		Fleet:  fleetClient,
		Triage: repo,
		Signer: signer,
		Auth: v1.AuthConfig{
			AdminUsername:       cfg.AdminUsername,
			AdminPasswordBcrypt: cfg.AdminPasswordBcrypt,
			// In production a TLS terminator (Caddy / Traefik / Cloudflare)
			// fronts the binary, so Secure cookies are mandatory. Local dev
			// without TLS overrides via SIGIL_INSECURE_COOKIE=1 — anything
			// else defaults to secure.
			CookieSecure: os.Getenv("SIGIL_INSECURE_COOKIE") != "1",
		},
	}

	router := server.NewRouter(v1Server)

	httpSrv := &http.Server{
		Addr:              cfg.ListenAddr,
		Handler:           router,
		ReadHeaderTimeout: 10 * time.Second,
	}

	// Wire SIGINT/SIGTERM so Ctrl-C in dev and `kill <pid>` in prod both
	// drain in-flight requests before Close().
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	listenErr := make(chan error, 1)
	go func() {
		log.Printf("sigil-manager: mode=%s addr=%s db=%s", modeLabel(cfg), cfg.ListenAddr, cfg.TriageDBPath)
		if err := httpSrv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			listenErr <- err
		}
		close(listenErr)
	}()

	select {
	case <-ctx.Done():
		log.Printf("sigil-manager: shutdown signal received, draining for up to %s", shutdownGrace)
		shutdownCtx, cancel := context.WithTimeout(context.Background(), shutdownGrace)
		defer cancel()
		if err := httpSrv.Shutdown(shutdownCtx); err != nil {
			log.Printf("sigil-manager: shutdown error: %v", err)
		}
	case err := <-listenErr:
		if err != nil {
			return err
		}
	}
	return nil
}

func modeLabel(cfg *config.Config) string {
	if cfg.IsMockFleet() {
		return "mock-fleet"
	}
	return "live-fleet (" + cfg.SigilServerBaseURL + ")"
}
