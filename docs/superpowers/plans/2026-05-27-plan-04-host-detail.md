# Plan 04 — Host detail page (`/hosts/$hostId`) Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add a read-only per-host detail page at `/hosts/$hostId` that renders one host's AI Guard risk (per tool, with reasons), host metadata, policy state, agent health, derived compliance status, and its recent events — and make the fleet tables' hostnames link to it.

**Architecture:** One Go passthrough handler over the existing `FleetHostByID` client method; a single TanStack Router route that fetches `GET /api/v1/fleet/hosts/{host_id}` (20s poll) plus the already-wired compliance/events; a single-scroll, AI-Guard-first page composed of small focused components. Reuses Plan 03's `EventsTable`, `deriveComplianceStatus`/`COMPLIANCE_META`, `humanTool`, `relativeAge`, `<SkeletonRows>`, and extracts the AI-Guard reason renderer out of `SlideOver` into a shared `ReasonList`.

**Tech Stack:** Go 1.22 + chi v5 (passthrough); React 19 + TanStack Router (file-based, `$hostId` param) + TanStack Query v5 (20s) + Tailwind v4 + Biome + Vitest + Playwright.

**Spec:** `docs/superpowers/specs/2026-05-26-plan-04-host-detail-design.md`

---

## File structure

**Backend**
- Modify `internal/api/v1/fleet.go` — add `handleFleetHostByID` (passthrough).
- Modify `internal/api/v1/server.go` — register `GET /fleet/hosts/{host_id}`.
- Modify `internal/api/v1/handlers_test.go` — 2 handler tests.

**Frontend — data**
- Modify `web/src/api/fleet.ts` — `HostDetail` type tree + `fleetHost` fetcher.
- Modify `web/src/hooks/useFleetQuery.ts` — accept optional query options (retry).
- Create `web/src/hooks/useFleetHost.ts`.

**Frontend — shared refactor**
- Modify `web/src/lib/labels.ts` (+ `labels.test.ts`) — move `shortPath` here.
- Create `web/src/components/ReasonList.tsx` — extracted reason renderer.
- Modify `web/src/components/AlertsQueue/SlideOver.tsx` — consume `ReasonList`.

**Frontend — host page**
- Create `web/src/routes/_authed/hosts/$hostId.tsx` — route + fetch + states + layout.
- Create `web/src/components/Host/HostHeader.tsx`.
- Create `web/src/components/Host/AiGuardByTool.tsx`.
- Create `web/src/components/Host/HostMetaCard.tsx`.
- Create `web/src/components/Host/PolicyHealthCard.tsx`.

**Frontend — link wiring**
- Modify `web/src/components/Fleet/RiskTable.tsx`, `ComplianceTable.tsx`, `EventsTable.tsx` — hostname cell → `<Link>`.

**Tests / ship**
- Modify `web/tests/e2e/fleet.spec.ts` — host-detail navigation + render + 404.
- Modify `README.md`.

---

## Task 0: Branch

Already on `feat/plan-04-host-detail` (created at spec time, with the design doc committed). Confirm:

- [ ] **Step 1:** `git branch --show-current` → `feat/plan-04-host-detail`. `git log --oneline -1` shows the `docs(spec): Plan 04 Host detail` commit.

---

## Task 1: Backend — `GET /api/v1/fleet/hosts/{host_id}` passthrough

**Files:**
- Modify `internal/api/v1/fleet.go`
- Modify `internal/api/v1/server.go`
- Test `internal/api/v1/handlers_test.go`

- [ ] **Step 1: Write the failing tests** — append to `internal/api/v1/handlers_test.go`:

```go
func TestFleet_HostByID_Passthrough(t *testing.T) {
	h := newHarness(t)
	c := h.loginCookie()
	// alice-mbp — the mock seeds host_meta + current_risk + ai_guard for it.
	code, body, _ := h.do(http.MethodGet, "/fleet/hosts/5a7c3e91-aaaa-bbbb-cccc-111111111111", nil, c)
	require.Equal(t, http.StatusOK, code)

	var hd struct {
		HostID   string `json:"host_id"`
		Hostname string `json:"hostname"`
		HostMeta json.RawMessage `json:"host_meta"`
		AiGuard  *struct {
			ByTool map[string]json.RawMessage `json:"by_tool"`
		} `json:"ai_guard"`
	}
	require.NoError(t, json.Unmarshal(body, &hd))
	assert.Equal(t, "5a7c3e91-aaaa-bbbb-cccc-111111111111", hd.HostID)
	assert.Equal(t, "alice-mbp", hd.Hostname)
	assert.NotNil(t, hd.HostMeta, "alice has a HostMetaSnapshot")
	require.NotNil(t, hd.AiGuard)
	assert.Contains(t, hd.AiGuard.ByTool, "claude_code")
}

func TestFleet_HostByID_NotFound(t *testing.T) {
	h := newHarness(t)
	c := h.loginCookie()
	code, body, _ := h.do(http.MethodGet, "/fleet/hosts/00000000-0000-0000-0000-000000000000", nil, c)
	require.Equal(t, http.StatusNotFound, code)
	assert.Contains(t, string(body), "not_found")
}
```

- [ ] **Step 2: Run to verify they fail**

Run: `go test ./internal/api/v1/ -run TestFleet_HostByID -v`
Expected: FAIL — route not registered (404 for both, so the passthrough test fails on the body assertions / the not-found test may pass coincidentally; the passthrough test must fail).

- [ ] **Step 3: Add the handler** — append to `internal/api/v1/fleet.go` (after `handleFleetCompliance`):

```go
// handleFleetHostByID is a pass-through to FleetClient.FleetHostByID (§5.4).
// A 404 (host_id not in the server's in-memory index) maps via mapFleetErr.
func (s *Server) handleFleetHostByID(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "host_id")
	if id == "" {
		writeError(w, http.StatusBadRequest, "invalid_query", "host_id required")
		return
	}
	host, err := s.Fleet.FleetHostByID(r.Context(), id)
	if err != nil {
		mapFleetErr(w, err)
		return
	}
	httputil.WriteJSON(w, http.StatusOK, host)
}
```

- [ ] **Step 4: Register the route** — in `internal/api/v1/server.go`, inside the authenticated `r.Group`, after the `/fleet/compliance` line:

```go
		r.Get("/fleet/compliance", s.handleFleetCompliance)
		r.Get("/fleet/hosts/{host_id}", s.handleFleetHostByID)
```

