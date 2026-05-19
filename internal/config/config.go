// Package config loads sigil-manager's runtime configuration from environment
// variables. The Config struct is the single shape every other internal package
// reads from; Load is the single producer.
//
// Required-vs-optional and validation rules are documented per-field on the
// struct. Operators should consult `.env.example` at the repo root for the
// canonical list of variables.
package config

import (
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"
)

// Config is the resolved runtime configuration for a sigil-manager process.
// Constructed via Load(). All fields are immutable after construction.
type Config struct {
	// Server
	ListenAddr string // LISTEN_ADDR, default ":8080"

	// Fleet client (sigil-server target)
	SigilServerBaseURL   string // SIGIL_SERVER_BASE_URL, required unless MockFleet
	SigilServerReadToken string // SIGIL_SERVER_READ_TOKEN, required unless MockFleet
	MockFleet            bool   // MOCK_FLEET=1 swaps in the mock client

	// Triage state DB
	TriageDBPath string // TRIAGE_DB_PATH, default "./var/triage.sqlite"

	// Single-admin auth (UI/UX §9)
	AdminUsername       string // ADMIN_USERNAME, required
	AdminPasswordBcrypt string // ADMIN_PASSWORD_BCRYPT, required (bcrypt hash)

	// JWT session
	JWTSecret []byte        // JWT_SECRET, required, min 32 bytes
	JWTTTL    time.Duration // derived from JWT_TTL_HOURS (default 12h)

	// Informational (consumed by SPA via /api/v1/meta, not the server core)
	FleetPollInterval time.Duration // FLEET_POLL_INTERVAL_SECONDS, default 5s
}

// IsMockFleet reports whether the operator opted into the in-process mock
// FleetClient. When true, SigilServerBaseURL and SigilServerReadToken are
// optional.
func (c *Config) IsMockFleet() bool {
	return c.MockFleet
}

// ErrMissingRequired indicates one or more required env vars were unset or
// empty.
var ErrMissingRequired = errors.New("config: required env var unset")

// Load reads the process environment and returns a validated Config.
//
// Validation:
//   - Required env vars must be present and non-empty (see struct doc).
//   - JWT_SECRET must be at least 32 bytes after trimming.
//   - ADMIN_PASSWORD_BCRYPT must look like a bcrypt hash ($2a$ / $2b$ / $2y$
//     prefix) — Load does not invoke bcrypt; that's the auth package's job.
//   - MOCK_FLEET="1" relaxes SIGIL_SERVER_BASE_URL and SIGIL_SERVER_READ_TOKEN.
//
// Errors wrap ErrMissingRequired for the "unset required" case so callers can
// `errors.Is(err, config.ErrMissingRequired)`.
func Load() (*Config, error) {
	c := &Config{
		ListenAddr:   strDefault("LISTEN_ADDR", ":8080"),
		MockFleet:    os.Getenv("MOCK_FLEET") == "1",
		TriageDBPath: strDefault("TRIAGE_DB_PATH", "./var/triage.sqlite"),

		SigilServerBaseURL:   strings.TrimSpace(os.Getenv("SIGIL_SERVER_BASE_URL")),
		SigilServerReadToken: strings.TrimSpace(os.Getenv("SIGIL_SERVER_READ_TOKEN")),

		AdminUsername:       strings.TrimSpace(os.Getenv("ADMIN_USERNAME")),
		AdminPasswordBcrypt: strings.TrimSpace(os.Getenv("ADMIN_PASSWORD_BCRYPT")),

		JWTSecret: []byte(strings.TrimSpace(os.Getenv("JWT_SECRET"))),
	}

	ttlHours, err := intDefault("JWT_TTL_HOURS", 12)
	if err != nil {
		return nil, err
	}
	c.JWTTTL = time.Duration(ttlHours) * time.Hour

	pollSeconds, err := intDefault("FLEET_POLL_INTERVAL_SECONDS", 5)
	if err != nil {
		return nil, err
	}
	c.FleetPollInterval = time.Duration(pollSeconds) * time.Second

	if err := c.validate(); err != nil {
		return nil, err
	}
	return c, nil
}

func (c *Config) validate() error {
	var missing []string

	if !c.MockFleet {
		if c.SigilServerBaseURL == "" {
			missing = append(missing, "SIGIL_SERVER_BASE_URL")
		}
		if c.SigilServerReadToken == "" {
			missing = append(missing, "SIGIL_SERVER_READ_TOKEN")
		}
	}
	if c.AdminUsername == "" {
		missing = append(missing, "ADMIN_USERNAME")
	}
	if c.AdminPasswordBcrypt == "" {
		missing = append(missing, "ADMIN_PASSWORD_BCRYPT")
	}
	if len(c.JWTSecret) == 0 {
		missing = append(missing, "JWT_SECRET")
	}

	if len(missing) > 0 {
		return fmt.Errorf("%w: %s", ErrMissingRequired, strings.Join(missing, ", "))
	}

	if len(c.JWTSecret) < 32 {
		return fmt.Errorf("config: JWT_SECRET must be at least 32 bytes (got %d)", len(c.JWTSecret))
	}

	if !isBcryptHash(c.AdminPasswordBcrypt) {
		return fmt.Errorf("config: ADMIN_PASSWORD_BCRYPT does not look like a bcrypt hash (must start with $2a$, $2b$, or $2y$)")
	}

	if c.JWTTTL <= 0 {
		return fmt.Errorf("config: JWT_TTL_HOURS must be positive (got %v)", c.JWTTTL)
	}
	if c.FleetPollInterval <= 0 {
		return fmt.Errorf("config: FLEET_POLL_INTERVAL_SECONDS must be positive (got %v)", c.FleetPollInterval)
	}

	return nil
}

func strDefault(key, fallback string) string {
	v := strings.TrimSpace(os.Getenv(key))
	if v == "" {
		return fallback
	}
	return v
}

func intDefault(key string, fallback int) (int, error) {
	v := strings.TrimSpace(os.Getenv(key))
	if v == "" {
		return fallback, nil
	}
	n, err := strconv.Atoi(v)
	if err != nil {
		return 0, fmt.Errorf("config: %s must be an integer: %w", key, err)
	}
	return n, nil
}

// bcrypt hash format: $2[abxy]$<cost>$<22-char-salt><31-char-hash>
// We only check the prefix here — the auth package owns real verification.
func isBcryptHash(s string) bool {
	if len(s) < 60 {
		return false
	}
	return strings.HasPrefix(s, "$2a$") ||
		strings.HasPrefix(s, "$2b$") ||
		strings.HasPrefix(s, "$2y$")
}
