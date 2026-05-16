# sigil-manager UI/UX Design Spec

- **Status**: Brainstorm complete · awaiting implementation plan
- **Date**: 2026-05-16
- **Owner**: Justin Kwon
- **Source**: Brainstorming session 2026-05-16 (Visual Companion + terminal)
- **Implements**: Strategy decisions in `sigil-strategy.md`

## 1. Context

`sigil-manager` is the self-hostable web console layer of the Sigil AI-SPM
project (middle tier between `sigil` daemon and `sigil-cloud` SaaS). This
document captures the UI/UX direction for v1.

The brainstorm targeted two user requirements:
1. **Professional as a security solution** — looks like a real security tool, not a SaaS demo.
2. **Easy to use** — a SOC analyst can sit down and triage immediately.

## 2. Goals & non-goals

### Goals (v1)

- A SOC analyst can open the console, see all open alerts, and triage each one (assign, acknowledge, resolve, note) without leaving the queue.
- An analyst can pivot from an alert to the host's context (recent events, policy state, other alerts on that host) in one click.
- A SOC analyst can browse the fleet sorted by AI Guard risk and find concerning hosts proactively.
- The console can run as a single `docker run` next to a user's existing `sigil-server`, with minimal configuration.

### Non-goals (v1, explicitly)

See section 11 "Out of scope" below.

## 3. Decisions log

Seven explicit decisions made during brainstorm. Each is binding for v1 — changing them invalidates downstream design.

| # | Decision | Choice | Alternatives considered |
|---|---|---|---|
| D1 | Primary user persona | **SOC analyst / incident responder** | DevSecOps engineer; CISO; mixed |
| D2 | Landing screen | **Alert queue** | Live event feed; fleet posture map; triage workbench |
| D3 | Visual personality | **Modern security product base (Panther/Datadog) + Industrial SOC density (Splunk ES)** | Pure industrial; pure quiet/terminal |
| D4 | Drill-down pattern | **Slide-over panel** from right when alert is clicked | Full-page detail; workbench tabs |
| D5 | Read-only model | **Fleet data is read from `sigil-server`; triage state (ack/assign/resolve/notes) is owned by `sigil-manager` in its own DB** | Pure read-only; strategy doc updated accordingly |
| D6 | MVP scope | **Alerts + Fleet (with Risk/Events/Compliance tabs) + Host detail + Settings** | Smaller (alerts only); wider (separate Events/Compliance pages) |
| D7 | Navigation chrome | **Top nav** (horizontal tabs) | Slim icon sidebar; expanded sidebar |

## 4. Information architecture

```
sigil-manager
├── /alerts                   (D2: landing)
│   └── ?alert=:id            → slide-over panel (D4)
├── /fleet
│   ├── /fleet/risk           (default tab)
│   ├── /fleet/events
│   └── /fleet/compliance
├── /hosts/:hostname          → Host detail
│   ├── tab: Alerts (default)
│   ├── tab: Events
│   └── tab: Compliance
└── /settings
```

**Nav order** (D7, top nav): `◆ sigil` · Alerts · Fleet · Settings.
`/hosts/:hostname` lives under Fleet conceptually (breadcrumb shows `Fleet / hostname`) but is reachable from both Fleet and the alert slide-over.

## 5. Key screens

### 5.1 Alerts (`/alerts`)

The landing screen. A SOC analyst opens the console and is immediately in triage.

**Layout** (single column, top nav above):
- Filter row: severity chips, status (open/ack/resolved), time range, search.
- Queue: dense rows at 28px height. Each row shows:
  - Severity dot (D3: critical has glow, others flat) · age · alert title · host · severity label · assignee (if any).
- Sort: severity desc → age desc by default. Sortable columns.
- Updated timestamp in header ("Updated 3s ago").

**Slide-over panel** (D4): clicking a row opens a panel from the right (~40% of viewport). Queue compresses but remains visible.
- Header: alert title + close (✕).
- Body: host (linked, violet accent), rule ID (mono), confidence score, time, status, raw payload excerpt (mono, in a subtle code block), assignee, notes.
- Actions: `Assign`, `Acknowledge`, `Resolve`. Each updates triage state on `sigil-manager` (D5).

If users find the slide-over too cramped for deep investigation, a full-page `/alerts/:id` view is the v1.1 escape hatch. v1 keeps the slide-over as the only detail surface to limit scope.

### 5.2 Fleet (`/fleet`)

Three tabs serve the dashboards promised by the strategy doc:

- **Risk** (default): host list sorted by AI Guard risk score. Each row shows risk bar (color-coded to severity scale), hostname, score (0-100), open alert count.
- **Events**: fleet-wide event timeline. Reverse-chronological list of events from all hosts. Filter by event type, host, time range.
- **Compliance**: per-host policy compliance. Each row: hostname, compliance score, count of failing policies, last policy update.

