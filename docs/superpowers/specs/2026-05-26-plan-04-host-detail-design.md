# Plan 04 — Host detail page (`/hosts/$hostId`)

**Status:** Design approved 2026-05-26. Ready for implementation plan.
**Consumes:** `GET /v1/fleet/hosts/{host_id}` (fleet API contract §5.4), plus
the already-wired `/v1/fleet/compliance` (§5.6) and `/v1/events` (§5.7).
**Builds on:** Plan 03 (Fleet pages). Reuses `EventsTable`,
`deriveComplianceStatus`/`COMPLIANCE_META`, `humanTool`, `relativeAge`,
`<SkeletonRows>`.

## 1. Goal

Drill-down from the fleet tables into a single host. Clicking a hostname in
the Risk / Events / Compliance tables (plain text since Plan 03) navigates to
`/hosts/$hostId`, a read-only page that shows everything `sigil-server` knows
about that host: its per-tool AI Guard risk (with reasons), host metadata,
policy state, agent health, derived compliance status, and a timeline of the
host's own events.

This is the highest-value page for small fleets. Real-deployment check
(2026-05-26, live `sigil-server` at 192.168.50.213:9090) showed the fleet
holding a **single host** — so the host-detail view, not the fleet tables, is
where an operator actually spends time.

## 2. Real-data grounding

Verified against the live server (host `ju571n`, `825e46a2-…`, Mac OS 26.5.0
/ aarch64, status `disconnected`). Findings that shape the design:

- **Nulls are normal, not errors.** A real host had `agent_health` latency /
  heartbeat fields all `null`, an interface `mac: null`, `policy_version: 0`,
  and `last_policy_reload_ts: null`. Per the contract, `null` means "not
  emitted yet," NOT "data unavailable." **Every block must render nulls
  gracefully** (e.g. "not reported yet" / "—"), never blank or "Invalid Date".
- **AI Guard `by_tool` is the rich, headline signal.** `claude_code` was
  `HIGH 5.75` with six heterogeneous reasons (`no_sandbox` + `executor`,
  `broad_matcher` + `hook_event`/`matcher` ×4, `permissions_deny_empty`) and
  `scope: {kind: "user_global"}`. The other tools (`cursor` MEDIUM,
  `claude_desktop`/`codex`/`continue_dev`/`gemini` all LOW 0.0) carried no
  signal. So most tools are quiet — the UI must amplify the risky ones and
  fold away the zero ones.
- Reason shapes match contract §14.5 exactly, i.e. identical to the AI Guard
  reasons the Plan 02 `SlideOver` already renders → reuse that rendering.

## 3. Decisions (with rationale)

| # | Decision | Rationale |
|---|----------|-----------|
| D1 | **Route by `host_id`** (`/hosts/$hostId`), display `hostname` | Host-detail API is keyed by `host_id`; no resolver needed; works for `hostname: null` hosts; `host_id` is immutable and unique. Ugly UUID URL is cheap for an internal console. |
| D2 | **Single scrollable page, AI-Guard-first** | One host = one snapshot; tabs add nav cost for little gain. Leading with AI Guard is on-brand for an AI-SPM console. |
| D3 | **AI Guard block = risky-tools-first cards + collapsed quiet tools** | Real data is mostly zeros; surface `bucket > low` tools with reasons inline, fold LOW/0 tools into a one-line strip. |
| D4 | **Compliance = derived status pill + `signature_failures_24h`** | Reuse `deriveComplianceStatus`/`COMPLIANCE_META`; `signature_failures_24h` lives on `/fleet/compliance` (F13), not on `HostDetail`, so the page fetches compliance and finds this host's row. Keeps the host page consistent with the Compliance tab. |
| D5 | **Include a per-host events section** | Reuse `EventsTable` with a `host_id` filter; completes the host story. Recent slice + a link to the full Fleet Events view filtered to this host. |
| D6 | **20s polling**, same cadence as the fleet pages | Consistency; host detail is live-ish. |

## 4. Architecture

