# sigil-manager Plan 02: Foundation + Alerts Queue

> **For agentic workers:** REQUIRED SUB-SKILL: Use
> `superpowers:subagent-driven-development` (recommended) or
> `superpowers:executing-plans` to implement this plan task-by-task. Steps
> use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Take the v1.0 fleet API contract
(`docs/superpowers/specs/2026-05-16-fleet-api-contract.md`) and the
UI/UX spec (`docs/superpowers/specs/2026-05-16-ui-ux-design.md`), and
ship the consumer-side foundation plus the Alerts queue (the landing
screen per UI/UX D2). End state: a developer runs `make dev`, logs in
with env-supplied credentials, sees the Alerts queue populated from
either a live `sigil-server` or the mock client, opens an alert
slide-over, marks it acknowledged/resolved with notes, and the triage
state survives a restart. CI green.

**Architecture:**
- Backend (Go): three layers behind chi.
  - `internal/fleet/` — `FleetClient` interface + types matching contract §5;
    `Http` impl against `sigil-server`; `Mock` impl with deterministic fixtures.
  - `internal/triage/` — SQLite repo for ack/assign/resolve/notes per
    `(host_id, event_id)` (per contract §1 guidance).
  - `internal/auth/` — single-admin bcrypt + JWT 12h.
  - `internal/api/v1/` — HTTP handlers wiring `FleetClient` + `TriageRepo` to
    the SPA under `/api/v1/...`.
- Frontend (React/Vite): design tokens per UI/UX §6, top nav per §4, two
  routes in scope (`/login`, `/alerts`), and the slide-over per §5.1/§7.
  TanStack Router + Query already scaffolded in Plan 01.
- Auth flow: UI POSTs `/api/v1/auth/login` → server returns JWT in
  HTTP-only `Set-Cookie`. SPA uses cookie for subsequent requests.
- Polling: TanStack Query polls `/api/v1/fleet/events` every 5s while the
  Alerts queue is visible (UI/UX §7.2).

**Tech stack additions over Plan 01:**

Backend:
- `modernc.org/sqlite` (pure-Go, no cgo) — triage state DB
- `github.com/golang-jwt/jwt/v5` — JWT signing/verification
- `golang.org/x/crypto/bcrypt` — admin password hashing

Frontend:
- `date-fns` — relative time strings ("Updated 3s ago")
- shadcn/ui primitives: `sheet` (slide-over), `input`, `label`, `badge`,
  `table`, `tabs`, `dialog`, `sonner` (toasts), `dropdown-menu`

Out of scope for Plan 02 (handled in Plan 03+):
- Fleet pages (Risk / Events / Compliance tabs)
- Host detail page + tabs
- Settings page (beyond a stub showing connection status)
- Search (UI/UX §7.3 — uses client-side filter on current page only in v1)
- Light theme
- i18n / Korean translation

---

## Prerequisites

The implementing engineer needs locally:
- **Go 1.22+** (already required by Plan 01)
- **Node 20 LTS+** and **npm 10+** (already required by Plan 01)
- **golangci-lint**, **air**, **make** (already from Plan 01)
- **A running `sigil-server` 0.5.0+ OR willingness to develop against the
  mock client** — `sigil-server` may not have a tagged 0.5.0 release at
  Plan 02 start, but its `main` branch (`Ju571nK/sigil` from `0dce160`
  onward) ships all 9 endpoints in this contract. See §13 of the contract
  for boot/rebuild behaviour.

A local `sigil-server` instance for end-to-end testing is recommended
but not required to land Plan 02 — `MOCK_FLEET=1` toggles the mock client
inside the manager binary so the developer can iterate on the UI without
running the full sigil stack.

---

## File structure (target end state of Plan 2)

```
sigil-manager/
├── .env.example                              # NEW: documents all env vars
├── cmd/sigil-manager/main.go                 # MODIFY: wire config + auth + fleet + triage
├── internal/
│   ├── api/                                  # MODIFY (existing /api/health stays)
│   │   ├── handlers.go                       # MODIFY: existing health handler
│   │   ├── handlers_test.go                  # existing
│   │   └── v1/                               # NEW
│   │       ├── auth.go                       # POST /api/v1/auth/login, /logout
│   │       ├── auth_test.go
│   │       ├── fleet.go                      # GET /api/v1/fleet/events, /meta, /healthz
│   │       ├── fleet_test.go
│   │       ├── triage.go                     # GET/POST/PATCH /api/v1/triage/*
│   │       ├── triage_test.go
│   │       └── middleware.go                 # bearer/cookie JWT middleware
│   ├── auth/                                 # NEW
│   │   ├── jwt.go                            # sign + verify
│   │   ├── jwt_test.go
│   │   ├── password.go                       # bcrypt wrapper
│   │   └── password_test.go
│   ├── config/                               # NEW
│   │   ├── config.go                         # env loader + defaults
│   │   └── config_test.go
│   ├── fleet/                                # NEW
│   │   ├── client.go                         # FleetClient interface + types
│   │   ├── http.go                           # Http impl against sigil-server
│   │   ├── http_test.go                      # httptest fake server
│   │   ├── mock.go                           # Mock impl with fixtures
│   │   ├── mock_test.go
│   │   └── retry.go                          # backoff helper + error mapping
│   ├── httputil/                             # existing
│   ├── triage/                               # NEW
│   │   ├── repo.go                           # CRUD + state log
│   │   ├── repo_test.go
│   │   ├── schema.go                         # migrations as embedded SQL
│   │   └── types.go                          # TriageRow, StatusEnum, NoteRow
│   └── server/                               # existing (embeds web/dist)
├── web/
│   ├── src/
│   │   ├── api/                              # NEW
│   │   │   ├── client.ts                     # typed fetch wrapper
│   │   │   ├── fleet.ts                      # fleet endpoint wrappers
│   │   │   ├── triage.ts                     # triage endpoint wrappers
│   │   │   └── auth.ts                       # login/logout
│   │   ├── components/
│   │   │   ├── AlertsQueue/                  # NEW
│   │   │   │   ├── FilterRow.tsx
│   │   │   │   ├── QueueTable.tsx
│   │   │   │   ├── QueueRow.tsx
│   │   │   │   └── SlideOver.tsx
│   │   │   ├── Layout/                       # NEW
│   │   │   │   ├── TopNav.tsx
│   │   │   │   └── PageShell.tsx
│   │   │   ├── ui/                           # shadcn/ui — extends Plan 01 Button
│   │   │   └── ...
│   │   ├── routes/
│   │   │   ├── __root.tsx                    # MODIFY: add nav + auth guard
│   │   │   ├── _authed/                      # NEW: authed layout group
│   │   │   │   ├── alerts.tsx
│   │   │   │   └── route.tsx                 # layout + guard
│   │   │   ├── login.tsx                     # NEW
│   │   │   └── index.tsx                     # MODIFY: redirect to /alerts
│   │   ├── styles/
│   │   │   └── globals.css                   # MODIFY: tokens per UI/UX §6
│   │   └── hooks/                            # NEW
│   │       ├── useAlerts.ts                  # TanStack Query polling
│   │       ├── useTriage.ts
│   │       └── useShortcuts.ts               # keyboard handler
│   ├── tests/e2e/                            # NEW
│   │   ├── login.spec.ts
│   │   └── alerts.spec.ts
│   ├── playwright.config.ts                  # NEW
│   └── package.json                          # MODIFY: deps + scripts
├── go.mod                                    # MODIFY: 3 new deps
├── go.sum                                    # auto-managed
├── Makefile                                  # MODIFY: add `make e2e`, `make migrate`
└── README.md                                 # MODIFY: env vars + login flow
```