Clicking any hostname → `/hosts/:hostname`.

### 5.3 Host detail (`/hosts/:hostname`)

Drilled-in view of one host.

**Layout**:
- Breadcrumb: `← Fleet / host-a3-prod-002` + risk score badge in top-right.
- Metadata bar: env (prod/staging/dev), models active on host, agent version + connection status dot, last-seen timestamp.
- Tabs: **Alerts** (default) · Events · Compliance.
  - Alerts tab: same queue UI as `/alerts` but filtered to this host.
  - Events tab: timeline scoped to host.
  - Compliance tab: policies + pass/fail + most recent violation.

### 5.4 Settings (`/settings`)

Minimal. Stub for v1:
- `sigil-server` connection URL + status indicator (green/yellow/red dot) + host count.
- Auth: basic username/password info (no UI for user management in v1 — single admin user via env var).
- Triage data retention: 90 days (display only, configurable via env in v1).

## 6. Visual system

### 6.1 Theme

**Dark only in v1.** Light theme deferred — adds ~2x design/QA cost and SOC analysts overwhelmingly run dark.

### 6.2 Color tokens

**Backgrounds:**

| Token | Hex | Use |
|---|---|---|
| `bg-page` | `#0a0a0c` | Page background (subtle warm-black) |
| `bg-surface` | `#0d0d10` | Panels, cards, slide-over |
| `bg-elevated` | `#18181b` | Row hover, inputs |
| `border-subtle` | `#1f1f23` | Row dividers, faint borders |
| `border` | `#27272a` | Standard borders, chip outlines |

**Text:**

| Token | Hex | Use |
|---|---|---|
| `text-primary` | `#fafafa` | Titles, emphasis |
| `text-body` | `#e4e4e7` | Body text, alert titles |
| `text-muted` | `#a1a1aa` | Secondary, metadata |
| `text-subtle` | `#71717a` | Timestamps, labels, placeholder |

**Severity (binding semantic — never reuse these hues for other meaning):**

| Severity | Hex | Visual |
|---|---|---|
| CRITICAL | `#ef4444` | red-500 + glow (box-shadow 0 0 8px rgba(239,68,68,0.6)) |
| HIGH | `#f97316` | orange-500, flat |
| MEDIUM | `#eab308` | yellow-500, flat |
| LOW | `#3b82f6` | blue-500, flat |
| INFO | `#71717a` | gray-500, flat |

Only critical uses glow. Destructive UI (e.g., "Delete connection") uses an outline-only style with red text — not a filled red button — to avoid muddying the severity language.

**Status / accent:**

| Token | Hex | Use |
|---|---|---|
| `status-healthy` | `#22c55e` | Connected, healthy, resolved |
| `status-degraded` | `#f59e0b` | Degraded, stale data warning |
| `status-down` | `#ef4444` | Disconnected, failed (shares critical hue intentionally) |
| `accent` | `#a78bfa` | Brand mark, links, primary button bg |

### 6.3 Typography

- **Sans (UI):** Inter, fallback `-apple-system, system-ui, sans-serif`.
- **Mono (timestamps, rule IDs, code excerpts):** JetBrains Mono, fallback `SF Mono, Menlo, monospace`.

**Size scale:**

| Use | Size |
|---|---|
| Labels, severity text | 10px |
| Alert row body | 11px |
| Filter chips, metadata | 12px |
| Page titles, slide-over header | 14px |
| Host detail hero numbers (risk score) | 20px+ |

### 6.4 Density / spacing

| Element | Value |
|---|---|
| Row height (alert / host list) | 28px |
| Row padding | 5px vertical / 10-14px horizontal |
| Section padding | 14px |
| Gap between cells | 8px |
| Border radius (rows, chips) | 4px |
| Border radius (panels) | 6px |
| Border radius (severity dot) | 50% |

Approximately 14 alert rows fit per 400px of viewport height — meets the "Splunk ES density" bar from D3.

## 7. Interaction patterns

### 7.1 Keyboard shortcuts (v1)

| Key | Action |
|---|---|
| `j` / `k` | Move selection down / up in alert queue |
| `Enter` | Open slide-over for selected alert |
| `Esc` | Close slide-over / modal |
| `a` | Assign (in slide-over) |
| `r` | Resolve (in slide-over) |
| `c` | Acknowledge (in slide-over) |
| `/` | Focus search bar |
| `g a` / `g f` / `g s` | Go to Alerts / Fleet / Settings (Vim-style leader) |
| `?` | Open shortcut cheatsheet |

A small `?` button persists in bottom-right of viewport.

### 7.2 Real-time updates

- Alert queue polls `sigil-server` every **5 seconds** in v1 (WebSocket deferred to v1.5+).
- Header displays "Updated Ns ago". Color changes to indicate freshness:
  - Default gray when < 30s
  - Yellow when 30–60s
  - Red when > 60s