And add to the route doc comment block (after the `/fleet/compliance` line):

```go
//	GET    /fleet/hosts/{host_id}          (cookie)
```

- [ ] **Step 5: Run to verify they pass**

Run: `go test ./internal/api/v1/ -run TestFleet_HostByID -v`
Expected: PASS (both).

- [ ] **Step 6: Full Go gate**

Run: `go test ./... && go vet ./...`
Expected: all pass.

- [ ] **Step 7: Commit**

```bash
git add internal/api/v1/fleet.go internal/api/v1/server.go internal/api/v1/handlers_test.go
git commit -m "$(cat <<'EOF'
feat(api): /api/v1/fleet/hosts/{host_id} passthrough

Plan 04 T1. handleFleetHostByID relays fleet.FleetHostByID (contract §5.4);
404 maps through mapFleetErr when the host_id isn't in the server index.
Mirrors the handleFleetEventByID passthrough.

Co-Authored-By: Claude Opus 4.7 (1M context) <noreply@anthropic.com>
EOF
)"
```

---

## Task 2: Frontend API client — `HostDetail` types + `fleetHost` fetcher

**Files:**
- Modify `web/src/api/fleet.ts`

These mirror the Go types (`internal/fleet/client.go`). Reuse the existing
`Scope` and `ReasonLike` types already exported from this file.

- [ ] **Step 1: Add the host-detail types + fetcher** — append to `web/src/api/fleet.ts` (after the compliance section):

```ts
// -----------------------------------------------------------------------------
// Host detail (/fleet/hosts/{host_id} — contract §5.4)
// -----------------------------------------------------------------------------

export interface NetInterface {
  name: string;
  mac: string | null;
  ipv4: string[];
  ipv6: string[];
}

export interface HostMeta {
  os_name: string;
  os_version: string;
  kernel_version: string;
  architecture: string;
  interfaces: NetInterface[];
  default_gateway_v4: string | null;
  default_gateway_v6: string | null;
  dns_servers: string[];
}

export interface PolicyState {
  last_applied_policy_version: number;
  policy_expired_active: boolean;
  last_policy_reload_ts: string | null;
}

export interface AgentHealth {
  recent_channel_stalls_24h: number;
  recent_watcher_degraded_24h: number;
  recent_sender_lag_critical_24h: number;
  last_heartbeat_ts: string | null;
  hash_p99_ms_latest: number | null;
  jsonl_above_soft_floor_latest: boolean | null;
}

/** One per-tool AI Guard rollup (§5.4 ai_guard.by_tool). Same reason/scope
 * shapes as AiGuardEvidence, so it reuses ReasonLike/Scope. */
export interface ToolAiGuard {
  score: number;
  bucket: 'low' | 'medium' | 'high' | 'critical' | string;
  assessed_ts: string;
  is_reattestation: boolean;
  scope: Scope;
  reasons: ReasonLike[];
}

/** Per-tool current risk rollup embedded in HostSummary.current_risk. */
export interface ToolRisk {
  score: number;
  bucket: string;
  assessed_ts: string;
}

export interface CurrentRisk {
  max_score: number;
  max_bucket: string;
  by_tool: Record<string, ToolRisk>;
}

/** Body of GET /fleet/hosts/{host_id} (§5.4): HostSummary + 4 nullable blocks. */
export interface HostDetail {
  host_id: string;
  hostname: string | null;
  agent_version: string;
  last_seen_ts: string;
  status: 'healthy' | 'stale' | 'disconnected' | string;
  current_risk: CurrentRisk | null;
  open_event_counts_24h: Record<string, number>;
  host_meta: HostMeta | null;
  policy_state: PolicyState | null;
  agent_health: AgentHealth | null;
  ai_guard: { by_tool: Record<string, ToolAiGuard> } | null;
}

export function fleetHost(hostId: string): Promise<HostDetail> {
  return api<HostDetail>(`/fleet/hosts/${encodeURIComponent(hostId)}`);
}
```

- [ ] **Step 2: Type-check**

Run: `cd web && npx tsc -b --noEmit`
Expected: clean (exit 0). (`Scope`/`ReasonLike`/`api` already exist in this file.)

- [ ] **Step 3: Commit**

```bash
git add web/src/api/fleet.ts
git commit -m "$(cat <<'EOF'
feat(web): HostDetail API client types + fleetHost fetcher

Plan 04 T2. Mirrors the Go HostDetail tree (host_meta / policy_state /
agent_health / ai_guard.by_tool), reusing the existing Scope + ReasonLike
types. fleetHost(hostId) hits /fleet/hosts/{id}.

Co-Authored-By: Claude Opus 4.7 (1M context) <noreply@anthropic.com>
EOF
)"
```

---

## Task 3: `useFleetHost` hook (+ optional retry on `useFleetQuery`)

**Files:**
- Modify `web/src/hooks/useFleetQuery.ts`
- Create `web/src/hooks/useFleetHost.ts`

- [ ] **Step 1: Extend `useFleetQuery` to accept optional options** — replace the body of `web/src/hooks/useFleetQuery.ts`:

```ts
import { type QueryKey, useQuery } from '@tanstack/react-query';

/** Fleet tabs poll slower than the alert queue (UI/UX §7.2 sets 5s for alerts). */
export const FLEET_POLL_INTERVAL_MS = 20_000;

type FleetQueryOptions = {
  /** Override react-query's default retry (e.g. don't retry a 404). */
  retry?: boolean | number | ((failureCount: number, error: Error) => boolean);
};

/**
 * Thin wrapper over useQuery with the fleet-page polling defaults applied.
 * Keeps the fleet hooks consistent and makes the interval one edit. `options`
 * is spread last so callers can override (existing callers pass none).
 */
export function useFleetQuery<T>(
  key: QueryKey,
  queryFn: () => Promise<T>,
  options?: FleetQueryOptions,
) {
  return useQuery({
    queryKey: key,
    queryFn,
    refetchInterval: FLEET_POLL_INTERVAL_MS,
    refetchIntervalInBackground: false,
    ...options,
  });
}
```

- [ ] **Step 2: Create `web/src/hooks/useFleetHost.ts`**