---

## Task 0: Branch setup

**Files:** none (git only).

- [ ] **Step 1:** Verify clean working tree on `main`.
  `git status` should show no modifications. If `go.sum` or
  `internal/server/dist/index.html` are still dirty from prior CI work,
  decide with the user whether to commit or stash before proceeding.
- [ ] **Step 2:** Confirm `main` is up to date with `origin/main`.
  `git fetch origin && git status` — if behind, `git pull --ff-only`.
- [ ] **Step 3:** Create feature branch.
  `git checkout -b feat/plan-02-foundation-and-alerts`.

---

## Task 1: Backend config struct + env loader

**Files:**
- CREATE `internal/config/config.go`
- CREATE `internal/config/config_test.go`
- CREATE `.env.example` at repo root

**Env vars Plan 02 introduces:**

| Var | Required | Default | Purpose |
|---|---|---|---|
| `LISTEN_ADDR` | no | `:8080` | HTTP listen address |
| `SIGIL_SERVER_BASE_URL` | yes (unless `MOCK_FLEET=1`) | — | sigil-server `/v1/*` base, e.g. `http://localhost:9090` |
| `SIGIL_SERVER_READ_TOKEN` | yes (unless `MOCK_FLEET=1`) | — | bearer token per contract F2 |
| `MOCK_FLEET` | no | `0` | `1` swaps in the mock client |
| `TRIAGE_DB_PATH` | no | `./var/triage.sqlite` | SQLite file location |
| `ADMIN_USERNAME` | yes | — | single admin (UI/UX §9) |
| `ADMIN_PASSWORD_BCRYPT` | yes | — | bcrypt hash (`openssl rand -base64 12 \| bcrypt-cli`) |
| `JWT_SECRET` | yes | — | min 32 bytes, used to HS256-sign session JWTs |
| `JWT_TTL_HOURS` | no | `12` | per UI/UX §9 |
| `FLEET_POLL_INTERVAL_SECONDS` | no | `5` | per UI/UX §7.2 (informational; SPA reads from `/api/v1/meta`) |

- [ ] **Step 1:** Define `Config` struct with one field per env var above.
  Use `os.Getenv` + small parsers for ints; reject empty required values
  with a descriptive error naming the env var.
- [ ] **Step 2:** Add `Load()` constructor that returns `(*Config, error)`.
  Validate JWT_SECRET length (≥32 bytes). Validate bcrypt hash by parsing
  the cost field (must start with `$2a$`, `$2b$`, or `$2y$`).
- [ ] **Step 3:** Add `IsMockFleet()` helper that returns true iff
  `MOCK_FLEET=1`. If true, `SIGIL_SERVER_BASE_URL` and
  `SIGIL_SERVER_READ_TOKEN` are optional.
- [ ] **Step 4:** Write `config_test.go` covering: missing required var,
  invalid bcrypt hash, short JWT secret, valid full config, mock-fleet
  config without sigil URL.
- [ ] **Step 5:** Write `.env.example` with all vars + comments + a working
  example admin hash (with a comment that real installs MUST replace it).
- [ ] **Step 6:** Verify: `go test ./internal/config/...` passes.
- [ ] **Step 7:** Commit.

```
feat(config): env loader for admin auth, sigil URL, triage DB

Plan 02 (issue #1). Introduces internal/config.Config covering all env
vars needed for the foundation phase: SIGIL_SERVER_BASE_URL +
_READ_TOKEN, MOCK_FLEET toggle, JWT_SECRET + TTL, ADMIN_USERNAME +
_PASSWORD_BCRYPT, TRIAGE_DB_PATH, LISTEN_ADDR. Required vars fail Load
with a named error; mock mode relaxes sigil URL/token. Adds
.env.example for operator onboarding.

Co-Authored-By: Claude Opus 4.7 (1M context) <noreply@anthropic.com>
```

---

## Task 2: FleetClient interface + Go types

**Files:**
- CREATE `internal/fleet/client.go`

This task is type-only — no implementation. Both `Http` (Task 3) and
`Mock` (Task 4) will satisfy this interface.

- [ ] **Step 1:** Define top-level interface in `client.go`:

```go
type Client interface {
    Meta(ctx context.Context) (*Meta, error)
    Healthz(ctx context.Context) (*Healthz, error)
    PolicyMeta(ctx context.Context) (*PolicyMeta, error)

    Events(ctx context.Context, p EventsParams) (*EventsPage, error)
    EventByID(ctx context.Context, id string) (*Event, error)

    // Plan 03+ surface (declared here so Mock + Http implement them now;
    // wired to handlers in later plans):
    FleetHosts(ctx context.Context, p HostsParams) (*HostsPage, error)
    FleetHostByID(ctx context.Context, id string) (*HostDetail, error)
    FleetRisk(ctx context.Context, p RiskParams) (*RiskPage, error)
    FleetCompliance(ctx context.Context, p ComplianceParams) (*CompliancePage, error)
}
```

- [ ] **Step 2:** Define request param structs (`EventsParams`,
  `HostsParams`, `RiskParams`, `ComplianceParams`) matching contract §5
  query params exactly. Use string slices for repeatable params (e.g.
  `HostIDs []string`).
- [ ] **Step 3:** Define response structs (`Meta`, `Healthz`, `PolicyMeta`,
  `EventsPage`, `HostsPage`, `HostDetail`, `RiskPage`, `CompliancePage`,
  `Event`, `HostSummary`, `HostMeta`, `RiskRow`, `ComplianceRow`,
  `Evidence`, `AiGuard`, etc.) with `json:"..."` tags exact to contract.
