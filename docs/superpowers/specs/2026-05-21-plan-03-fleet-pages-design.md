# Plan 03 Design — Fleet pages (`/fleet/risk`, `/fleet/events`, `/fleet/compliance`)

- **Status:** Design approved 2026-05-21. Ready for implementation-plan authoring.
- **Date:** 2026-05-21
- **Owner:** Justin Kwon
- **Builds on:** Plan 02 (Foundation + Alerts queue), merged to `main`
  (PR #4 `9b914fe`, follow-up PR #6 `f5269ce`).
- **Cross-links:**
  - Fleet API contract: `docs/superpowers/specs/2026-05-16-fleet-api-contract.md`
    (§5.5 risk, §5.6 compliance, §5.7 events; §13.1 `open_alert_count_24h`
    divergence)
  - UI/UX design: `docs/superpowers/specs/2026-05-16-ui-ux-design.md`
    (§5.2 Fleet, §6 visual system, §7.2 polling, §8 states)
  - Producer divergence issue: `Ju571nK/sigil#21`

---

## 1. Goal

Ship the **Fleet** section (UI/UX D6): three tabs that let a SOC analyst
browse the fleet proactively rather than reactively through the alert
queue. All three read from `sigil-server` via the existing `FleetClient`;
none of them write state.

**End state:** an analyst clicks `Fleet` in the top nav, lands on
`/fleet/risk` (hosts sorted by AI Guard risk), can switch to `/fleet/events`
(fleet-wide event timeline) and `/fleet/compliance` (per-host policy state
with a derived status pill), and each tab preserves its filters in the URL.
CI green.

## 2. Scope

**IN (this plan):**
- Two new `/api/v1` passthrough routes: `GET /fleet/risk`,
  `GET /fleet/compliance`. (`/fleet/events` already exists from Plan 02.)
- Fleet tab shell + three routes under a nested `_authed/fleet/` layout
  (approach A — nested file-based routes).
- Risk tab: host list sorted by `score desc`, risk bar, hostname (plain
  text), score (0.0–10.0), `top_tool`, and the trailing-24h warn count
  shown honestly with a caveat tooltip.
- Events tab: fleet-wide reverse-chronological event timeline with
  event-type / host / time-range filters. A **new** `EventsTable`
  component, distinct from the Alerts `QueueTable`.
- Compliance tab: per-host raw signals + a **client-derived** status pill
  (✅ In sync / ⚠️ Drift / ❌ Expired / ❌ Failing signature).
- Slow polling (20s) for all three tabs (UI/UX §7.2 sets 5s for alerts;
  fleet data changes slowly, so a longer interval reduces server load).
- States: empty / loading / error / stale per tab, reusing Plan 02 §8
  patterns.
- Tests: Go handler tests (risk + compliance), a Vitest unit test for the
  compliance derivation, and Playwright e2e covering navigation + tab
  switching + per-tab rendering.

**OUT (deferred to later plans):**
- `/hosts/:hostname` host detail, hostname→host_id resolution, and making
  hostnames clickable (**Plan 04**). In Plan 03 hostnames render as plain
  text.
- Real Settings page (**Plan 05**).
- Light theme, responsive tweaks, accessibility audit (**Plan 06**).
- Client-side recomputation of a precise alert count (we display the
  server's coarse warn count with a caveat instead — see §7).

## 3. Architecture & routing (approach A)

Nested file-based routes under the existing `_authed` pathless layout, so
the auth guard from Plan 02 applies unchanged:

```
web/src/routes/_authed/fleet/
  route.tsx        → tab-bar layout; renders <Outlet/> below the tabs
  index.tsx        → redirect to /fleet/risk
  risk.tsx         → Risk tab (default)
  events.tsx       → fleet-wide Events timeline
  compliance.tsx   → Compliance tab
```

- `/fleet` (index) issues a `redirect({ to: '/fleet/risk' })` in
  `beforeLoad` so the bare path always resolves to the default tab.
- `route.tsx` renders the tab bar (see §6 `FleetTabs`) followed by the
  child route via `<Outlet/>`. The tab bar uses shadcn `Tabs` for the
  visual treatment only; the triggers are `<Link>`s so navigation is
  route-based and each tab is independently deep-linkable / bookmarkable
  (matches the UI/UX §4 information architecture, which lists
  `/fleet/risk`, `/fleet/events`, `/fleet/compliance` as distinct paths).
- Each tab route declares its own `validateSearch` with all-optional
  fields (same idiom as Plan 02's `alerts.tsx`), so `<Link to="/fleet/risk">`
  needs no search and filters round-trip through the URL.
- `TopNav`'s `Fleet` link (a stub in Plan 02) becomes active and shows the
  active style whenever the path starts with `/fleet`.

## 4. Backend — thin passthroughs

The `FleetClient` interface already implements every endpoint these pages
need (`FleetRisk`, `FleetCompliance`, `Events`), with both HTTP and Mock
implementations and unit tests, delivered in Plan 02. Only the `/api/v1`
HTTP surface is missing two routes.

- `GET /api/v1/fleet/risk` → `handleFleetRisk`:
  reads `tool`, `min_bucket`, `cursor`, `limit` query params → calls
  `Fleet.FleetRisk(ctx, RiskParams{...})` → returns the `RiskPage` body.
- `GET /api/v1/fleet/compliance` → `handleFleetCompliance`:
  reads `cursor`, `limit` → calls `Fleet.FleetCompliance(...)` → returns
  the `CompliancePage` body.
- `GET /api/v1/fleet/events` — **already wired** in Plan 02 (the Alerts
  queue uses it). The fleet Events tab calls the same route without the
  `min_ai_guard_bucket` / alert-kind filters so it returns the full
  timeline.

Both new handlers:
- sit behind the existing `RequireAuth` middleware,
- translate fleet errors through the existing `mapFleetErr` (so upstream
  sigil-server URLs never leak, and 503 maps to a consumer-side
  `service_unavailable`),
- perform **no compliance status derivation** — per contract F13 the
  server exposes raw signals only and the UI derives the pill (§5).

`internal/api/v1/handlers_test.go` gains cases for both: happy path
(fixture passthrough shape), unauthorized (401), and upstream-down
(maps to the consumer error code).

## 5. Frontend data layer

`web/src/api/fleet.ts` adds:
- Wire types `RiskRow`, `RiskPage`, `ComplianceRow`, `CompliancePage`
  mirroring contract §5.5 / §5.6 verbatim.
- `fetchFleetRisk(params)`, `fetchFleetCompliance(params)` using the
  existing typed fetch wrapper (`credentials: 'include'`, the same
  error-subclass mapping as Plan 02).
- The fleet Events tab reuses the existing events fetch with a
  fleet-wide (unfiltered) parameter set.

Hooks (`web/src/hooks/`):
- `useFleetRisk(filter)`, `useFleetCompliance()`, `useFleetEvents(filter)`
  — TanStack Query with a **20s `refetchInterval`**. A small shared
  `useFleetQuery` base centralizes the polling + stale + retry config so
  the three hooks stay consistent (and so a future interval change is one
  edit).

## 6. Compliance status derivation

The one piece of genuine logic. A pure module
(`web/src/lib/compliance.ts`) exports:

```ts
type ComplianceStatus = 'in_sync' | 'drift' | 'expired' | 'failing_signature';
function deriveComplianceStatus(row: ComplianceRow): ComplianceStatus
```

Single worst-state pill, evaluated in priority order (contract §5.6):

| Condition | Status | Pill |
|---|---|---|
| `policy_expired_active` | `expired` | ❌ Expired |
| `signature_failures_24h > 0` | `failing_signature` | ❌ Failing signature |
| `version_drift > 0` | `drift` | ⚠️ Drift |
| otherwise | `in_sync` | ✅ In sync |

Priority when several apply: Expired > Failing signature > Drift >
In sync. Keeping the rule consumer-side (not server-side) matches F13 —
the producer never exposes a numeric compliance score.

## 7. The `open_alert_count_24h` caveat

Contract §5.5 / §13.1: the server's `open_alert_count_24h` is implemented
as the trailing-24h sum of **all** `severity=warn` events, not the
alert-definition-filtered count the field name implies (producer issue
`Ju571nK/sigil#21`). Per the approved decision, the Risk tab renders this
value honestly:

- Column header: **"Warn 24h"** (not "Open alerts").
- Tooltip: *"Trailing-24h warn events; not alert-definition filtered
  (issue #21)."*

We do **not** recompute a precise count client-side in this plan (it would
cost one `/v1/events` request per host). If a tighter number is needed
later, that is a separate enhancement tracked against issue #21.

## 8. Components

All under `web/src/components/Fleet/` unless noted:

- `FleetTabs` — the tab bar: three `<Link>`s styled via shadcn `Tabs`,
  active state driven by the current route.
- `RiskTable` / `RiskRow` — risk bar colored to the bucket (reuse the
  Plan 02 severity color tokens), hostname as **plain text**, score
  (0.0–10.0, mono), `top_tool` (humanized via the shared tool labeler),
  `reasons_count`, and the "Warn 24h" column with the caveat tooltip.
  Filter row: `tool`, `min_bucket`.
- `ComplianceTable` / `ComplianceRow` — status pill (from
  `deriveComplianceStatus`), version drift shown as `applied → current`,
  `signature_failures_24h`, last reload (relative time).
- `EventsTable` / `EventRow` — **new, separate from `QueueTable`**:
  fleet-wide timeline. Columns: severity dot · age · kind · host · tool
  (when AI-guard) · short summary. Filter row: event-type, host,
  time-range. No triage-centric columns (no ack pill / assignee), which
  is why it is a distinct component rather than a `QueueTable` reuse.
- States per tab: empty / loading skeleton rows / error banner / stale
  indicator, reusing the Plan 02 §8 building blocks.

A shared tool-label helper: Plan 02 has two copies of `humanTool()`
(in `SlideOver.tsx` and `QueueRow.tsx`). Plan 03 introduces a third
consumer (Risk/Events rows). To avoid a fourth copy, extract a single
`humanTool()` into a shared module (e.g. `web/src/lib/labels.ts`) and have
all callers import it. This is a targeted cleanup justified by the new
consumer, not unrelated refactoring.

## 9. States (UI/UX §8)

- **Empty:** Risk — "No hosts above the selected risk level" + reset link.
  Events — "No events in the selected range" + range hint. Compliance —
  "No hosts reporting policy state yet."
- **Loading:** skeleton rows at the tab's row density on first load;
  subsequent 20s polls refresh in place without a skeleton flash.
- **Error:** the shared error banner from Plan 02 (`/fleet/healthz`-driven
  connection banner already mounts in the `_authed` layout; per-tab fetch
  errors render inline).
- **Stale:** the freshness indicator pattern from Plan 02, tuned to the
  20s interval.

## 10. Testing

- **Go:** `handlers_test.go` cases for `/fleet/risk` and
  `/fleet/compliance` — happy path, 401, upstream-down error mapping.
- **Vitest:** `deriveComplianceStatus` truth table — each of the four
  states plus the priority ordering when multiple conditions hold.
- **Playwright e2e** (against the production binary, `MOCK_FLEET=1`):
  1. `Fleet` nav → `/fleet` redirects to `/fleet/risk`.
  2. Tab switching changes both URL and content.
  3. Risk tab renders mock rows sorted by score; the "Warn 24h" tooltip
     is present.
  4. Compliance pills derive correctly across the mock's mixed hosts.
  5. Events tab renders the fleet-wide timeline; a filter narrows it.
- **Mock fixtures:** the Plan 02 fixture already seeds 5 hosts with mixed
  risk and policy versions. Plan 03 tunes the compliance signals so the
  fixture exercises each of the four derived states (in-sync, drift,
  expired, failing-signature) at least once.

## 11. File-structure delta

```
internal/api/v1/
  fleet.go              MODIFY  + handleFleetRisk, handleFleetCompliance
  server.go             MODIFY  + 2 routes
  handlers_test.go      MODIFY  + risk/compliance cases

internal/fleet/
  mock.go               MODIFY  tune compliance signals for 4 states

web/src/api/fleet.ts    MODIFY  + RiskRow/ComplianceRow types + fetchers
web/src/hooks/
  useFleetQuery.ts      ADD     shared polling base
  useFleetRisk.ts       ADD
  useFleetCompliance.ts ADD
  useFleetEvents.ts     ADD
web/src/lib/
  compliance.ts         ADD     deriveComplianceStatus (+ Vitest)
  labels.ts             ADD     shared humanTool() (dedup 3 callers)
web/src/components/Fleet/
  FleetTabs.tsx         ADD
  RiskTable.tsx         ADD
  RiskRow.tsx           ADD
  ComplianceTable.tsx   ADD
  ComplianceRow.tsx     ADD
  EventsTable.tsx       ADD
  EventRow.tsx          ADD
web/src/routes/_authed/fleet/
  route.tsx             ADD
  index.tsx             ADD
  risk.tsx              ADD
  events.tsx            ADD
  compliance.tsx        ADD
web/src/components/Layout/TopNav.tsx   MODIFY  activate Fleet link
web/tests/e2e/fleet.spec.ts            ADD
```

## 12. Stable-interface guarantee

Plan 03 adds only new routes, components, and hooks. It does not modify the
auth, `FleetClient`, or triage repo APIs — those remain the stable
interfaces established in Plan 02 for Plans 04–06 to build on.