```ts
import { NotFoundError } from '@/api/client';
import { fleetHost } from '@/api/fleet';
import { useFleetQuery } from './useFleetQuery';

/**
 * Single host detail (§5.4) at the fleet poll interval. A 404 (host evicted
 * from the server index) surfaces immediately — we don't retry it — so the
 * route can paint its "host not found" panel without a multi-second stall.
 */
export function useFleetHost(hostId: string) {
  const q = useFleetQuery(['fleet', 'host', hostId], () => fleetHost(hostId), {
    retry: (failureCount, error) => !(error instanceof NotFoundError) && failureCount < 2,
  });
  return {
    host: q.data,
    isPending: q.isPending && !q.data,
    error: q.error,
    isFetching: q.isFetching,
    lastUpdatedAt: q.dataUpdatedAt,
    refetch: q.refetch,
  };
}
```

- [ ] **Step 3: Type-check + existing tests** (the 3 existing hooks pass no `options`, so behavior is unchanged)

Run: `cd web && npx tsc -b --noEmit && npm run test`
Expected: tsc clean; all existing Vitest tests pass.

- [ ] **Step 4: Commit**

```bash
git add web/src/hooks/useFleetQuery.ts web/src/hooks/useFleetHost.ts
git commit -m "$(cat <<'EOF'
feat(web): useFleetHost hook (20s poll, no-retry on 404)

Plan 04 T3. useFleetQuery gains an optional options arg (spread last, existing
callers unaffected); useFleetHost uses it to skip retrying a 404 so the route
can show "host not found" promptly.

Co-Authored-By: Claude Opus 4.7 (1M context) <noreply@anthropic.com>
EOF
)"
```

---

## Task 4: Extract shared `ReasonList` (behavior-preserving refactor)

`SlideOver` renders AI Guard reasons; the host page needs identical rendering.
Extract the renderer. `shortPath` is used by BOTH the reason renderer AND
`SlideOver`'s scope formatter (line ~511), so it moves to `lib/labels.ts`
(shared display formatters, next to `humanKind`/`humanTool`).

**Files:**
- Modify `web/src/lib/labels.ts` + `web/src/lib/labels.test.ts`
- Create `web/src/components/ReasonList.tsx`
- Modify `web/src/components/AlertsQueue/SlideOver.tsx`

- [ ] **Step 1: Move `shortPath` into `web/src/lib/labels.ts`** — append (the body is the exact one currently in `SlideOver.tsx`):

```ts
/** Shortens a filesystem path for display — keeps the last two segments,
 *  prefixing an ellipsis when truncated, e.g. "/a/b/c/d" → "…/c/d". */
export function shortPath(p: string): string {
  const parts = p.split('/').filter(Boolean);
  if (parts.length <= 2) return p;
  return `…/${parts.slice(-2).join('/')}`;
}
```

> Note: copy the body verbatim from `SlideOver.tsx:516` rather than the
> illustrative version above if they differ — the existing behavior is the
> source of truth.

- [ ] **Step 2: Add a `shortPath` test** — append to `web/src/lib/labels.test.ts`:

```ts
import { shortPath } from './labels';

describe('shortPath', () => {
  it('returns short paths unchanged', () => {
    expect(shortPath('/etc')).toBe('/etc');
    expect(shortPath('a/b')).toBe('a/b');
  });
  it('keeps the last two segments with an ellipsis when longer', () => {
    expect(shortPath('/home/demo/project/.claude/settings.json')).toBe('…/.claude/settings.json');
  });
});
```

> If `labels.test.ts` already imports from `./labels`, extend that import line
> instead of adding a duplicate import.

- [ ] **Step 3: Create `web/src/components/ReasonList.tsx`** — move `ReasonItem`, `reasonKey`, `asString`, `asStringArray` verbatim out of `SlideOver.tsx` into this file (bodies are the source of truth), wrapped by an exported `ReasonList`:

```tsx
import type { ReasonLike } from '@/api/fleet';
import { humanKind, shortPath } from '@/lib/labels';

/**
 * Renders AI Guard reasons (contract §14.5 variant shapes: no_sandbox,
 * broad_matcher, permissions_deny_empty, mcp_server_local_command, the
 * source_chain breadcrumb, …). Shared by the alerts SlideOver and the
 * host-detail AI Guard block.
 */
export function ReasonList({ reasons }: { reasons: ReasonLike[] }) {
  return (
    <ul className="space-y-0.5">
      {reasons.map((r) => (
        <ReasonItem key={reasonKey(r)} reason={r} />
      ))}
    </ul>
  );
}

// --- moved verbatim from SlideOver.tsx (ReasonItem, reasonKey, asString,
//     asStringArray). shortPath now lives in lib/labels. ---

function ReasonItem({ reason: r }: { reason: ReasonLike }) {
  // (exact body from SlideOver.tsx:382–425)
}

function reasonKey(r: ReasonLike): string {
  // (exact body from SlideOver.tsx:427–430)
}

function asString(v: unknown): string | undefined {
  // (exact body from SlideOver.tsx:432–435)
}

function asStringArray(v: unknown): string[] {
  // (exact body from SlideOver.tsx:436–…)
}
```

> The implementer must paste the real bodies from the current `SlideOver.tsx`
> (do not re-type from memory). `ReasonItem` references `humanKind` and
> `shortPath` — both now imported from `@/lib/labels`.