### 4.1 Backend (Go) — one passthrough handler
Add `GET /api/v1/fleet/hosts/{host_id}` → `s.handleFleetHostByID`, a pure
passthrough over the existing `fleet.Client.FleetHostByID`, mirroring Plan 03's
`handleFleetRisk`/`handleFleetCompliance`:
- Read `host_id` from the chi URL param.
- Call `FleetHostByID(ctx, hostID)`; on `fleet.ErrNotFound` return the mapped
  404 via `mapFleetErr`; on success `httputil.WriteJSON`.
- Register inside the authenticated `r.Group` in `server.go`; update the route
  doc comment.
- No `/fleet/hosts` list handler — there is no host-list page (hosts are
  reached by clicking into the existing fleet tables).

### 4.2 Frontend data fetching (TanStack Query, 20s)
- New `web/src/api/fleet.ts` types mirroring the Go `HostDetail` tree:
  `HostDetail`, `HostMeta`, `NetInterface`, `PolicyState`, `AgentHealth`,
  `AiGuardByTool` (`{ by_tool: Record<string, ToolAiGuard> }`), `ToolAiGuard`
  (`score`, `bucket`, `assessed_ts`, `is_reattestation`, `scope`, `reasons`).
  Reasons/scope are kept as loosely-typed JSON (same approach as the existing
  AI Guard evidence types) and parsed by the renderer.
- New fetcher `fleetHost(hostId): Promise<HostDetail>` → `GET /fleet/hosts/{id}`.
- New hook `useFleetHost(hostId)` on `useFleetQuery` (20s), key
  `['fleet','host',hostId]`, returning `{ host, isPending, error, ... }`. A 404
  surfaces as a typed error the page renders as "host not found."
- Compliance pill: reuse `useFleetCompliance()`, then
  `rows.find(r => r.host_id === hostId)` → `deriveComplianceStatus(row)` +
  `row.signature_failures_24h`. (Acceptable: the compliance endpoint returns
  the whole fleet; we filter client-side. Fleets are small.)
- Events: reuse `useFleetEvents({ evidenceKinds: [], hostIDs: [hostId], since: null })`
  — the hook's default page (limit 100), host-filtered, rendered as-is; no
  separate pagination on this page. "See all" deep-links to the full Fleet
  Events view (§7) for anything beyond that slice.

### 4.3 Components
- New route `web/src/routes/_authed/hosts/$hostId.tsx` — owns the fetch, the
  loading/404 states, and lays out the sections.
- New `web/src/components/Host/HostHeader.tsx` — hostname, status pill,
  compliance pill, last-seen (`relativeAge`), agent version, `host_id` (mono),
  back-to-fleet link.
- New `web/src/components/Host/AiGuardByTool.tsx` — risky-first cards +
  collapsed-quiet strip; renders each tool's `bucket`/`score`/`scope` and its
  reasons via the shared `ReasonList` (below).
- New `web/src/components/Host/HostMetaCard.tsx` — OS/arch/kernel, gateway,
  DNS, interfaces (name/mac/ipv4/ipv6); null-tolerant.
- New `web/src/components/Host/PolicyHealthCard.tsx` — policy version / expired
  / last reload, plus agent-health counters and latest heartbeat/latency
  (mostly null on disconnected hosts → "not reported yet").
- Reused: `EventsTable` (per-host section), `<SkeletonRows>`, `humanTool`,
  `relativeAge`, `deriveComplianceStatus`/`COMPLIANCE_META`.

### 4.4 Small refactor — shared `ReasonList`
The Plan 02 `SlideOver` renders AI Guard reasons (the §14.5 variant shapes:
`no_sandbox`, `broad_matcher`, `permissions_deny_empty`, `source_chain`,
`mcp_server_local_command`, …). `AiGuardByTool` needs the identical rendering.
Extract a presentational `web/src/components/ReasonList.tsx` from `SlideOver`
(shared home, since both `AlertsQueue/SlideOver` and `Host/AiGuardByTool`
consume it), and have both use it — same
rule-of-three reasoning as the Plan 03 `humanKind`/`relativeAge` extraction.
`SlideOver`'s behavior must stay byte-identical (verify in review).

## 5. Page layout (single scroll, AI-Guard-first)