- [ ] **Step 4:** `Evidence` MUST be a struct with `Kind string` plus a
  `Raw json.RawMessage` for the full payload, since the wire is
  `#[serde(tag = "kind")]` and we don't want to enumerate all 25+ variants
  in Go. Higher layers parse `Raw` based on `Kind` when needed.
  Specifically expose a `EvidenceAiGuard` helper that decodes
  `AiGuardRiskAssessed` payloads (this is the only variant Plan 02 ships
  UI for).
- [ ] **Step 5:** Add doc comments on every public type with one-liner
  saying which contract §X.Y it implements.
- [ ] **Step 6:** Verify: `go build ./internal/fleet/...` succeeds.
- [ ] **Step 7:** Commit.

```
feat(fleet): Client interface + types per contract v1.0 §5

Plan 02. Defines the consumer-side interface that both Http (Task 3)
and Mock (Task 4) implementations satisfy. Response types mirror
contract §5 response shapes; Evidence carries kind + raw JSON so we
don't enumerate all 25+ variants here. EvidenceAiGuard helper decodes
the only variant Plan 02 renders.

Co-Authored-By: Claude Opus 4.7 (1M context) <noreply@anthropic.com>
```

---

## Task 3: HTTP FleetClient implementation

**Files:**
- CREATE `internal/fleet/http.go`
- CREATE `internal/fleet/http_test.go`
- CREATE `internal/fleet/retry.go`

- [ ] **Step 1:** Write failing tests in `http_test.go` against an
  `httptest.NewServer` that records requests and returns canned JSON
  matching contract §5. Coverage:
  - `Healthz` succeeds with no auth header.
  - All other methods include `Authorization: Bearer <token>`.
  - 401 → typed `ErrUnauthorized` (consumer surfaces this to the
    Settings banner per contract §3.2).
  - 404 on `/v1/meta` → typed `ErrReadAPIDisabled` (per contract §3.4).
  - 503 → typed `ErrServiceUnavailable` with `RetryAfter time.Duration`
    parsed from header (per contract §6 + F15).
  - Query param building matches contract §5.7 — e.g.
    `?host_id=a&host_id=b&since=...&evidence_kind=foo,bar`.
- [ ] **Step 2:** Implement `Http` struct with fields `baseURL`, `token`,
  `httpClient *http.Client` (5s timeout default; configurable).
- [ ] **Step 3:** Helper `doJSON[T any](ctx, method, path, query url.Values,
  body io.Reader) (T, error)` that handles the auth header, response
  status mapping, and JSON decoding.
- [ ] **Step 4:** Implement each `Client` method by calling `doJSON`.
- [ ] **Step 5:** `retry.go` — wrap idempotent GETs with bounded retry:
  3 attempts, exponential backoff with jitter, only on
  `ErrServiceUnavailable` (uses `Retry-After`) and transient network
  errors. **No retry on 401/404** — those are configuration errors.
- [ ] **Step 6:** Verify: `go test ./internal/fleet/...` passes.
- [ ] **Step 7:** Commit.

```
feat(fleet): HTTP client against sigil-server contract v1.0

Plan 02. Implements internal/fleet.Http satisfying the Client
interface. Typed errors for 401 / 404 (read API disabled) / 503 (boot
rebuild). Retry only on 503 + transient network errors; auth and
config errors are surfaced immediately. httptest-backed unit tests
cover happy path + each error mapping.

Co-Authored-By: Claude Opus 4.7 (1M context) <noreply@anthropic.com>
```

---

## Task 4: Mock FleetClient

**Files:**
- CREATE `internal/fleet/mock.go`
- CREATE `internal/fleet/mock_test.go`

- [ ] **Step 1:** Write failing tests asserting Mock returns
  deterministic fixtures with at least:
  - 5 hosts (mix of healthy/stale/disconnected statuses).
  - 30 events spanning the trailing 24h, with a mix of severities and
    evidence kinds (≥5 `ai_guard_risk_assessed` with bucket high/critical
    so the alert queue has rows).
  - 1 host with non-null `host_meta` (HostMetaSnapshot full block) and 1
    with `null` host_meta (host that never emitted snapshot).
  - Cursor pagination over events: `limit=10` returns 10 + cursor,
    walking to completion.
  - **Wire-variant coverage** (per contract §14.5 aggregate checklist —
    keeps Plan 02 rendering exercised against everything sigil has shipped
    through 3b.6.2): at least one `ai_guard_risk_assessed` event for
    **each** of the four current `tool` values (`claude_code`, `codex`,
    `claude_desktop`, `continue_dev`) **and** at least one event each for
    the three `scope.kind` shapes (`user_global`, `project` with a
    non-empty `path`, `application` with `app` set). The four-tool × three-
    scope matrix doesn't need full coverage, but the union of `tool` values
    used and the union of `scope.kind` values used must each be complete.
- [ ] **Step 2:** Implement `Mock` struct holding pre-built fixtures in
  memory. Fixtures live in `mock_fixtures.go` and `mock_fixtures_test.go`
  (split for readability if Mock grows).
- [ ] **Step 3:** `Mock` walks cursor by sorting events by `event_id` desc
  (matches contract §5.7 ordering).
- [ ] **Step 4:** Add `NewMock(seed time.Time)` so tests can pin
  timestamps deterministically.
- [ ] **Step 5:** Add `internal/fleet/factory.go` that returns
  `Client` based on `config.IsMockFleet()`.
- [ ] **Step 6:** Verify: `go test ./internal/fleet/...` passes; both
  implementations pass the same suite of interface-level tests (write a
  shared `TestClientContract(t, c Client)` helper invoked from both
  `http_test.go` and `mock_test.go`).
- [ ] **Step 7:** Commit.

```
feat(fleet): Mock client + factory toggled by MOCK_FLEET env

Plan 02. internal/fleet.Mock holds in-memory fixtures (5 hosts, 30
events, mixed evidence/severity, one HostMetaSnapshot) so the SPA
can iterate without running sigil-server locally. Same interface
contract as Http; a shared test helper runs the same checks against
both implementations.

Co-Authored-By: Claude Opus 4.7 (1M context) <noreply@anthropic.com>
```

---

## Task 5: Triage SQLite + repo