- [ ] **Step 4: Update `SlideOver.tsx` to consume `ReasonList`:**
  1. Add imports: `import { ReasonList } from '@/components/ReasonList';` and add `shortPath` to the existing `@/lib/labels` import (`import { humanKind, humanTool, shortPath } from '@/lib/labels';`).
  2. Replace the inline reasons block (the `<Fact label="Reasons">` `<ul>…ReasonItem…</ul>`, ~lines 369–379) with:
     ```tsx
     {ag && ag.reasons.length > 0 && (
       <Fact label="Reasons">
         <ReasonList reasons={ag.reasons} />
       </Fact>
     )}
     ```
  3. Delete the now-moved functions from `SlideOver.tsx`: `ReasonItem`, `reasonKey`, `asString`, `asStringArray`, and the local `shortPath` (it's imported now). The scope formatter that calls `shortPath` (~line 511) keeps working via the import.

- [ ] **Step 5: Verify the refactor is output-identical**

Run: `cd web && npx tsc -b --noEmit && npm run test && npm run lint && npm run build`
Expected: all clean. (The alerts `SlideOver` reason rendering is exercised by `web/tests/e2e/alerts.spec.ts`; run it in Task 10's full e2e.)

- [ ] **Step 6: Commit**

```bash
git add web/src/lib/labels.ts web/src/lib/labels.test.ts web/src/components/ReasonList.tsx web/src/components/AlertsQueue/SlideOver.tsx
git commit -m "$(cat <<'EOF'
refactor(web): extract shared ReasonList from SlideOver

Plan 04 T4. The AI Guard reason renderer (ReasonItem/reasonKey + asString/
asStringArray coercers) moves into components/ReasonList.tsx so the host
page can reuse it; shortPath moves to lib/labels (used by both the reason
renderer and SlideOver's scope formatter). Output-identical; SlideOver now
renders <ReasonList reasons={ag.reasons} />.

Co-Authored-By: Claude Opus 4.7 (1M context) <noreply@anthropic.com>
EOF
)"
```

---

## Task 5: Host route shell + `HostHeader` + states

**Files:**
- Create `web/src/routes/_authed/hosts/$hostId.tsx`
- Create `web/src/components/Host/HostHeader.tsx`

- [ ] **Step 1: Create `web/src/components/Host/HostHeader.tsx`**

```tsx
import { Link } from '@tanstack/react-router';
import { type ComplianceStatus, COMPLIANCE_META } from '@/lib/compliance';
import { relativeAge } from '@/lib/time';
import { cn } from '@/lib/utils';

interface Props {
  hostname: string | null;
  hostId: string;
  status: string;
  lastSeenTs: string;
  agentVersion: string;
  /** Derived from the compliance row, when this host appears in /fleet/compliance. */
  compliance?: ComplianceStatus;
}

/** Host detail header (UI/UX §5.3): identity + status + compliance + back link. */
export function HostHeader({
  hostname,
  hostId,
  status,
  lastSeenTs,
  agentVersion,
  compliance,
}: Props) {
  const tone = statusTone(status);
  const cm = compliance ? COMPLIANCE_META[compliance] : null;
  return (
    <div className="mb-4">
      <Link to="/fleet/risk" className="text-xs text-text-muted hover:text-text-primary">
        ◂ Fleet
      </Link>
      <div className="mt-1 flex flex-wrap items-center gap-3">
        <h1 className="text-lg font-semibold text-text-primary">
          {hostname ?? hostId.split('-')[0]}
        </h1>
        <Dot tone={tone} label={status} />
        {cm && <Pill tone={cm.tone}>{cm.label}</Pill>}
        <span className="text-xs text-text-muted">
          last seen <span title={lastSeenTs}>{relativeAge(lastSeenTs)} ago</span>
        </span>
        <span className="text-xs text-text-muted">agent v{agentVersion}</span>
        <span className="ml-auto font-mono text-[10px] text-text-subtle" title={hostId}>
          {hostId}
        </span>
      </div>
    </div>
  );
}

function statusTone(status: string): 'healthy' | 'degraded' | 'down' | 'subtle' {
  if (status === 'healthy') return 'healthy';
  if (status === 'stale') return 'degraded';
  if (status === 'disconnected') return 'down';
  return 'subtle';
}

function Dot({ tone, label }: { tone: 'healthy' | 'degraded' | 'down' | 'subtle'; label: string }) {
  const bg = {
    healthy: 'bg-status-healthy',
    degraded: 'bg-status-degraded',
    down: 'bg-status-down',
    subtle: 'bg-text-subtle',
  }[tone];
  return (
    <span className="flex items-center gap-1.5 text-xs text-text-muted">
      <span className={cn('inline-block h-1.5 w-1.5 rounded-full', bg)} />
      {label}
    </span>
  );
}

function Pill({ tone, children }: { tone: 'healthy' | 'degraded' | 'down'; children: React.ReactNode }) {
  const cls = {
    healthy: 'text-status-healthy border-status-healthy/40 bg-status-healthy/10',
    degraded: 'text-status-degraded border-status-degraded/40 bg-status-degraded/10',
    down: 'text-status-down border-status-down/40 bg-status-down/10',
  }[tone];
  return (
    <span className={cn('inline-block rounded border px-1.5 py-px text-[10px] uppercase tracking-wide', cls)}>
      {children}
    </span>
  );
}
```

- [ ] **Step 2: Create the route `web/src/routes/_authed/hosts/$hostId.tsx`** (blocks are placeholders here; Tasks 6–8 fill them):

```tsx
import { createFileRoute } from '@tanstack/react-router';
import { NotFoundError } from '@/api/client';
import { HostHeader } from '@/components/Host/HostHeader';
import { useFleetCompliance } from '@/hooks/useFleetCompliance';
import { useFleetHost } from '@/hooks/useFleetHost';
import { deriveComplianceStatus } from '@/lib/compliance';

export const Route = createFileRoute('/_authed/hosts/$hostId')({
  component: HostDetailPage,
});

function HostDetailPage() {
  const { hostId } = Route.useParams();
  const { host, isPending, error } = useFleetHost(hostId);

  // Compliance status is derived from the fleet-wide compliance feed (F13);
  // find this host's row, if present.
  const compliance = useFleetCompliance();
  const row = compliance.rows.find((r) => r.host_id === hostId);
  const status = row ? deriveComplianceStatus(row) : undefined;

  if (error instanceof NotFoundError) {
    return <NotFoundPanel hostId={hostId} />;
  }
  if (error) {
    return (
      <div className="px-4 py-6 text-sm text-sev-critical">Failed to load host: {error.message}</div>
    );
  }
  if (isPending || !host) {
    return <div className="space-y-3 px-1 py-2">
      <div className="h-5 w-48 animate-pulse rounded bg-bg-elevated" />
      <div className="h-24 w-full animate-pulse rounded bg-bg-elevated" />
      <div className="h-24 w-full animate-pulse rounded bg-bg-elevated" />
    </div>;
  }

  return (
    <div>
      <HostHeader
        hostname={host.hostname}
        hostId={host.host_id}
        status={host.status}
        lastSeenTs={host.last_seen_ts}
        agentVersion={host.agent_version}
        compliance={status}
      />
      {/* T6: <AiGuardByTool byTool={host.ai_guard?.by_tool ?? {}} /> */}
      {/* T7: <div className="grid gap-3 md:grid-cols-2"> HostMetaCard | PolicyHealthCard </div> */}
      {/* T8: per-host events section */}
    </div>
  );
}

function NotFoundPanel({ hostId }: { hostId: string }) {
  return (
    <div className="rounded-md border border-border bg-bg-surface px-6 py-10 text-center">
      <p className="text-sm text-text-primary">Host not found</p>
      <p className="mt-1 font-mono text-xs text-text-muted">{hostId}</p>
      <p className="mt-3 text-xs text-text-muted">
        It may have been evicted from the server's index. ◂{' '}
        <a className="text-accent hover:underline" href="/fleet/risk">Back to Fleet</a>
      </p>
    </div>
  );
}
```

- [ ] **Step 3: Verify route registration + states**

Run: `cd web && npx tsc -b --noEmit && npm run build`
Expected: clean; `routeTree.gen.ts` now includes `/hosts/$hostId` (fullPath `/hosts/$hostId`). The page compiles with header + placeholders.

- [ ] **Step 4: Manual check**

Run: `MOCK_FLEET=1 make dev-go` + `cd web && npm run dev`; open `/hosts/5a7c3e91-aaaa-bbbb-cccc-111111111111`.
Expected: header shows `alice-mbp`, a status dot, a compliance pill, last-seen, agent version, the host_id. `/hosts/nope` shows the "Host not found" panel.

- [ ] **Step 5: Commit**

```bash
git add web/src/routes/_authed/hosts/$hostId.tsx web/src/components/Host/HostHeader.tsx
git commit -m "$(cat <<'EOF'
feat(web): host detail route shell + HostHeader

Plan 04 T5. /hosts/$hostId fetches the host (20s) and derives its compliance
pill from the compliance feed; renders HostHeader (identity/status/compliance/
back link) with loading, error, and a dedicated 404 "host not found" panel.
AI Guard / meta / events blocks land in T6–T8.

Co-Authored-By: Claude Opus 4.7 (1M context) <noreply@anthropic.com>
EOF
)"
```

---

## Task 6: `AiGuardByTool` block

**Files:**
- Create `web/src/components/Host/AiGuardByTool.tsx`
- Modify `web/src/routes/_authed/hosts/$hostId.tsx` (wire it in)

- [ ] **Step 1: Create `web/src/components/Host/AiGuardByTool.tsx`**

```tsx
import type { Scope, ToolAiGuard } from '@/api/fleet';
import { ReasonList } from '@/components/ReasonList';
import { humanTool, shortPath } from '@/lib/labels';
import { cn } from '@/lib/utils';

interface Props {
  byTool: Record<string, ToolAiGuard>;
}

/**
 * AI Guard per-tool risk (UI/UX §5.3). Risky tools (bucket > low) get a card
 * with their reasons inline; quiet (low/0) tools collapse into one strip so
 * the signal isn't drowned out — most tools are quiet in real fleets.
 */
export function AiGuardByTool({ byTool }: Props) {
  const tools = Object.entries(byTool);
  if (tools.length === 0) {
    return (
      <Section>
        <p className="px-1 py-3 text-sm text-text-muted">No AI Guard assessments yet.</p>
      </Section>
    );
  }
  const risky = tools
    .filter(([, t]) => t.bucket !== 'low')
    .sort((a, b) => b[1].score - a[1].score);
  const quiet = tools.filter(([, t]) => t.bucket === 'low');

  return (
    <Section>
      {risky.length === 0 && (
        <p className="px-1 pb-2 text-sm text-text-muted">No tools above the low bucket.</p>
      )}
      <div className="space-y-2">
        {risky.map(([tool, t]) => (
          <ToolCard key={tool} tool={tool} t={t} />
        ))}
      </div>
      {quiet.length > 0 && (
        <details className="mt-3 text-xs text-text-muted">
          <summary className="cursor-pointer select-none">
            Low ({quiet.length}): {quiet.map(([tool]) => humanTool(tool)).join(' · ')}
          </summary>
          <div className="mt-1 space-y-0.5 pl-3">
            {quiet.map(([tool, t]) => (
              <div key={tool}>
                {humanTool(tool)} — {t.score.toFixed(1)}
              </div>
            ))}
          </div>
        </details>
      )}
    </Section>
  );
}

function Section({ children }: { children: React.ReactNode }) {
  return (
    <section className="mb-4">
      <h2 className="mb-2 text-xs font-medium uppercase tracking-wide text-text-subtle">
        AI Guard risk
      </h2>
      {children}
    </section>
  );
}

function ToolCard({ tool, t }: { tool: string; t: ToolAiGuard }) {
  return (
    <div className={cn('rounded-md border border-border bg-bg-surface p-3', bucketBorder(t.bucket))}>
      <div className="flex flex-wrap items-center gap-2 text-xs">
        <span className="font-medium text-text-primary">{humanTool(tool)}</span>
        <span className={cn('uppercase tracking-wide font-medium', bucketText(t.bucket))}>
          {t.bucket} {t.score.toFixed(2)}
        </span>
        <span className="text-text-subtle">{scopeLabel(t.scope)}</span>
        {t.is_reattestation && <span className="text-text-subtle">· re-attested</span>}
      </div>
      {t.reasons.length > 0 && (
        <div className="mt-2 text-xs">
          <ReasonList reasons={t.reasons} />
        </div>
      )}
    </div>
  );
}

function scopeLabel(scope: Scope): string {
  if (scope.kind === 'project') return `project · ${shortPath(scope.path)}`;
  if (scope.kind === 'application') return `app · ${scope.app}`;
  return 'user global';
}

function bucketText(bucket: string): string {
  switch (bucket) {
    case 'critical':
      return 'text-sev-critical';
    case 'high':
      return 'text-sev-high';
    case 'medium':
      return 'text-sev-medium';
    case 'low':
      return 'text-sev-low';
    default:
      return 'text-sev-info';
  }
}

function bucketBorder(bucket: string): string {
  switch (bucket) {
    case 'critical':
      return 'border-l-2 border-l-sev-critical';
    case 'high':
      return 'border-l-2 border-l-sev-high';
    case 'medium':
      return 'border-l-2 border-l-sev-medium';
    default:
      return '';
  }
}
```

- [ ] **Step 2: Wire it into the route** — in `web/src/routes/_authed/hosts/$hostId.tsx`, add the import and replace the `{/* T6 ... */}` comment:

```tsx
import { AiGuardByTool } from '@/components/Host/AiGuardByTool';
```
```tsx
      <AiGuardByTool byTool={host.ai_guard?.by_tool ?? {}} />
```

- [ ] **Step 3: Verify**

Run: `cd web && npx tsc -b --noEmit && npm run lint && npm run build`
Expected: clean. Manual: the alice host shows a `claude_code` card; quiet tools fold into the "Low (…)" strip.

- [ ] **Step 4: Commit**

```bash
git add web/src/components/Host/AiGuardByTool.tsx web/src/routes/_authed/hosts/$hostId.tsx
git commit -m "$(cat <<'EOF'
feat(web): host AI Guard by-tool block

Plan 04 T6. Risky tools (bucket>low) render as cards sorted by score with
reasons inline (shared ReasonList); quiet low/0 tools collapse into one
<details> strip. Handles the empty/no-ai-guard host gracefully.

Co-Authored-By: Claude Opus 4.7 (1M context) <noreply@anthropic.com>
EOF
)"
```

---

## Task 7: `HostMetaCard` + `PolicyHealthCard` (null-tolerant)

**Files:**
- Create `web/src/components/Host/HostMetaCard.tsx`
- Create `web/src/components/Host/PolicyHealthCard.tsx`
- Modify `web/src/routes/_authed/hosts/$hostId.tsx`

- [ ] **Step 1: Create `web/src/components/Host/HostMetaCard.tsx`**

```tsx
import type { HostMeta } from '@/api/fleet';

/** Host metadata block (§5.4 host_meta). Null = host hasn't sent a
 *  HostMetaSnapshot yet, NOT an error. */
export function HostMetaCard({ meta }: { meta: HostMeta | null }) {
  return (
    <Card title="Host metadata">
      {!meta ? (
        <Empty>No host metadata reported yet.</Empty>
      ) : (
        <dl className="space-y-1 text-xs">
          <Row label="OS">{meta.os_name} {meta.os_version}</Row>
          <Row label="Kernel">{meta.kernel_version}</Row>
          <Row label="Arch">{meta.architecture}</Row>
          <Row label="Gateway">{meta.default_gateway_v4 ?? '—'}</Row>
          <Row label="DNS">{meta.dns_servers.length ? meta.dns_servers.join(', ') : '—'}</Row>
          <Row label="Interfaces">
            <ul className="space-y-0.5">
              {meta.interfaces.map((i) => (
                <li key={i.name} className="font-mono">
                  {i.name}
                  {i.ipv4.length > 0 && <span className="text-text-subtle"> · {i.ipv4.join(', ')}</span>}
                  {i.mac && <span className="text-text-subtle"> · {i.mac}</span>}
                </li>
              ))}
            </ul>
          </Row>
        </dl>
      )}
    </Card>
  );
}

export function Card({ title, children }: { title: string; children: React.ReactNode }) {
  return (
    <section className="rounded-md border border-border bg-bg-surface p-3">
      <h2 className="mb-2 text-xs font-medium uppercase tracking-wide text-text-subtle">{title}</h2>
      {children}
    </section>
  );
}

export function Empty({ children }: { children: React.ReactNode }) {
  return <p className="text-xs text-text-muted">{children}</p>;
}

function Row({ label, children }: { label: string; children: React.ReactNode }) {
  return (
    <div className="flex gap-2">
      <dt className="w-20 shrink-0 text-text-subtle">{label}</dt>
      <dd className="text-text-body">{children}</dd>
    </div>
  );
}
```

- [ ] **Step 2: Create `web/src/components/Host/PolicyHealthCard.tsx`**

```tsx
import type { AgentHealth, PolicyState } from '@/api/fleet';
import { Card, Empty } from '@/components/Host/HostMetaCard';
import { relativeAge } from '@/lib/time';

interface Props {
  policy: PolicyState | null;
  health: AgentHealth | null;
  /** From the compliance row (F13); shown alongside policy when present. */
  signatureFailures24h?: number;
}

/** Policy state + agent health (§5.4). Disconnected hosts carry many nulls;
 *  every field degrades to "—" / "not reported yet". */
export function PolicyHealthCard({ policy, health, signatureFailures24h }: Props) {
  return (
    <Card title="Policy & agent health">
      <dl className="space-y-1 text-xs">
        {policy ? (
          <>
            <Row label="Policy">v{policy.last_applied_policy_version}{policy.policy_expired_active ? ' · expired' : ''}</Row>
            <Row label="Reloaded">{policy.last_policy_reload_ts ? `${relativeAge(policy.last_policy_reload_ts)} ago` : '—'}</Row>
          </>
        ) : (
          <Row label="Policy"><Empty>not reported yet</Empty></Row>
        )}
        {typeof signatureFailures24h === 'number' && (
          <Row label="Sig fails 24h">{signatureFailures24h}</Row>
        )}
        <div className="my-1 border-t border-border-subtle" />
        {health ? (
          <>
            <Row label="Heartbeat">{health.last_heartbeat_ts ? `${relativeAge(health.last_heartbeat_ts)} ago` : '—'}</Row>
            <Row label="Stalls 24h">{health.recent_channel_stalls_24h}</Row>
            <Row label="Watcher 24h">{health.recent_watcher_degraded_24h}</Row>
            <Row label="Sender lag 24h">{health.recent_sender_lag_critical_24h}</Row>
            <Row label="Hash p99">{health.hash_p99_ms_latest != null ? `${health.hash_p99_ms_latest} ms` : '—'}</Row>
          </>
        ) : (
          <Row label="Health"><Empty>not reported yet</Empty></Row>
        )}
      </dl>
    </Card>
  );
}

function Row({ label, children }: { label: string; children: React.ReactNode }) {
  return (
    <div className="flex gap-2">
      <dt className="w-28 shrink-0 text-text-subtle">{label}</dt>
      <dd className="text-text-body">{children}</dd>
    </div>
  );
}
```

- [ ] **Step 3: Wire into the route** — in `$hostId.tsx`, add imports and replace the `{/* T7 ... */}` comment. The compliance `row` (already computed in Task 5) supplies `signatureFailures24h`:

```tsx
import { HostMetaCard } from '@/components/Host/HostMetaCard';
import { PolicyHealthCard } from '@/components/Host/PolicyHealthCard';
```
```tsx
      <div className="mb-4 grid gap-3 md:grid-cols-2">
        <HostMetaCard meta={host.host_meta} />
        <PolicyHealthCard
          policy={host.policy_state}
          health={host.agent_health}
          signatureFailures24h={row?.signature_failures_24h}
        />
      </div>
```

- [ ] **Step 4: Verify**

Run: `cd web && npx tsc -b --noEmit && npm run lint && npm run build`
Expected: clean. Manual: alice shows OS/arch/interfaces + policy/health; a disconnected/null-heavy host shows "—"/"not reported yet" without crashing.

- [ ] **Step 5: Commit**

```bash
git add web/src/components/Host/HostMetaCard.tsx web/src/components/Host/PolicyHealthCard.tsx web/src/routes/_authed/hosts/$hostId.tsx
git commit -m "$(cat <<'EOF'
feat(web): host metadata + policy/agent-health cards

Plan 04 T7. Two-column null-tolerant blocks: HostMetaCard (OS/arch/gateway/
DNS/interfaces) and PolicyHealthCard (policy version/expiry/reload, sig fails
from the compliance row, agent-health counters/heartbeat/latency). Every
field degrades to "—" / "not reported yet" for disconnected hosts.

Co-Authored-By: Claude Opus 4.7 (1M context) <noreply@anthropic.com>
EOF
)"
```

---

## Task 8: Per-host events section

**Files:**
- Modify `web/src/routes/_authed/hosts/$hostId.tsx`

Reuses Plan 03's `EventsTable` + `useFleetEvents` with a `host_id` filter, and
deep-links "see all" to the Fleet Events tab's existing `host` search param.

- [ ] **Step 1: Add the events section to the route** — in `$hostId.tsx`, add imports and replace the `{/* T8 ... */}` comment:

```tsx
import { Link } from '@tanstack/react-router';
import { EventsTable } from '@/components/Fleet/EventsTable';
import { useFleetEvents } from '@/hooks/useFleetEvents';
```

Add inside `HostDetailPage`, before the `return` (after the existing hooks):

```tsx
  const events = useFleetEvents({ evidenceKinds: [], hostIDs: [hostId], since: null });
```

And the section (after the T7 grid):

```tsx
      <section>
        <div className="mb-2 flex items-center justify-between">
          <h2 className="text-xs font-medium uppercase tracking-wide text-text-subtle">
            Recent events
          </h2>
          <Link
            to="/fleet/events"
            search={{ host: [hostId] }}
            className="text-xs text-text-muted hover:text-text-primary"
          >
            see all in Fleet Events ▸
          </Link>
        </div>
        <div className="overflow-hidden rounded-md border border-border bg-bg-surface">
          {events.error ? (
            <div className="px-4 py-6 text-sm text-sev-critical">
              Failed to load events: {events.error.message}
            </div>
          ) : (
            <EventsTable rows={events.rows} isPending={events.isPending} />
          )}
        </div>
      </section>
```

> Note: `useFleetEvents` must be called unconditionally (hooks rule) — place it
> with the other hooks at the top of the component, above the early returns.
> The early `NotFoundError` / loading returns already sit below all hook calls.

- [ ] **Step 2: Verify**

Run: `cd web && npx tsc -b --noEmit && npm run lint && npm run build`
Expected: clean. The `to="/fleet/events" search={{ host: [hostId] }}` type-checks against the events route's `validateSearch` (`host?: string[]`). Manual: the section lists alice's events; "see all" navigates to `/fleet/events?host=<id>`.

- [ ] **Step 3: Commit**

```bash
git add web/src/routes/_authed/hosts/$hostId.tsx
git commit -m "$(cat <<'EOF'
feat(web): per-host recent events section

Plan 04 T8. Reuses EventsTable + useFleetEvents host_id-filtered for this
host, with a "see all" deep-link into the Fleet Events tab's host search
param (finally populating the filter Plan 03 only parsed).

Co-Authored-By: Claude Opus 4.7 (1M context) <noreply@anthropic.com>
EOF
)"
```

---

## Task 9: Link fleet-table hostnames to the host page

**Files:**
- Modify `web/src/components/Fleet/RiskTable.tsx`
- Modify `web/src/components/Fleet/ComplianceTable.tsx`
- Modify `web/src/components/Fleet/EventsTable.tsx`

The `/hosts/$hostId` route now exists (Task 5), so the typed `<Link to>` resolves.
Displayed text is unchanged; only the wrapping changes. Null-hostname hosts
still link (by `host_id`).

- [ ] **Step 1: `RiskTable.tsx`** — add `import { Link } from '@tanstack/react-router';` and replace the host cell's inner text:

Current:
```tsx
            <td className="px-3 py-2 font-mono text-text-primary" title={row.host_id}>
              {row.hostname ?? row.host_id.split('-')[0]}
            </td>
```
New:
```tsx
            <td className="px-3 py-2 font-mono text-text-primary" title={row.host_id}>
              <Link
                to="/hosts/$hostId"
                params={{ hostId: row.host_id }}
                className="hover:text-accent hover:underline"
              >
                {row.hostname ?? row.host_id.split('-')[0]}
              </Link>
            </td>
```

- [ ] **Step 2: `ComplianceTable.tsx`** — add the `Link` import and apply the same wrapping to its host cell (`{row.hostname ?? row.host_id.split('-')[0]}` inside the `<td … title={row.host_id}>`), `params={{ hostId: row.host_id }}`.

- [ ] **Step 3: `EventsTable.tsx`** — add the `Link` import and wrap its host cell. Note this table uses `ev.host_id` and shows only the prefix:

Current:
```tsx
              <td className="px-3 py-2 font-mono text-text-muted" title={ev.host_id}>
                {ev.host_id.split('-')[0]}
              </td>
```
New:
```tsx
              <td className="px-3 py-2 font-mono text-text-muted" title={ev.host_id}>
                <Link
                  to="/hosts/$hostId"
                  params={{ hostId: ev.host_id }}
                  className="hover:text-accent hover:underline"
                >
                  {ev.host_id.split('-')[0]}
                </Link>
              </td>
```

- [ ] **Step 4: Verify**

Run: `cd web && npx tsc -b --noEmit && npm run lint && npm run build`
Expected: clean. The Plan 03 `fleet.spec.ts` host-cell assertions still pass — `getByRole('cell', { name: … })` matches by accessible name, which is unchanged by wrapping the text in a link.

- [ ] **Step 5: Commit**

```bash
git add web/src/components/Fleet/RiskTable.tsx web/src/components/Fleet/ComplianceTable.tsx web/src/components/Fleet/EventsTable.tsx
git commit -m "$(cat <<'EOF'
feat(web): link fleet-table hostnames to /hosts/$hostId

Plan 04 T9. Activates the plain-text hostnames Plan 03 deliberately left
unlinked across Risk/Compliance/Events tables. Displayed text unchanged;
null-hostname hosts still link by host_id.

Co-Authored-By: Claude Opus 4.7 (1M context) <noreply@anthropic.com>
EOF
)"
```

---

## Task 10: E2E — host detail navigation, render, 404

**Files:**
- Modify `web/tests/e2e/fleet.spec.ts`

Mock host ids: alice `5a7c3e91-aaaa-bbbb-cccc-111111111111` (rich: host_meta +
ai_guard claude_code), dave `5a7c3e91-aaaa-bbbb-cccc-444444444444` (null
hostname, expired, no ai_guard).

- [ ] **Step 1: Append host-detail specs** to the `test.describe('fleet pages', …)` block in `web/tests/e2e/fleet.spec.ts`:

```ts
  test('clicking a hostname opens the host detail page', async ({ page }) => {
    await login(page);
    await page.goto('/fleet/risk');
    // alice-mbp is a risk row; its hostname cell is now a link.
    await page.getByRole('link', { name: /alice/ }).first().click();
    await expect(page).toHaveURL(/\/hosts\/5a7c3e91-aaaa-bbbb-cccc-111111111111/);
    // Header + AI Guard block render.
    await expect(page.getByRole('heading', { name: /alice-mbp/ })).toBeVisible({ timeout: 5_000 });
    await expect(page.getByText(/AI Guard risk/i)).toBeVisible();
    await expect(page.getByText(/claude.code/i).first()).toBeVisible();
  });

  test('host detail shows metadata + per-host events', async ({ page }) => {
    await login(page);
    await page.goto('/hosts/5a7c3e91-aaaa-bbbb-cccc-111111111111');
    await expect(page.getByText(/Host metadata/i)).toBeVisible({ timeout: 5_000 });
    await expect(page.getByText(/Policy & agent health/i)).toBeVisible();
    await expect(page.getByText(/Recent events/i)).toBeVisible();
  });

  test('unknown host id shows the not-found panel', async ({ page }) => {
    await login(page);
    await page.goto('/hosts/00000000-0000-0000-0000-000000000000');
    await expect(page.getByText(/Host not found/i)).toBeVisible({ timeout: 5_000 });
  });
```

- [ ] **Step 2: Run the full e2e suite (twice — flakiness check)**

Run (repo root): `make e2e`
Expected: all specs pass (the 16 from Plan 03 + 3 new = 19), green on two runs.

> If a selector is ambiguous (strict-mode), scope with `.first()` or a more
> specific role — same pattern as the existing fleet specs. The AI-Guard tool
> name renders via `humanTool('claude_code')` → "Claude Code", so the
> `/claude.code/i` regex (dot matches the space) is intentionally loose; adjust
> to the exact rendered string if needed.

- [ ] **Step 3: Commit**

```bash
git add web/tests/e2e/fleet.spec.ts
git commit -m "$(cat <<'EOF'
test(e2e): host detail — nav from table, render, 404

Plan 04 T10. Click a hostname → /hosts/$id; header + AI Guard + metadata +
events render; an unknown host id shows the not-found panel.

Co-Authored-By: Claude Opus 4.7 (1M context) <noreply@anthropic.com>
EOF
)"
```

---

## Task 11: README + ship

**Files:**
- Modify `README.md`

- [ ] **Step 1:** Confirm CI needs no change — `.github/workflows/ci.yml`'s e2e job runs `npx playwright test` (whole suite), so the new specs are auto-discovered. Go tests run via `go test ./...`.

- [ ] **Step 2: Update `README.md`** — replace the Plan-04 forward-reference and add a section. Find:

```markdown
Plan 04+ adds the `/hosts/:host` pages on top of the same foundation —
see [`docs/superpowers/plans/`](docs/superpowers/plans/).
```
Replace with:
```markdown
Plan 05+ adds the Settings page on top of the same foundation —
see [`docs/superpowers/plans/`](docs/superpowers/plans/).
```
And after the `## What's in Plan 03` section, add:
```markdown
## What's in Plan 04

Host detail at `/hosts/$hostId` (reached by clicking a hostname in any fleet
table): per-tool AI Guard risk with reasons, host metadata, policy state,
agent health, a derived compliance pill, and the host's recent events.
Read-only, 20s polling, null-tolerant for disconnected hosts.
```

- [ ] **Step 3: Full local gate**

Run: `make test && make lint && make build && make e2e`
Expected: all green — Go tests (incl. the new host handler tests), Vitest (incl. `shortPath`), golangci-lint, Biome, build, and 19 Playwright specs.

> Known local-only flake: chaining all gates can starve `alerts.spec.ts:19`;
> CI runs e2e as an isolated job. Re-run `make e2e` alone if a non-Plan-04 spec
> flakes.

- [ ] **Step 4: Commit**

```bash
git add README.md
git commit -m "$(cat <<'EOF'
docs(readme): add Plan 04 Host detail summary

Plan 04 T11. README notes the /hosts/$hostId page (AI Guard by-tool, metadata,
policy/health, compliance pill, recent events) and bumps the forward-reference
to Plan 05 (Settings). CI unchanged.

Co-Authored-By: Claude Opus 4.7 (1M context) <noreply@anthropic.com>
EOF
)"
```

- [ ] **Step 5: Push + open PR** (mirror Plan 03; do NOT merge — main needs explicit authorization)

```bash
git push -u origin feat/plan-04-host-detail
gh pr create --repo Ju571nK/sigil-manager --base main \
  --title "Plan 04: Host detail page (/hosts/\$hostId)" \
  --body "Implements docs/superpowers/specs/2026-05-26-plan-04-host-detail-design.md. …"
```

---

## Verification (end state of Plan 04)

A developer on `feat/plan-04-host-detail` with `MOCK_FLEET=1 make dev` can:
1. Open `/fleet/risk`, click a hostname → land on `/hosts/$hostId`.
2. See the header (hostname, status dot, compliance pill, last-seen, agent, host_id).
3. See AI Guard per-tool: risky tools as cards with inline reasons, quiet tools folded into a strip.
4. See host metadata (OS/arch/interfaces) and policy/agent-health, with nulls shown as "—"/"not reported yet".
5. See the host's recent events, and "see all" deep-links to `/fleet/events?host=<id>`.
6. Visit `/hosts/<unknown>` → "Host not found" panel.

CI green; `make test lint build e2e` pass locally (19 Playwright specs).

## What's out of Plan 04 (future)
- Plan 05 Settings; Plan 06 polish (light theme, responsive, broader a11y).
- No host-list page, no triage/writes on the host page, no per-tool history.
