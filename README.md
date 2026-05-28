# sigil-manager

Self-hostable web console for the [Sigil](https://github.com/Ju571nK/sigil)
AI-SPM project. Reads fleet data from `sigil-server`; owns local triage
state (acknowledge / assign / resolve / notes) for SOC analyst workflow.

> **Status:** Plan 02 (Foundation + Alerts queue) — feature-branch in
> review. Login → /alerts → slide-over → ack/assign/notes round-trips
> against either a live `sigil-server` or the built-in mock. See
> [`docs/superpowers/plans/2026-05-18-plan-02-foundation-and-alerts-queue.md`](docs/superpowers/plans/2026-05-18-plan-02-foundation-and-alerts-queue.md).

## Stack

- **Backend:** Go 1.22+, chi router, modernc.org/sqlite (pure-Go), JWT auth
- **Frontend:** React + Vite + TypeScript, TanStack Router/Query, Tailwind v4, shadcn/ui
- **Tests:** `go test` + Vitest + Playwright (vs. the embedded SPA in the production binary)
- **Lint:** golangci-lint + Biome
- **Distribution:** single Go binary with the SPA embedded via `embed.FS`

## Prerequisites

- Go 1.22+
- Node 20 LTS+
- `golangci-lint`, `air` (`go install github.com/air-verse/air@latest`)
- `make`

## Quickstart (mock mode)

No `sigil-server` required — boots the in-process Mock FleetClient with
30 deterministic events covering all four AI-Guard tools + three scope
shapes from contract §14.5.

```bash
# 1. Configure
cp .env.example .env
# Edit .env: set ADMIN_PASSWORD_BCRYPT to a hash you control + JWT_SECRET
# to a fresh 32+ byte value. See "Configuration" below for the snippets.

# 2. Run the prebuilt binary
MOCK_FLEET=1 make build && MOCK_FLEET=1 ./sigil-manager
# Or with hot reload:
#   Terminal 1 (Go):  MOCK_FLEET=1 make dev-go
#   Terminal 2 (Vite): make dev-web
#   Then open http://localhost:5173

# 3. Log in
open http://localhost:8080
# Use ADMIN_USERNAME + the plaintext password you hashed for ADMIN_PASSWORD_BCRYPT.
```

## Quickstart (connected to sigil-server)

Real fleet data from a running [Ju571nK/sigil](https://github.com/Ju571nK/sigil)
sigil-server. The read API must be enabled on the server side — see the
sigil README for the `SIGIL_SERVER_READ_API_ENABLED` + `SIGIL_SERVER_READ_TOKEN`
env vars.

```bash
cp .env.example .env
# Edit .env:
#   unset / comment out MOCK_FLEET
#   SIGIL_SERVER_BASE_URL=http://your-sigil-server:9090
#   SIGIL_SERVER_READ_TOKEN=<the same secret you set on sigil-server>
#   ADMIN_PASSWORD_BCRYPT=<bcrypt hash>
#   JWT_SECRET=<32+ byte random>

make build
./sigil-manager
open http://localhost:8080
```

## Configuration

All config is read from environment variables by `internal/config.Load()`.
The process refuses to start if any **required** var is missing.

| Variable | Required | Default | Purpose |
|---|---|---|---|
| `LISTEN_ADDR` | no | `:8080` | HTTP listen address |
| `SIGIL_SERVER_BASE_URL` | yes (unless `MOCK_FLEET=1`) | — | Base URL of sigil-server `/v1/*` API, no trailing slash |
| `SIGIL_SERVER_READ_TOKEN` | yes (unless `MOCK_FLEET=1`) | — | Bearer token sigil-server validates per contract §F2 |
| `MOCK_FLEET` | no | `0` | Set to `1` to bypass HTTP and use the in-process Mock fixture |
| `TRIAGE_DB_PATH` | no | `./var/triage.sqlite` | SQLite file for ack/assign/notes; directory must be writable |
| `ADMIN_USERNAME` | yes | — | Login username for the single admin |
| `ADMIN_PASSWORD_BCRYPT` | yes | — | Bcrypt hash of the admin password (must start with `$2a$/$2b$/$2y$`, 60 chars) |
| `JWT_SECRET` | yes | — | HS256 signing secret for session JWTs (≥32 bytes) |
| `JWT_TTL_HOURS` | no | `12` | Session lifetime in hours (UI/UX §9) |
| `FLEET_POLL_INTERVAL_SECONDS` | no | `5` | How often the SPA polls `/fleet/events` (UI/UX §7.2) |
| `SIGIL_INSECURE_COOKIE` | no | unset | Set to `1` for local-dev only to clear the cookie `Secure` flag |

### Generating secrets

```bash
# Bcrypt the admin password (cost 10):
docker run --rm pajlada/bcrypt-cli 'your-password'
# Or via Go:
#   import "golang.org/x/crypto/bcrypt"
#   h, _ := bcrypt.GenerateFromPassword([]byte("your-password"), 10)

# JWT secret (32 bytes, base64-encoded):
openssl rand -base64 32
```

## Commands

| Command          | What it does                                            |
|------------------|---------------------------------------------------------|
| `make dev-go`    | Go server with hot reload via air (port 8080)           |
| `make dev-web`   | Vite frontend dev server (port 5173, proxies `/api`)    |
| `make build`     | Build SPA, embed it, build Go binary → `./sigil-manager` |
| `make test`      | All unit tests (Go + Vitest)                            |
| `make e2e`       | Build binary, then run Playwright vs. embedded SPA      |
| `make lint`      | golangci-lint + Biome                                   |
| `make clean`     | Remove build artifacts                                  |

## Running the production binary

```bash
make build
./sigil-manager
# Default port :8080. Override with LISTEN_ADDR=:9000.
```

## Architecture

```
sigil-manager (single Go binary)
├── chi router
├── /api/health         → liveness probe (unauthed)
├── /api/v1/auth/*      → login / logout / me (cookie JWT)
├── /api/v1/fleet/*     → passthrough to FleetClient (Http or Mock)
├── /api/v1/triage/*    → local SQLite triage store
└── /*                  → embedded SPA (web/dist/ → internal/server/dist/)
```

In dev, Vite serves the SPA directly and proxies `/api/*` to the Go
process. In prod, Go serves both from one process on one port.

## What's in Plan 02

The current foundation ships: config + env loader, `FleetClient`
interface with both HTTP and Mock implementations, SQLite-backed triage
store, bcrypt + JWT cookie auth, the `/alerts` queue page with filter
chips + 5s polling + keyboard shortcuts (`j/k/Enter/Esc/a/c/r/i/n///?`),
and a slide-over with FactGrid + acknowledge/assign/notes actions.

Plan 05+ adds the Settings page on top of the same foundation —
see [`docs/superpowers/plans/`](docs/superpowers/plans/).

## What's in Plan 03

The Fleet section: `/fleet/risk` (hosts sorted by AI Guard risk),
`/fleet/events` (fleet-wide event timeline), and `/fleet/compliance`
(per-host policy state with a client-derived status pill). Read-only,
20s polling.

## What's in Plan 04

Host detail at `/hosts/$hostId` (reached by clicking a hostname in any fleet
table): per-tool AI Guard risk with reasons, host metadata, policy state,
agent health, a derived compliance pill, and the host's recent events.
Read-only, 20s polling, null-tolerant for disconnected hosts.

## What this repo is NOT

This is the OSS, self-hostable console only. Multi-tenancy, billing,
SSO/SAML, and compliance pipelines live in the (private) `sigil-cloud`
repo. See [`CLAUDE.md`](CLAUDE.md) and `sigil-strategy.md` for the
scope boundaries.

## License

TBD between Apache-2.0 and BSL/source-available. Decision pending; see
`sigil-strategy.md` for context.