**Files:**
- CREATE `internal/triage/schema.go`
- CREATE `internal/triage/types.go`
- CREATE `internal/triage/repo.go`
- CREATE `internal/triage/repo_test.go`
- MODIFY `go.mod` (add `modernc.org/sqlite`)

Schema (embedded SQL string in `schema.go`):

```sql
CREATE TABLE IF NOT EXISTS triage (
  host_id     TEXT NOT NULL,
  event_id    TEXT NOT NULL,
  status      TEXT NOT NULL CHECK(status IN
              ('open','acknowledged','investigating','resolved')),
  assignee    TEXT,
  evidence_snapshot TEXT NOT NULL,   -- raw JSON, per contract §13 retention mitigation
  created_at  TEXT NOT NULL,
  updated_at  TEXT NOT NULL,
  PRIMARY KEY (host_id, event_id)
);

CREATE INDEX IF NOT EXISTS triage_status_idx ON triage(status, updated_at DESC);

CREATE TABLE IF NOT EXISTS triage_notes (
  id          INTEGER PRIMARY KEY AUTOINCREMENT,
  host_id     TEXT NOT NULL,
  event_id    TEXT NOT NULL,
  author      TEXT NOT NULL,
  body        TEXT NOT NULL,
  created_at  TEXT NOT NULL,
  FOREIGN KEY (host_id, event_id) REFERENCES triage(host_id, event_id)
);

CREATE TABLE IF NOT EXISTS triage_log (
  id          INTEGER PRIMARY KEY AUTOINCREMENT,
  host_id     TEXT NOT NULL,
  event_id    TEXT NOT NULL,
  actor       TEXT NOT NULL,
  from_status TEXT,
  to_status   TEXT,
  at          TEXT NOT NULL
);
```

- [ ] **Step 1:** `go get modernc.org/sqlite@latest`. Pin the version in
  `go.mod`.
- [ ] **Step 2:** Write failing tests in `repo_test.go` against an
  in-memory database (`sqlite://file::memory:?cache=shared`). Cover:
  - `Upsert(ctx, TriageRow)` creates a new row with `status=open`.
  - `Upsert` updates status + assignee + bumps `updated_at`, appends a
    `triage_log` row.
  - `GetByEventKey(ctx, host_id, event_id)` returns the row or
    `ErrNotFound`.
  - `ListByStatus(ctx, statuses []string, limit, cursor)` returns paged
    results ordered by `updated_at DESC`.
  - `AppendNote` adds rows; `ListNotes` returns chronological.
  - `evidence_snapshot` round-trips raw JSON.
- [ ] **Step 3:** Implement `Repo` struct with constructor
  `Open(path string) (*Repo, error)` that runs `schema.go` SQL on open
  (idempotent — uses `CREATE TABLE IF NOT EXISTS`).
- [ ] **Step 4:** Migration story is "embed SQL + run on every open".
  Note in repo doc that v1 has no schema versioning; renaming columns is
  out of scope until we have more than one schema.
- [ ] **Step 5:** Verify: `go test ./internal/triage/...` passes.
- [ ] **Step 6:** Commit.

```
feat(triage): SQLite repo for alert ack/assign/resolve/notes

Plan 02. internal/triage.Repo persists ack state, assignee, notes, and
a state-change log keyed by (host_id, event_id) per the contract §1
guidance about EventUnprocessableLocal orphans. evidence_snapshot
column stores the original event payload so triage rows outlive
server-side JSONL retention (contract §13 mitigation). modernc.org
SQLite means zero cgo — pure Go build.

Co-Authored-By: Claude Opus 4.7 (1M context) <noreply@anthropic.com>
```

---

## Task 6: Auth (bcrypt + JWT + middleware)

**Files:**
- CREATE `internal/auth/password.go`
- CREATE `internal/auth/password_test.go`
- CREATE `internal/auth/jwt.go`
- CREATE `internal/auth/jwt_test.go`
- MODIFY `go.mod` (`github.com/golang-jwt/jwt/v5`, `golang.org/x/crypto`)

- [ ] **Step 1:** `go get github.com/golang-jwt/jwt/v5
  golang.org/x/crypto/bcrypt`.
- [ ] **Step 2:** `password.go` exposes
  - `Verify(plaintext, hash string) bool` — wraps
    `bcrypt.CompareHashAndPassword`, returns `false` on any error.
  - `Hash(plaintext string) (string, error)` — convenience for tests +
    operator tooling.
- [ ] **Step 3:** `jwt.go` exposes
  - `type Signer struct { secret []byte; ttl time.Duration }`.
  - `NewSigner(secret string, ttl time.Duration) (*Signer, error)`
    (validates secret ≥32 bytes).
  - `Sign(subject string) (string, time.Time, error)` returns
    `(token, expiresAt, error)` with `subject` set to admin username,
    `iat`/`exp` claims.
  - `Verify(token string) (subject string, err error)` returns
    `ErrExpired`/`ErrInvalid` for the two failure modes.
- [ ] **Step 4:** Failing tests in both `_test.go` files. Bcrypt: a known
  hash verifies for the known password and not for any other. JWT:
  round-trip; tamper one byte → invalid; advance clock past `exp` →
  expired; short secret → constructor errors.
- [ ] **Step 5:** Verify: `go test ./internal/auth/...` passes.
- [ ] **Step 6:** Commit.

```
feat(auth): bcrypt password + JWT signer for single admin

Plan 02. internal/auth.Signer issues HS256-signed JWTs (12h default,
configurable via JWT_TTL_HOURS) and verifies them. internal/auth
password helpers wrap bcrypt so the rest of the codebase only sees
Verify(plaintext, hash). Per UI/UX §9 this is the only auth in v1 —
no refresh, no OAuth, no SAML.

Co-Authored-By: Claude Opus 4.7 (1M context) <noreply@anthropic.com>
```

---

## Task 7: API handlers + auth middleware

**Files:**
- CREATE `internal/api/v1/auth.go` + `auth_test.go`
- CREATE `internal/api/v1/fleet.go` + `fleet_test.go`
- CREATE `internal/api/v1/triage.go` + `triage_test.go`
- CREATE `internal/api/v1/middleware.go`
- MODIFY `internal/server/server.go` (mount `/api/v1` chi router)

The `/api/v1` namespace contains exactly what the SPA needs in this
plan. It is a thin wrapper around `FleetClient` + `TriageRepo`. We do
NOT proxy sigil-server URLs 1:1 — the manager is allowed to compose,
filter, and project.

**Endpoints in scope of Plan 02:**

