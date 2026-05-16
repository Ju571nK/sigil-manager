# sigil-manager

Self-hostable web console for the [Sigil](https://github.com/Ju571nK/sigil)
AI-SPM project. Reads fleet data from `sigil-server`; owns local triage
state (acknowledge / assign / resolve / notes) for SOC analyst workflow.

> **Status:** Pre-alpha. Scaffolded — no features yet. See
> `docs/superpowers/specs/2026-05-16-ui-ux-design.md` for the v1 plan.

## Stack

- **Backend:** Go 1.22+, chi router, modernc.org/sqlite (pure-Go), JWT auth
- **Frontend:** React + Vite + TypeScript, TanStack Router/Query, Tailwind v4, shadcn/ui
- **Tests:** `go test` + Vitest
- **Lint:** golangci-lint + Biome
- **Distribution:** single Go binary with the SPA embedded via `embed.FS`

## Prerequisites

- Go 1.22+
- Node 20 LTS+
- `golangci-lint`, `air` (`go install github.com/air-verse/air@latest`)
- `make`

## Quickstart

```bash
# Install frontend deps
cd web && npm ci && cd ..

# Terminal 1: Go server with hot reload
make dev-go

# Terminal 2: Vite frontend (proxies /api to Go)
make dev-web

# Open http://localhost:5173 — should show the hello-world health probe
```

## Commands

| Command          | What it does                                            |
|------------------|---------------------------------------------------------|
| `make dev-go`    | Go server with hot reload via air (port 8080)           |
| `make dev-web`   | Vite frontend dev server (port 5173, proxies `/api`)    |
| `make build`     | Build SPA, embed it, build Go binary → `./sigil-manager` |
| `make test`      | All tests (Go + frontend)                               |
| `make lint`      | golangci-lint + Biome                                   |
| `make clean`     | Remove build artifacts                                  |

## Running the production binary

```bash
make build
./sigil-manager
# Default port :8080, override with SIGIL_MANAGER_ADDR=:9000
```

## Architecture

```
sigil-manager (single Go binary)
├── chi router
├── /api/*  → Go HTTP handlers
└── /*      → embedded SPA (web/dist/ → internal/server/dist/)
```

In dev, Vite serves the SPA directly and proxies `/api/*` to the Go
process. In prod, Go serves both from one process on one port.

## What this repo is NOT

This is the OSS, self-hostable console only. Multi-tenancy, billing,
SSO/SAML, and compliance pipelines live in the (private) `sigil-cloud`
repo. See `CLAUDE.md` and `sigil-strategy.md` for the scope boundaries.

## License

TBD between Apache-2.0 and BSL/source-available. Decision pending; see
`sigil-strategy.md` for context.