```
◂ Fleet   ju571n   ● disconnected   ✔ In sync   last seen 1d ago   agent 0.1.0
                                                 host_id 825e46a2-…  (mono)
────────────────────────────────────────────────────────────────────────────
AI GUARD RISK                                            max: HIGH 5.75
┌ claude_code  HIGH 5.75   user_global · assessed 2h ─────────────────────┐
│  no sandbox · host_shell   broad matcher · Notification *   …   deny empty │
└────────────────────────────────────────────────────────────────────────┘
┌ cursor  MEDIUM 2.5  … reasons ──────────────────────────────────────────┐
└────────────────────────────────────────────────────────────────────────┘
LOW (0.0): claude_desktop · codex · continue_dev · gemini        [ expand ]
────────────────────────────────────────────────────────────────────────────
┌ Host metadata ───────────────┐   ┌ Policy ───────────────────────────────┐
│ Mac OS 26.5.0 · aarch64       │   │ version 0 · not expired · reload —    │
│ kernel 25.5.0                 │   ├ Agent health ─────────────────────────┤
│ gw 192.168.50.1 · dns …       │   │ heartbeat — · stalls 0 · lag 0 · …    │
│ interfaces: bridge100 …       │   │ hash p99 — · jsonl floor —            │
└───────────────────────────────┘   └───────────────────────────────────────┘
────────────────────────────────────────────────────────────────────────────
Recent events (this host)                       see all in Fleet Events ▸
[ sev · age · kind · tool · … rows from EventsTable, host_id-filtered ]
```

## 6. States
- **Loading:** block-level skeletons (reuse `<SkeletonRows>` where a list fits;
  simple pulse bars for the header/cards).
- **404 (host not in index):** dedicated "Host not found" panel with a
  back-to-fleet link. The contract notes this happens for hosts evicted from
  the in-memory index.
- **Disconnected / null-heavy host:** each block degrades to "not reported yet"
  / "—"; never blank, never "Invalid Date" (use `relativeAge`'s guarded
  formatting).
- **Upstream 503 (server boot/rebuild):** same `ServiceUnavailableError`
  handling the fleet hooks already use → a transient "reconnecting" affordance.

## 7. Hostname link wiring (the core deliverable)
Activate the plain-text hostnames Plan 03 deliberately left unlinked:
- `RiskTable.tsx`, `ComplianceTable.tsx`, `EventsTable.tsx` host cells →
  `<Link to="/hosts/$hostId" params={{ hostId: row.host_id }}>`.
- The displayed text stays `hostname ?? host_id.split('-')[0]`; only the
  wrapping changes. Null-hostname hosts still link (by `host_id`).
- The Fleet Events "see all" deep-link uses the events route's existing
  `host` search param (`/fleet/events?host=<id>`), which Plan 03 parsed but
  never populated.

## 8. Out of scope
- No triage / writes on the host page (read-only, like the rest of the
  console).
- No host-list page, no `/fleet/hosts` list passthrough.
- No per-tool history/sparklines (HostDetail is a current snapshot only).
- Light theme / responsive / broader a11y stay in Plan 06.

## 9. Testing
- **Go:** handler test for `/api/v1/fleet/hosts/{host_id}` — 200 passthrough +
  404 mapping (mirror the Plan 03 handler tests; the mock already implements
  `FleetHostByID`). Confirm the mock's `dave-vm` (null hostname, expired) and a
  rich host (alice, with `by_tool`) both round-trip.
- **Web unit:** any new pure helper (e.g. risky/quiet tool partition); the
  extracted `ReasonList` against the §14 variant fixtures.
- **E2E (`fleet.spec.ts` sibling or extension):** click a hostname in the Risk
  table → land on `/hosts/$hostId`; header + AI Guard + meta/policy/health
  render; the per-host events section paints; a bogus `/hosts/<unknown>`
  shows the 404 panel.

## 10. Risks / notes
- `ReasonList` extraction must not regress `SlideOver` — treat as a
  behavior-preserving refactor with its own review check.
- Client-side compliance filtering assumes small fleets (true today; the
  endpoint has no single-host filter). If fleets grow large this becomes a
  full-fleet fetch per host view — acceptable for now, note for a future
  server-side `?host_id=` ask if it bites.
- `host_id` is a path param; ensure the route validates/escapes it and a
  malformed id yields the 404 panel rather than a crash.