| Method | Path | Auth | Backed by |
|---|---|---|---|
| POST | `/api/v1/auth/login` | none | `auth.Signer` + `auth.Verify` |
| POST | `/api/v1/auth/logout` | cookie | clears cookie |
| GET  | `/api/v1/auth/me` | cookie | returns `{username, expiresAt}` |
| GET  | `/api/v1/fleet/meta` | cookie | `FleetClient.Meta` (relayed verbatim) |
| GET  | `/api/v1/fleet/healthz` | cookie | `FleetClient.Healthz` |
| GET  | `/api/v1/fleet/events` | cookie | `FleetClient.Events` with passthrough query params; **enriched with `triage` block per event** |
| GET  | `/api/v1/fleet/events/:id` | cookie | `FleetClient.EventByID` with triage |
| POST | `/api/v1/triage/upsert` | cookie | upserts status/assignee, logs the transition |
| POST | `/api/v1/triage/note` | cookie | appends note |
| GET  | `/api/v1/triage/:host_id/:event_id` | cookie | full triage row + notes + log |

The "enriched" events endpoint joins each event with its triage row (if
any) so the queue UI can render the assignee + status pill without a
second fetch per row.

- [ ] **Step 1:** `middleware.go` exposes
  `RequireAuth(signer *auth.Signer) func(http.Handler) http.Handler`.
  It reads the `sigil_session` cookie, verifies JWT, stuffs the
  `subject` into the request context, returns `401 unauthorized` JSON on
  invalid/missing/expired.
- [ ] **Step 2:** `auth.go` implements POST `/login` (body
  `{"username":"...","password":"..."}` → checks against
  `config.AdminUsername` + `config.AdminPasswordBcrypt`, sets HTTP-only
  `sigil_session` cookie with the JWT; 60s timing-equalization sleep on
  failure to dodge user-enumeration). `/logout` clears the cookie.
  `/me` returns `{username, expiresAt}`.
- [ ] **Step 3:** `fleet.go` translates HTTP query strings into
  `FleetClient` params, calls the client, decorates the response with
  triage state.
- [ ] **Step 4:** `triage.go` implements upsert/note/get with strict
  validation (status in enum, `host_id`/`event_id` non-empty,
  `actor = subject from ctx`).
- [ ] **Step 5:** Failing tests for each handler using `httptest` +
  `fleet.Mock` + in-memory triage repo. Auth tests cover wrong creds,
  expired cookie, missing cookie. Fleet tests cover query passthrough
  and triage join. Triage tests cover the state machine
  (`open→ack→investigating→resolved` and back).
- [ ] **Step 6:** Wire `/api/v1` into `internal/server/server.go` under
  the existing chi router. `/api/v1/auth/login` is the only
  unauthenticated route in this prefix.
- [ ] **Step 7:** Verify: `make test` passes; `make build` builds.
- [ ] **Step 8:** Commit.

```
feat(api): /api/v1 wires fleet client + triage repo + JWT cookie auth

Plan 02. Adds the consumer-side HTTP surface the SPA calls:
- /api/v1/auth/{login,logout,me} — cookie JWT issued from bcrypt admin creds
- /api/v1/fleet/{meta,healthz,events,events/:id} — passthrough to FleetClient
  with triage join on event responses
- /api/v1/triage/{upsert,note,:host_id/:event_id} — local triage state CRUD

All but /login require the sigil_session cookie. fleet.Mock keeps tests
hermetic. State machine enforced server-side.

Co-Authored-By: Claude Opus 4.7 (1M context) <noreply@anthropic.com>
```

---

## Task 8: Wire main.go — config → repo → fleet → server

**Files:**
- MODIFY `cmd/sigil-manager/main.go`

- [ ] **Step 1:** Replace existing `main` body with: load config, open
  triage repo, build fleet client (Http or Mock based on
  `config.IsMockFleet()`), build auth signer, build chi router with
  `/api/health` (existing) + `/api/v1/...` (new) + SPA static
  fallback (existing `internal/server` package).
- [ ] **Step 2:** Add graceful shutdown — context cancellation on
  SIGINT/SIGTERM, `http.Server.Shutdown` with 5s timeout, triage repo
  `Close()`.
- [ ] **Step 3:** Smoke test by hand: `MOCK_FLEET=1
  ADMIN_USERNAME=admin
  ADMIN_PASSWORD_BCRYPT='$2a$10$...'
  JWT_SECRET='aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa'
  make dev` → `curl -X POST localhost:8080/api/v1/auth/login -d
  '{"username":"admin","password":"..."}'` returns cookie; `curl
  -b "sigil_session=..." localhost:8080/api/v1/fleet/events`
  returns the mock fixtures.
- [ ] **Step 4:** Verify: `make test` passes; `make dev` boots.
- [ ] **Step 5:** Commit.

```
feat(main): wire config + auth + fleet + triage into the binary

Plan 02. main.go now loads internal/config, opens the triage repo,
builds Http or Mock fleet client based on MOCK_FLEET, builds the
JWT signer, and mounts everything under chi. Graceful shutdown on
SIGINT/SIGTERM. /api/health (Plan 01) stays as the liveness probe;
/api/v1/* is the SPA's surface.

Co-Authored-By: Claude Opus 4.7 (1M context) <noreply@anthropic.com>
```

---

## Task 9: Frontend design tokens + shadcn/ui additions

**Files:**
- MODIFY `web/src/styles/globals.css`
- MODIFY `web/tailwind.config.ts` (if present) or whatever Tailwind v4
  config Plan 01 chose
- ADD shadcn/ui components: `sheet`, `input`, `label`, `badge`, `table`,
  `tabs`, `dialog`, `sonner`, `dropdown-menu`

- [ ] **Step 1:** Map all UI/UX §6 tokens into `globals.css` as CSS
  variables on `:root` + `.dark`:
  - `--bg-page`, `--bg-surface`, `--bg-elevated`, `--border-subtle`,
    `--border`
  - `--text-primary`, `--text-body`, `--text-muted`, `--text-subtle`
  - `--sev-critical`, `--sev-high`, `--sev-medium`, `--sev-low`,
    `--sev-info`
  - `--status-healthy`, `--status-degraded`, `--status-down`, `--accent`
  - Spacing/radius constants (`--row-height`, `--radius-row`,
    `--radius-panel`)
  - Critical-only glow: `--glow-critical`.
- [ ] **Step 2:** Tell Tailwind v4 about the tokens via the `@theme`
  directive so utility classes (`bg-page`, `border-subtle`, etc.) work.