- A new critical alert at the top of the queue gets a 1-second highlight animation. Polling pauses while user is hovering rows to avoid sort-jitter.

### 7.3 Search and filter

- Filter chips toggle on click, debounced (300ms) to URL state.
- Search input live-fetches at 300ms throttle.
- Empty result with active filters: "No alerts match. Clear filters?" with a reset link.

### 7.4 URL routing

All state is URL-reflected so analysts can share links:
- `/alerts?sev=critical&status=open&q=injection&alert=A4821` opens the alerts page with filters applied and slide-over showing alert A4821.

### 7.5 Triage state machine

`open → acknowledged → investigating → resolved`. No `closed` state. All transitions reversible by any authenticated user in v1 (no role-based restrictions until v1.5+).

### 7.6 Triage data model (sigil-manager owned)

Stored locally (D5). Fields:
- Alert reference (alert ID from `sigil-server`)
- Status (enum above)
- Assignee (free-text username for v1, no directory lookup)
- Notes (plain text, no markdown in v1, with author + timestamp per note)
- State change log (who set what when)

## 8. States

### 8.1 Empty states

| Screen | Empty case | Treatment |
|---|---|---|
| Alerts | Filters return 0 | "No matching alerts in last 24h" + reset link |
| Alerts | No open alerts at all | "🎉 No open alerts. Last incident was 2 days ago." |
| Fleet | No hosts connected | "No hosts connected yet" + link to sigil-server setup guide |
| Host detail Events tab | No events in range | "No events in the last 24h" + time-range adjustment hint |

### 8.2 Loading

- Initial page load: header renders immediately; body shows **5 skeleton rows** (not a spinner — perceived speed is faster).
- Polling refresh: no visible indicator beyond "Updated Ns ago" timestamp.
- Slide-over open: header renders immediately; body shows skeleton blocks.

### 8.3 Error states

| Error | Treatment |
|---|---|
| `sigil-server` connection lost | Top banner: "Lost connection to sigil-server. Retrying every 10s..." with manual retry button. Show last cached data with stale indicator. |
| Auth failure (401) | Redirect to login page. |
| API 500 / unknown | Inline error in affected row + retry; rest of page stays usable. |

### 8.4 Stale data indicator

When `sigil-server` hasn't returned fresh data:
- < 30s: gray "Updated Ns ago" (normal)
- 30–60s: yellow
- 60s: red, plus banner

## 9. Auth

v1: **single admin user** via username/password (bcrypt hash + JWT session, 12-hour expiry, no refresh token). No user management UI. Credentials set via env var on container startup.

Deferred to v1.1: single OAuth provider (GitHub). Deferred to never (this repo): SAML, SCIM, multi-user RBAC — these are `sigil-cloud` concerns.

## 10. Brand mark

Placeholder: `◆ sigil` (diamond glyph + word mark in `text-primary`). Real branding to be defined separately; nothing in v1 depends on a specific logo asset.

## 11. Out of scope (v1)

Explicit YAGNI list — anything here is **not** in v1 and pushing it back into v1 needs a separate decision:

- Multi-tenancy, org/team hierarchy, tenant routing
- Billing, payments, usage metering, invoicing
- SAML, SCIM, enterprise SSO, RBAC beyond single-user
- Admin/CS panel, impersonation, audit log of admin actions
- Compliance evidence pipeline (SOC 2, GDPR DPA)
- Slack / PagerDuty / external webhook integrations
- Hunts (saved queries)
- Cases (incident grouping across alerts)
- Light theme
- Mobile-optimized UI (responsive but desktop-first)
- i18n / 다국어 (v1 is English-only; Korean evaluated for v1.5+)
- AI-assisted alert classification, auto-prioritization

The first six explicitly belong to `sigil-cloud` per `sigil-strategy.md` and must not be implemented in this repo.

## 12. Deferred to implementation plan

Decisions intentionally left for the `writing-plans` skill, not this brainstorm:

- Frontend stack (React + Next.js? Vite + React? SvelteKit?)
- Backend stack (Go? Rust? Node?)
- Container topology (single image vs. front+back split)
- Triage state DB (SQLite vs. embedded Postgres vs. external)
- Auth library (jose? lucia? home-rolled?)
- Build / test / lint toolchain
- Deployment example assets (`docker-compose.yml`, etc.)

## 13. Cross-references

- `sigil-strategy.md` — productization strategy; the "read-only against sigil-server" wording (both in the tier table and in the "What sigil-manager is" section) was updated during this brainstorm to reflect D5.
- `CLAUDE.md` — repo guardrails; scope boundaries here align with the "MUST NOT contain" list there.
- `sigil-server` Phase 3b.4 — fleet aggregation API on the `sigil` repo defines the upstream data shape this console consumes. UI prototyping can proceed against mock data while 3b.4 is in flight, but the data contract from 3b.4 must be stable before this console is wired to a real server.