- [ ] **Step 3:** Add fonts: `Inter` (sans) + `JetBrains Mono` (mono).
  Use `next/font` equivalent (Vite + `@fontsource/inter` and
  `@fontsource/jetbrains-mono`).
- [ ] **Step 4:** Install the 9 shadcn/ui components above with
  `npx shadcn@latest add ...`. **Adjust their default tokens** to use
  our CSS variables (the generator defaults to `--background`,
  `--foreground` — we need to map those to `--bg-page`, `--text-body`).
- [ ] **Step 5:** Verify: `npm --prefix web run build` succeeds; visual
  spot-check by booting `make dev` and inspecting the body background +
  fonts in DevTools.
- [ ] **Step 6:** Commit.

```
feat(web): design tokens per UI/UX §6 + shadcn/ui primitives

Plan 02. Adds the dark-only token system from UI/UX spec §6 (bg,
text, severity hues incl. critical glow, status accent) as CSS vars
in globals.css and wires them into Tailwind v4 via @theme. Installs
shadcn/ui sheet/input/label/badge/table/tabs/dialog/sonner/
dropdown-menu and rewires their default tokens onto ours. Fonts:
Inter + JetBrains Mono.

Co-Authored-By: Claude Opus 4.7 (1M context) <noreply@anthropic.com>
```

---

## Task 10: App shell + top nav + routing + auth guard

**Files:**
- MODIFY `web/src/routes/__root.tsx`
- CREATE `web/src/routes/login.tsx`
- CREATE `web/src/routes/_authed/route.tsx`
- MODIFY `web/src/routes/index.tsx`
- CREATE `web/src/components/Layout/TopNav.tsx`
- CREATE `web/src/components/Layout/PageShell.tsx`
- CREATE `web/src/api/client.ts`
- CREATE `web/src/api/auth.ts`

- [ ] **Step 1:** `api/client.ts` wraps `fetch` with credentials
  `include`, a base URL of `/api/v1`, JSON parse, error-mapping to
  typed errors (`UnauthorizedError`, `NotFoundError`, etc.). Throws on
  non-2xx.
- [ ] **Step 2:** `api/auth.ts`: `login()`, `logout()`, `me()`.
- [ ] **Step 3:** TanStack Router setup: per-route data loaders, error
  boundaries. The `_authed` layout group is the auth guard: its
  `beforeLoad` calls `me()` and, on `UnauthorizedError`, throws
  `redirect({to: '/login'})`.
- [ ] **Step 4:** `TopNav.tsx`: brand mark `◆ sigil` + nav items
  (`Alerts` link visible in this plan; `Fleet`, `Settings` are stubs
  saying "Coming in Plan 03/04"). Right side: connection-state pill
  (green/yellow/red dot + "Connected" / "Stale" / "Disconnected" — read
  from `useQuery(['fleetHealthz'])` polling every 10s) + logout button.
- [ ] **Step 5:** `PageShell.tsx`: container with `bg-page`, max-width
  wrap, padding per UI/UX §6.4.
- [ ] **Step 6:** `login.tsx`: dark-themed login card (username/password
  inputs, submit button, error toast on 401). Match UI/UX visual style.
- [ ] **Step 7:** `/` (existing index route) redirects to `/alerts` if
  authed, otherwise `/login`.
- [ ] **Step 8:** Verify: in browser, `make dev` → visit `/` → see
  `/login` → submit creds → land on `/alerts` (empty for now — Tasks
  11-12 fill it). Try expired cookie (manually shorten `JWT_TTL_HOURS=0`,
  re-login, wait > 1s, refresh) → redirected to `/login`.
- [ ] **Step 9:** Commit.

```
feat(web): app shell + top nav + login flow + _authed router guard

Plan 02. SPA chrome per UI/UX §4 + §7. TanStack Router _authed layout
group blocks anonymous access by calling /api/v1/auth/me before render.
TopNav has connection-state pill polling /api/v1/fleet/healthz every
10s and a logout button. /login is the only public route. / redirects.

Co-Authored-By: Claude Opus 4.7 (1M context) <noreply@anthropic.com>
```

---

## Task 11: Alerts queue UI (rows + filter + polling)

**Files:**
- CREATE `web/src/routes/_authed/alerts.tsx`
- CREATE `web/src/components/AlertsQueue/FilterRow.tsx`
- CREATE `web/src/components/AlertsQueue/QueueTable.tsx`
- CREATE `web/src/components/AlertsQueue/QueueRow.tsx`
- CREATE `web/src/api/fleet.ts`
- CREATE `web/src/hooks/useAlerts.ts`

- [ ] **Step 1:** `api/fleet.ts` exposes typed wrappers for
  `getMeta()`, `getEvents(params)`, `getEventById(id)`. Types mirror the
  Go `EventsParams`/`EventsPage` shapes from Task 2.
- [ ] **Step 2:** `useAlerts.ts`: TanStack Query hook that builds the
  alert query from `/v1/meta.alerts_definition_default` (cached for the
  session) plus current filter state. Calls
  `/api/v1/fleet/events?evidence_kind=...&min_ai_guard_bucket=high&...`
  with the configured kinds. Polls every 5s (UI/UX §7.2). Pauses
  polling while user hovers a row (`onMouseEnter`/`onMouseLeave` ref
  passed in) to avoid sort jitter.
- [ ] **Step 3:** `FilterRow.tsx`: severity chips, status chips
  (open/ack/resolved — these read from joined triage state), time range
  selector (`24h` / `7d` / `custom`), search input (client-side filter
  only in v1 per contract §13). All filter state goes to URL params.
- [ ] **Step 4:** `QueueTable.tsx`: scrollable list at 28px row height
  (UI/UX §6.4). Sticky header. Empty/loading/error per UI/UX §8.
- [ ] **Step 5:** `QueueRow.tsx`: severity dot (critical has glow per
  UI/UX §6.2) + relative age via `date-fns/formatDistanceToNow` +
  evidence title (computed: `evidence.kind` snake_case → Title Case for
  display, `AiGuardRiskAssessed` becomes "AI Guard risk:
  {top_reason.kind}" using the joined event payload) + host
  (hostname if present, else host_id short) + severity label + assignee
  pill (from triage join).
- [ ] **Step 6:** Header right side: "Updated Ns ago" timestamp + color
  per UI/UX §7.2 / §8.4 (gray < 30s, yellow 30-60s, red > 60s).
- [ ] **Step 7:** Sort: default severity desc → age desc. Column header
  click toggles.
- [ ] **Step 8:** New critical alert at top gets a 1-second highlight
  animation (CSS transition on `box-shadow` + critical glow color
  fading).
- [ ] **Step 9:** Verify: `make dev` with `MOCK_FLEET=1` → log in → see
  the seeded mock alerts in the queue, observe polling refresh.
- [ ] **Step 10:** Commit.

```
feat(web): Alerts queue — rows, filter, polling, sort

Plan 02. /alerts landing screen per UI/UX D2. useAlerts hook polls
/api/v1/fleet/events every 5s (pauses while hovering a row), reads
alerts definition from /api/v1/fleet/meta, applies severity/status/
time/search filters to URL state. QueueTable at 28px density per
UI/UX §6.4. Critical severity gets glow + 1s highlight on arrival.

Co-Authored-By: Claude Opus 4.7 (1M context) <noreply@anthropic.com>
```

---

## Task 12: Alert slide-over + triage actions + keyboard shortcuts

**Files:**
- CREATE `web/src/components/AlertsQueue/SlideOver.tsx`
- CREATE `web/src/hooks/useTriage.ts`
- CREATE `web/src/hooks/useShortcuts.ts`
- CREATE `web/src/api/triage.ts`

- [ ] **Step 1:** `api/triage.ts`: `getTriage(host_id, event_id)`,
  `upsertTriage(payload)`, `appendNote(payload)`.
- [ ] **Step 2:** `useTriage.ts`: TanStack Query for the slide-over's
  triage details, mutations for upsert/note with optimistic updates +
  invalidation of `useAlerts` so the queue row reflects the new state
  immediately.
- [ ] **Step 3:** `SlideOver.tsx` using shadcn `Sheet`: opens from
  right (40% viewport per UI/UX §5.1), header with alert title + close
  (✕), body sections (Host, Rule ID, Confidence, Time, Status, raw
  payload in `<pre>` with mono font, Assignee, Notes), and Actions
  (`Assign` (focus-and-edit assignee field), `Acknowledge`, `Resolve`).
  Each action calls a triage mutation and updates the status badge.
- [ ] **Step 4:** URL routing: open via `?alert=:event_id` (UI/UX §7.4).
  Clicking a queue row pushes the param; closing removes it.
- [ ] **Step 5:** `useShortcuts.ts`: global keydown handler. Bindings
  per UI/UX §7.1:
  - `j`/`k` selection movement in queue.
  - `Enter` open selected alert.
  - `Esc` close slide-over.
  - `a` assign (focuses assignee field in slide-over).
  - `r` resolve.
  - `c` acknowledge.
  - `/` focuses search.
  - `g a` / `g f` / `g s` navigate (vim-style leader).
  - `?` opens shortcut cheatsheet (a simple dialog listing all
    bindings).
  Ignored when focus is inside `<input>`/`<textarea>` unless the
  binding is `Esc`.
- [ ] **Step 6:** Verify in browser: open a critical alert → slide-over
  appears → set assignee = "alice" → press `c` to acknowledge → queue
  row shows the ack pill + assignee. Refresh page; state persists.
- [ ] **Step 7:** Commit.

```
feat(web): alert slide-over + triage actions + keyboard shortcuts

Plan 02. Right-side Sheet panel at 40% viewport per UI/UX §5.1 with
ack/assign/resolve actions wired through /api/v1/triage. Optimistic
mutations + queue invalidation so the row updates immediately. Full
keyboard shortcut set from UI/UX §7.1 (j/k/Enter/Esc/a/r/c, '/' for
search, 'g a/f/s' leader nav, '?' cheatsheet). Shortcuts respect input
focus.

Co-Authored-By: Claude Opus 4.7 (1M context) <noreply@anthropic.com>
```

---

## Task 13: States — empty / loading / error / stale-data

**Files:**
- MODIFY `web/src/components/AlertsQueue/QueueTable.tsx`
- MODIFY `web/src/components/AlertsQueue/SlideOver.tsx`
- MODIFY `web/src/components/Layout/TopNav.tsx`

- [ ] **Step 1:** Empty states per UI/UX §8.1:
  - Filters return 0 → "No matching alerts in last 24h" + reset link.
  - No open alerts at all → "🎉 No open alerts. Last incident was N
    days ago." (compute from joined triage data).
- [ ] **Step 2:** Loading states per UI/UX §8.2:
  - Initial queue load: 5 skeleton rows (28px each, animated shimmer).
  - Polling refresh: invisible — only "Updated Ns ago" updates.
  - Slide-over open: header immediate, body skeleton blocks.
- [ ] **Step 3:** Error states per UI/UX §8.3:
  - Lost connection (`/fleet/healthz` consistently fails or returns
    503): TopNav banner "Lost connection to sigil-server. Retrying every
    10s..." with manual retry; queue continues to show last-cached data
    with stale indicator.
  - 401 anywhere → redirect to `/login`.
  - 500/unknown → inline error in affected row + retry button; rest of
    page stays usable.
- [ ] **Step 4:** Stale data per UI/UX §8.4:
  - `Updated <30s`: gray.
  - `30–60s`: yellow.
  - `>60s`: red + banner.
- [ ] **Step 5:** Verify by toggling `MOCK_FLEET=0` with no
  sigil-server reachable → expect the connection-lost banner + stale
  indicator + last-cached data still rendered.
- [ ] **Step 6:** Commit.

```
feat(web): empty / loading / error / stale-data states per UI/UX §8

Plan 02. AlertsQueue handles all four state families per spec:
skeleton rows on initial load, "Updated Ns ago" coloring (gray/yellow/
red), banner on lost connection (10s retry + manual button), inline
row error + retry on partial failures, 401 → /login, and the no-
incidents-yet celebration empty state.

Co-Authored-By: Claude Opus 4.7 (1M context) <noreply@anthropic.com>
```

---

## Task 14: E2E test setup + golden flows

**Files:**
- CREATE `web/playwright.config.ts`
- CREATE `web/tests/e2e/login.spec.ts`
- CREATE `web/tests/e2e/alerts.spec.ts`
- MODIFY `web/package.json` (add `@playwright/test` + `e2e` script)
- MODIFY `Makefile` (add `make e2e`)

- [ ] **Step 1:** `npm --prefix web i -D @playwright/test &&
  npx playwright install chromium`.
- [ ] **Step 2:** Playwright config: web server points at the Go binary
  built with `MOCK_FLEET=1`, dev creds set in `webServer.env`, base URL
  `http://localhost:8080`.
- [ ] **Step 3:** `login.spec.ts`: visit `/`, expect redirect to
  `/login`. Type creds, submit, expect redirect to `/alerts`. Refresh,
  expect still on `/alerts`. Click logout, expect back to `/login`.
- [ ] **Step 4:** `alerts.spec.ts`: log in. Expect ≥3 alert rows.
  Click first row, expect slide-over with assignee field. Type "alice",
  press `c` (acknowledge), expect status pill = "Acknowledged". Refresh
  page, expect ack persists. Apply severity filter "critical only",
  expect row count reduced. Press `?`, expect shortcut cheatsheet
  dialog.
- [ ] **Step 5:** `make e2e` runs `go build` + starts the binary + runs
  `playwright test`.
- [ ] **Step 6:** Verify: `make e2e` passes locally.
- [ ] **Step 7:** Commit.

```
test(e2e): Playwright login + alerts triage golden flows

Plan 02. End-to-end tests run against the Go binary in MOCK_FLEET=1
mode. login.spec.ts covers redirect + login + logout + cookie
persistence. alerts.spec.ts covers slide-over open, assignee field,
keyboard acknowledge, persisted triage across refresh, severity
filter, and the '?' cheatsheet dialog. `make e2e` is the one-line
runner.

Co-Authored-By: Claude Opus 4.7 (1M context) <noreply@anthropic.com>
```

---

## Task 15: CI update + README + ship checklist

**Files:**
- MODIFY `.github/workflows/ci.yml`
- MODIFY `README.md`
- MODIFY `.env.example` (final pass)

- [ ] **Step 1:** Add an E2E job to CI: matrix is just `ubuntu-22.04`
  (Playwright Linux is well-supported). Steps: Node + Go setup; cache
  Playwright browsers; `make build`; `make e2e`. Uploads `playwright-
  report/` as an artifact on failure.
- [ ] **Step 2:** Existing test/lint/build jobs unchanged.
- [ ] **Step 3:** README sections to update or add:
  - "Configuration" — full env var table from Task 1 + how to generate
    a bcrypt hash + how to generate a 32-byte JWT secret
    (`openssl rand -base64 32`).
  - "Quickstart (mock mode)" — `cp .env.example .env`, set creds,
    `MOCK_FLEET=1 make dev`, log in.
  - "Quickstart (connected to sigil-server)" — same but unset
    `MOCK_FLEET`, set `SIGIL_SERVER_BASE_URL` + `_READ_TOKEN`. Link to
    sigil-server README on how to enable the read API.
  - "What's in Plan 02" — short summary linking to this plan file.
- [ ] **Step 4:** Update `.env.example` with the final variable set
  matching what's actually loaded by `internal/config`.
- [ ] **Step 5:** Verify: CI green on the feature branch's PR.
- [ ] **Step 6:** Commit.

```
ci+docs: Playwright e2e leg, README env config, .env.example

Plan 02. CI now runs Playwright on ubuntu-22.04 against the MOCK_FLEET
binary, uploads the report on failure. README gains Configuration +
Quickstart sections covering both mock and connected modes, plus the
bcrypt/JWT secret generation snippets. .env.example mirrors
internal/config.

Co-Authored-By: Claude Opus 4.7 (1M context) <noreply@anthropic.com>
```

---

## Verification (end state of Plan 02)

A developer on a clean clone should be able to:

1. Copy `.env.example` to `.env`, fill in admin creds + JWT secret.
2. Run `MOCK_FLEET=1 make dev`.
3. Visit `http://localhost:8080/`, get redirected to `/login`.
4. Log in with the env-supplied creds, land on `/alerts`.
5. See ≥3 mock alerts with mixed severities at 28px row density.
6. Click an alert → slide-over from the right at 40% viewport.
7. Type an assignee, press `c` to acknowledge, see the queue row
   update.
8. Refresh the browser — triage state persists.
9. Stop the server, observe the connection banner appear within 30s.
10. Restart the server, observe the banner disappear and the queue
    refresh.

CI is green. `make test`, `make lint`, `make build`, `make e2e` all
pass locally.

---

## What's out of Plan 02 (handled by future plans)

- **Plan 03 — Fleet pages:** `/fleet/risk`, `/fleet/events`,
  `/fleet/compliance` tabs. Consumes the same `FleetClient`; reuses the
  shell + tokens from Plan 02.
- **Plan 04 — Host detail:** `/hosts/:hostname` route with
  `Alerts/Events/Compliance` tabs. Adds hostname-to-host_id resolver
  (per contract §9 mapping note).
- **Plan 05 — Settings:** real settings page with connection URL
  display, retention display, future re-config UI.
- **Plan 06 — Polish:** light theme (deferred per UI/UX §6.1),
  responsive tweaks, broader keyboard shortcut coverage,
  accessibility audit.

Each follow-up plan should fit on top of this one without modifying
the auth, fleet client, or triage repo APIs. Those are intended as
stable interfaces for the rest of the console.

---

## Risks + mitigations

| Risk | Mitigation |
|---|---|
| `sigil-server 0.5.0` not tagged yet at Plan 02 start | `MOCK_FLEET=1` makes the entire Plan 02 implementable + testable without sigil-server. Connected mode is verified once `0dce160` ≤ `main` of sigil. |
| `open_alert_count_24h` returns `sum_warn()` instead of alerts-definition-filtered (contract §13.1 follow-up) | The Alerts queue uses `/api/v1/fleet/events` directly with `evidence_kind` + `min_ai_guard_bucket` filters — does NOT rely on the rolled-up count. The count is only displayed in the (not-yet-built) Fleet Risk row, so this risk lands in Plan 03 not Plan 02. |
| shadcn/ui Sheet's default tokens don't match our palette | Task 9 explicitly rewires shadcn's CSS variables onto our tokens. Visual QA at the end of Task 9 catches mismatches before any feature work. |
| Playwright on macOS Apple Silicon is flakier than Linux | CI runs only on Linux; locally devs can `make test` (Go) + `npm test` (Vitest) without `make e2e`. E2E is gating only on CI. |
| Triage SQLite at `./var/triage.sqlite` is writeable in dev but not in a stock container | README's deploy guidance recommends mounting `/var/lib/sigil-manager` as a volume. Plan 02 doesn't ship the container image — that's a follow-up. |

---

## Tracking

This plan implements Issue
[#1 (Plan 02 epic)](https://github.com/Ju571nK/sigil-manager/issues/1).
When Plan 02 lands, the epic body should be updated to:
- Move "Tasks" list to "Done" with links to each commit.
- Add a note pointing to Plan 03 (Fleet pages) as the next chunk.
- Status: closed (or kept open as a tracker for Plans 03–06 if we
  prefer that).
