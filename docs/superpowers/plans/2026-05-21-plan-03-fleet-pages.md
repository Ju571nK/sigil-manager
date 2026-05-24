# sigil-manager Plan 03: Fleet pages (Risk / Events / Compliance)

> **For agentic workers:** REQUIRED SUB-SKILL: Use
> `superpowers:subagent-driven-development` (recommended) or
> `superpowers:executing-plans` to implement this plan task-by-task. Steps
> use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Ship the Fleet section (UI/UX D6) — three tabs (`/fleet/risk`,
`/fleet/events`, `/fleet/compliance`) that let a SOC analyst browse the
fleet proactively. All read-only against `sigil-server`. End state: an
analyst clicks `Fleet` in the top nav, lands on `/fleet/risk`, switches
tabs, and each tab preserves its filters in the URL. CI green.

**Architecture:** Nested file-based routes under the existing `_authed`
layout (approach A): `_authed/fleet/route.tsx` renders a tab bar +
`<Outlet/>`; `risk.tsx` / `events.tsx` / `compliance.tsx` are sibling
routes; `index.tsx` redirects to `/fleet/risk`. The Go `FleetClient`
already implements `FleetRisk`, `FleetCompliance`, and `Events` (Plan 02,
tested), so the backend is just two thin `/api/v1` passthroughs. The
compliance status pill is derived **client-side** from raw signals
(contract F13). Fleet tabs poll at 20s (vs. the alert queue's 5s).

**Tech stack:** No new dependencies. Reuses everything from Plan 02 —
chi, TanStack Router/Query, shadcn `Tabs` (added in Plan 02 T9),
`date-fns`, Biome, Vitest, Playwright.

**Source-of-truth docs:**
- Design: `docs/superpowers/specs/2026-05-21-plan-03-fleet-pages-design.md`
- Contract: `docs/superpowers/specs/2026-05-16-fleet-api-contract.md`
  (§5.5 risk, §5.6 compliance, §5.7 events; §13.1 `open_alert_count_24h`)
- UI/UX: `docs/superpowers/specs/2026-05-16-ui-ux-design.md` (§5.2, §8)

**Out of scope (later plans):**
- `/hosts/:hostname` detail + hostname→host_id resolution + clickable
  hostnames (Plan 04). Hostnames render as plain text here.
- Real Settings page (Plan 05). Light theme / a11y (Plan 06).
- Client-side recomputation of a precise alert count — we show the
  server's coarse warn count with a caveat (issue #21).

---

## Prerequisites

The implementing engineer needs locally (all already required by Plan 02):
- **Go 1.22+**, **Node 20 LTS+**, **npm 10+**
- **golangci-lint**, **air**, **make**
- Develop against the mock client (`MOCK_FLEET=1`) — no running
  `sigil-server` required. The Plan 02 mock already seeds 5 hosts with
  mixed risk + policy state; T2 tunes it so all four compliance states
  are represented.

Branch off the current `main` (which has Plan 02 + the 3b.3.1/3b.7 sync).

---

## File structure (delta over Plan 02)

```
internal/api/v1/
  fleet.go              MODIFY  + handleFleetRisk, handleFleetCompliance, parseRiskParams
  server.go             MODIFY  + 2 routes under the authed group
  handlers_test.go      MODIFY  + risk/compliance cases

internal/fleet/
  mock.go               MODIFY  per-host signature_failures so 4 compliance states exist
  mock_test.go          MODIFY  + assertion that the 4 states are present

web/src/api/fleet.ts    MODIFY  + Risk/Compliance types + fetchFleetRisk/fetchFleetCompliance
web/src/lib/
  labels.ts             ADD     shared humanTool() (dedups 3 callers)
  labels.test.ts        ADD
  compliance.ts         ADD     deriveComplianceStatus
  compliance.test.ts    ADD
web/src/hooks/
  useFleetQuery.ts      ADD     shared 20s-poll base
  useFleetRisk.ts       ADD
  useFleetCompliance.ts ADD
  useFleetEvents.ts     ADD
web/src/components/Fleet/
  FleetTabs.tsx         ADD
  RiskTable.tsx         ADD
  ComplianceTable.tsx   ADD
  EventsTable.tsx       ADD
web/src/components/Layout/TopNav.tsx   MODIFY  activate Fleet link
web/src/components/AlertsQueue/SlideOver.tsx  MODIFY  import humanTool from labels
web/src/components/AlertsQueue/QueueRow.tsx   MODIFY  import humanTool from labels
web/src/routes/_authed/fleet/
  route.tsx             ADD     tab-bar layout + <Outlet/>
  index.tsx             ADD     redirect to /fleet/risk
  risk.tsx              ADD
  events.tsx            ADD
  compliance.tsx        ADD
web/tests/e2e/fleet.spec.ts            ADD
```

Note: TanStack Router's file-based routing auto-generates `routeTree.gen.ts`
(gitignored). The Vite plugin regenerates it on `npm run dev` / `npm run
build`; no manual edit needed.

---

## Task 0: Branch setup

**Files:** none (git only)

- [ ] **Step 1:** From an up-to-date `main`, create the feature branch:

```bash
git checkout main && git pull --ff-only origin main
git checkout -b feat/plan-03-fleet-pages
```

- [ ] **Step 2:** Confirm the tree is clean and the binary builds:

```bash
make build
```

Expected: `./sigil-manager` builds with no errors.

---

## Task 1: Backend — `/api/v1/fleet/risk` + `/fleet/compliance`

**Files:**
- MODIFY `internal/api/v1/fleet.go` (add 2 handlers + `parseRiskParams`)
- MODIFY `internal/api/v1/server.go` (add 2 routes)
- MODIFY `internal/api/v1/handlers_test.go` (add cases)

The `FleetClient` already exposes `FleetRisk(ctx, RiskParams)` and
`FleetCompliance(ctx, ComplianceParams)` returning `*RiskPage` /
`*CompliancePage` (both already have JSON tags). These handlers are pure
passthroughs that reuse the existing `mapFleetErr` + `splitComma` helpers.

- [ ] **Step 1: Write the failing handler tests.** Append to
  `internal/api/v1/handlers_test.go`. Match the existing test style in
  that file (it builds a `Server` with `fleet.NewMock(...)` and an
  in-memory triage repo, issues authed requests, asserts on the JSON).
  Use the same auth-cookie helper the existing tests use (find it near
  the top of the file — it logs in and returns the `sigil_session`
  cookie).

```go
func TestHandleFleetRisk_PassThroughSortedRows(t *testing.T) {
	srv, cookie := newTestServerWithAuth(t)

	rr := doAuthedGet(t, srv, "/fleet/risk?limit=100", cookie)
	require.Equal(t, http.StatusOK, rr.Code)

	var body struct {
		Rows []struct {
			HostID            string  `json:"host_id"`
			Score             float64 `json:"score"`
			Bucket            string  `json:"bucket"`
			OpenAlertCount24h int     `json:"open_alert_count_24h"`
		} `json:"rows"`
		NextCursor *string `json:"next_cursor"`
	}
	require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &body))
	require.NotEmpty(t, body.Rows, "mock seeds >=1 host with AI guard risk")
	// §5.5: rows are sorted by score desc.
	for i := 1; i < len(body.Rows); i++ {
		require.GreaterOrEqual(t, body.Rows[i-1].Score, body.Rows[i].Score)
	}
}

func TestHandleFleetRisk_BadLimitIs400(t *testing.T) {
	srv, cookie := newTestServerWithAuth(t)
	rr := doAuthedGet(t, srv, "/fleet/risk?limit=abc", cookie)
	require.Equal(t, http.StatusBadRequest, rr.Code)
}

func TestHandleFleetRisk_Unauthed401(t *testing.T) {
	srv, _ := newTestServerWithAuth(t)
	rr := doGet(t, srv, "/fleet/risk")
	require.Equal(t, http.StatusUnauthorized, rr.Code)
}

func TestHandleFleetCompliance_PassThroughRawSignals(t *testing.T) {
	srv, cookie := newTestServerWithAuth(t)

	rr := doAuthedGet(t, srv, "/fleet/compliance?limit=100", cookie)
	require.Equal(t, http.StatusOK, rr.Code)

	var body struct {
		Rows []struct {
			HostID               string `json:"host_id"`
			VersionDrift         int    `json:"version_drift"`
			PolicyExpiredActive  bool   `json:"policy_expired_active"`
			SignatureFailures24h int    `json:"signature_failures_24h"`
		} `json:"rows"`
	}
	require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &body))
	require.NotEmpty(t, body.Rows)
	// Server exposes raw signals only — no derived status field (F13).
	require.NotContains(t, rr.Body.String(), "compliance_score")
	require.NotContains(t, rr.Body.String(), `"status"`)
}
```

> If the existing test file uses different helper names than
> `newTestServerWithAuth` / `doAuthedGet` / `doGet`, reuse whatever the
> file already defines — read the top of `handlers_test.go` first and
> mirror its conventions exactly. Do not invent new helpers if equivalents
> exist.

- [ ] **Step 2: Run the tests to verify they fail.**

Run: `go test ./internal/api/v1/ -run TestHandleFleet -v`
Expected: FAIL — `handleFleetRisk` / `handleFleetCompliance` undefined (or
routes return 404).

- [ ] **Step 3: Add the handlers to `internal/api/v1/fleet.go`.** Append
  after `handleFleetEventByID` (before `lookupTriageView`):

```go
// handleFleetRisk is a pass-through to FleetClient.FleetRisk (§5.5). The
// RiskPage already carries JSON tags so we relay it verbatim. Note the
// open_alert_count_24h caveat (contract §13.1 / issue #21) is a rendering
// concern, handled SPA-side — the server relays the producer's number.
func (s *Server) handleFleetRisk(w http.ResponseWriter, r *http.Request) {
	params, err := parseRiskParams(r)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid_query", err.Error())
		return
	}
	page, err := s.Fleet.FleetRisk(r.Context(), params)
	if err != nil {
		mapFleetErr(w, err)
		return
	}
	httputil.WriteJSON(w, http.StatusOK, page)
}

// handleFleetCompliance is a pass-through to FleetClient.FleetCompliance
// (§5.6). Per F13 the server returns raw signals only — the status pill is
// derived SPA-side (web/src/lib/compliance.ts).
func (s *Server) handleFleetCompliance(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	params := fleet.ComplianceParams{Cursor: q.Get("cursor")}
	if v := q.Get("limit"); v != "" {
		n, err := strconv.Atoi(v)
		if err != nil {
			writeError(w, http.StatusBadRequest, "invalid_query", "limit must be an integer")
			return
		}
		params.Limit = n
	}
	page, err := s.Fleet.FleetCompliance(r.Context(), params)
	if err != nil {
		mapFleetErr(w, err)
		return
	}
	httputil.WriteJSON(w, http.StatusOK, page)
}

// parseRiskParams translates the public query string into a
// [fleet.RiskParams] (§5.5). `tool` is a comma list; `min_bucket` is
// relayed as-is (the client clamps unknown values).
func parseRiskParams(r *http.Request) (fleet.RiskParams, error) {
	q := r.URL.Query()
	out := fleet.RiskParams{
		Cursor:    q.Get("cursor"),
		Tool:      splitComma(q.Get("tool")),
		MinBucket: q.Get("min_bucket"),
	}
	if v := q.Get("limit"); v != "" {
		n, err := strconv.Atoi(v)
		if err != nil {
			return out, errors.New("limit must be an integer")
		}
		out.Limit = n
	}
	return out, nil
}
```

- [ ] **Step 4: Wire the routes in `internal/api/v1/server.go`.** Inside
  the authenticated `r.Group(...)`, after the existing
  `r.Get("/fleet/events/{event_id}", ...)` line, add:

```go
		r.Get("/fleet/risk", s.handleFleetRisk)
		r.Get("/fleet/compliance", s.handleFleetCompliance)
```

Also update the route-table doc comment above `Routes()` to list the two
new lines (keep the comment honest):

```go
//	GET    /fleet/risk                       (cookie)
//	GET    /fleet/compliance                 (cookie)
```

- [ ] **Step 5: Run the tests to verify they pass.**

Run: `go test ./internal/api/v1/ -run TestHandleFleet -v`
Expected: PASS (all four new tests + the existing fleet/events tests).

- [ ] **Step 6: Run the whole Go suite + lint.**

Run: `go test ./... && golangci-lint run ./...`
Expected: ok, 0 issues.

- [ ] **Step 7: Commit.**

```bash
git add internal/api/v1/fleet.go internal/api/v1/server.go internal/api/v1/handlers_test.go
git commit -m "$(cat <<'EOF'
feat(api): /api/v1/fleet/risk + /fleet/compliance passthroughs

Plan 03 T1. Two thin handlers over the existing FleetClient.FleetRisk /
FleetCompliance (built + tested in Plan 02). risk parses tool/min_bucket/
limit/cursor; compliance parses limit/cursor. Both relay the page verbatim
(RiskPage/CompliancePage already carry JSON tags) and reuse mapFleetErr so
upstream URLs never leak. Compliance stays raw-signals-only per F13 — the
status pill is derived SPA-side.

Co-Authored-By: Claude Opus 4.7 (1M context) <noreply@anthropic.com>
EOF
)"
```

---

## Task 2: Mock fixture — represent all four compliance states

**Files:**
- MODIFY `internal/fleet/mock.go` (per-host signature failures)
- MODIFY `internal/fleet/mock_test.go` (assert the four states exist)

Today the mock hardcodes `SignatureFailures24h: 0` for every host, so the
"Failing signature" derived state is never exercised. The mock's 5 hosts
already cover in-sync (alice/eve, drift 0), drift (bob/carol), and expired
(dave). We add per-host signature failures and set one host (carol) to a
nonzero value so all four states appear — and carol also has drift, which
verifies the SPA's priority rule (failing-signature outranks drift).

- [ ] **Step 1: Write the failing assertion.** Append to
  `internal/fleet/mock_test.go`:

```go
func TestMock_ComplianceCoversAllRawSignalStates(t *testing.T) {
	m := newTestMock(t)
	page, err := m.FleetCompliance(context.Background(), ComplianceParams{Limit: 100})
	require.NoError(t, err)

	var sawInSync, sawDrift, sawExpired, sawSigFail bool
	for _, row := range page.Rows {
		switch {
		case row.PolicyExpiredActive:
			sawExpired = true
		case row.SignatureFailures24h > 0:
			sawSigFail = true
		case row.VersionDrift > 0:
			sawDrift = true
		default:
			sawInSync = true
		}
	}
	require.True(t, sawInSync, "need a host with no drift/expiry/sig-failures")
	require.True(t, sawDrift, "need a host with version_drift > 0")
	require.True(t, sawExpired, "need a host with policy_expired_active")
	require.True(t, sawSigFail, "need a host with signature_failures_24h > 0")
}
```

- [ ] **Step 2: Run it to verify it fails.**

Run: `go test ./internal/fleet/ -run TestMock_ComplianceCoversAllRawSignalStates -v`
Expected: FAIL on `sawSigFail` (all hosts currently have 0 signature failures).

- [ ] **Step 3: Add a per-host signature-failures map.** In
  `internal/fleet/mock.go`:

  (a) Add the field to the `MockClient` struct, next to `versionDrift`:

```go
	versionDrift map[string]int // host_id → server_current_policy_version - last_applied
	sigFailures  map[string]int // host_id → signature_failures_24h
```

  (b) Initialize it in `NewMock`:

```go
	m := &MockClient{seed: seed, hostDetails: map[string]*HostDetail{}, versionDrift: map[string]int{}, sigFailures: map[string]int{}}
```

  (c) Add a `sigFail` field to the `seed` struct in `buildHosts` (next to
  `expired`, `warn`, `info`):

```go
		expired       bool
		sigFail       int
```

  (d) Set carol's seed `sigFail: 3` (carol is the `stale` host with
  `policyVer: 16`). Find the carol seed entry and add the field:

```go
		{
			id: "5a7c3e91-aaaa-bbbb-cccc-333333333333", name: "carol-dev",
			status: "stale", lastSeenDelta: -30 * time.Minute,
			risk: &CurrentRisk{
				MaxScore: 6.8, MaxBucket: "high",
				ByTool: map[string]ToolRisk{
					"claude_desktop": {Score: 6.8, Bucket: "high", AssessedTS: m.seed.Add(-1 * time.Hour)},
				},
			},
			hostMeta: hostMetaCarol, policyVer: 16, warn: 8, info: 410, sigFail: 3,
		},
```

  (e) In the seed loop where `m.versionDrift[s.id] = ...` is set, also
  store the signature failures:

```go
		m.sigFailures[s.id] = s.sigFail
```

  (f) In `FleetCompliance`, replace the hardcoded `SignatureFailures24h: 0`
  with the per-host value:

```go
			SignatureFailures24h:       m.sigFailures[h.HostID],
```

- [ ] **Step 4: Run the new test + the full fleet suite.**

Run: `go test ./internal/fleet/ -v`
Expected: PASS (the new test + all existing mock/http tests).

- [ ] **Step 5: Commit.**

```bash
git add internal/fleet/mock.go internal/fleet/mock_test.go
git commit -m "$(cat <<'EOF'
test(fleet): mock seeds all four compliance raw-signal states

Plan 03 T2. Adds a per-host signature_failures_24h map (was hardcoded 0).
carol now reports 3 signature failures (and already has version drift), so
the fixture exercises in-sync / drift / expired / failing-signature — and
the failing-signature-over-drift priority the SPA derivation must honor.

Co-Authored-By: Claude Opus 4.7 (1M context) <noreply@anthropic.com>
EOF
)"
```

---

## Task 3: Frontend API client — Risk + Compliance types & fetchers

**Files:**
- MODIFY `web/src/api/fleet.ts`

Mirror the Go wire shapes (`internal/fleet/client.go` `RiskRow` /
`ComplianceRow`) exactly.

- [ ] **Step 1: Add the types + fetchers.** Append to
  `web/src/api/fleet.ts` (after the events helpers, before
  `extractAiGuard`):

```ts
// -----------------------------------------------------------------------------
// Fleet Risk (/api/v1/fleet/risk — contract §5.5)
// -----------------------------------------------------------------------------

/** One row of `/api/v1/fleet/risk.rows`. Mirrors fleet.RiskRow. */
export interface RiskRow {
  host_id: string;
  hostname: string | null;
  score: number; // 0.0–10.0 (F7)
  bucket: 'low' | 'medium' | 'high' | 'critical' | string;
  top_tool: string;
  reasons_count: number;
  assessed_ts: string;
  /**
   * Trailing-24h warn-event count. Per contract §13.1 / issue #21 this is
   * NOT alert-definition filtered — render it as "Warn 24h", not "alerts".
   */
  open_alert_count_24h: number;
}

export interface RiskPage {
  rows: RiskRow[];
  next_cursor: string | null;
}

export interface RiskParams {
  cursor?: string;
  limit?: number;
  tool?: string[]; // comma-joined
  min_bucket?: 'low' | 'medium' | 'high' | 'critical';
}

export function fetchFleetRisk(params: RiskParams = {}): Promise<RiskPage> {
  const q = new URLSearchParams();
  if (params.cursor) q.set('cursor', params.cursor);
  if (typeof params.limit === 'number') q.set('limit', String(params.limit));
  if (params.tool?.length) q.set('tool', params.tool.join(','));
  if (params.min_bucket) q.set('min_bucket', params.min_bucket);
  const s = q.toString();
  return api<RiskPage>(`/fleet/risk${s ? `?${s}` : ''}`);
}

// -----------------------------------------------------------------------------
// Fleet Compliance (/api/v1/fleet/compliance — contract §5.6)
// -----------------------------------------------------------------------------

/** One row of `/api/v1/fleet/compliance.rows`. Mirrors fleet.ComplianceRow. */
export interface ComplianceRow {
  host_id: string;
  hostname: string | null;
  last_applied_policy_version: number;
  server_current_policy_version: number;
  version_drift: number;
  policy_expired_active: boolean;
  last_policy_reload_ts: string | null;
  signature_failures_24h: number;
}

export interface CompliancePage {
  rows: ComplianceRow[];
  next_cursor: string | null;
}

export interface ComplianceParams {
  cursor?: string;
  limit?: number;
}

export function fetchFleetCompliance(params: ComplianceParams = {}): Promise<CompliancePage> {
  const q = new URLSearchParams();
  if (params.cursor) q.set('cursor', params.cursor);
  if (typeof params.limit === 'number') q.set('limit', String(params.limit));
  const s = q.toString();
  return api<CompliancePage>(`/fleet/compliance${s ? `?${s}` : ''}`);
}
```

- [ ] **Step 2: Verify it type-checks.**

Run: `cd web && npx tsc -b --noEmit`
Expected: no errors.

- [ ] **Step 3: Lint.**

Run: `cd web && npm run lint`
Expected: no fixes / no errors.

- [ ] **Step 4: Commit.**

```bash
git add web/src/api/fleet.ts
git commit -m "$(cat <<'EOF'
feat(web): fleet risk + compliance API client types

Plan 03 T3. Adds RiskRow/RiskPage/RiskParams + ComplianceRow/CompliancePage/
ComplianceParams mirroring the Go wire shapes, plus fetchFleetRisk /
fetchFleetCompliance over the existing typed fetch wrapper. open_alert_count_24h
documented as the issue-#21 coarse warn count.

Co-Authored-By: Claude Opus 4.7 (1M context) <noreply@anthropic.com>
EOF
)"
```

---

## Task 4: Shared `humanTool()` label module

**Files:**
- ADD `web/src/lib/labels.ts`
- ADD `web/src/lib/labels.test.ts`
- MODIFY `web/src/components/AlertsQueue/SlideOver.tsx` (import, drop local copy)
- MODIFY `web/src/components/AlertsQueue/QueueRow.tsx` (import, drop local copy)

Plan 02 has two identical `humanTool()` copies; Plan 03 adds a third
consumer (Risk/Events rows). Extract one shared copy now (targeted dedup,
justified by the new consumer).

- [ ] **Step 1: Write the failing test.** Create
  `web/src/lib/labels.test.ts`:

```ts
import { describe, expect, it } from 'vitest';
import { humanTool } from './labels';

describe('humanTool', () => {
  it('maps the six known tool wire strings to display names', () => {
    expect(humanTool('claude_code')).toBe('Claude Code');
    expect(humanTool('claude_desktop')).toBe('Claude Desktop');
    expect(humanTool('continue_dev')).toBe('Continue.dev');
    expect(humanTool('codex')).toBe('Codex');
    expect(humanTool('gemini')).toBe('Gemini');
    expect(humanTool('cursor')).toBe('Cursor');
  });

  it('falls back to the raw string for unknown tools', () => {
    expect(humanTool('future_tool')).toBe('future_tool');
  });
});
```

- [ ] **Step 2: Run it to verify it fails.**

Run: `cd web && npm test -- --run src/lib/labels.test.ts`
Expected: FAIL — cannot resolve `./labels`.

- [ ] **Step 3: Create `web/src/lib/labels.ts`.**

```ts
/** Maps an AI-tool wire string (contract §14.5/§14.7) to a display name. */
export function humanTool(tool: string): string {
  switch (tool) {
    case 'claude_code':
      return 'Claude Code';
    case 'claude_desktop':
      return 'Claude Desktop';
    case 'continue_dev':
      return 'Continue.dev';
    case 'codex':
      return 'Codex';
    case 'gemini':
      return 'Gemini';
    case 'cursor':
      return 'Cursor';
    default:
      return tool;
  }
}
```

- [ ] **Step 4: Run the test to verify it passes.**

Run: `cd web && npm test -- --run src/lib/labels.test.ts`
Expected: PASS.

- [ ] **Step 5: Replace the two local copies with the import.**

  (a) In `web/src/components/AlertsQueue/SlideOver.tsx`: delete the local
  `function humanTool(tool: string): string { ... }` definition and add to
  the imports at the top:

```ts
import { humanTool } from '@/lib/labels';
```

  (b) In `web/src/components/AlertsQueue/QueueRow.tsx`: delete its local
  `function humanTool(...)` definition and add the same import.

- [ ] **Step 6: Verify type-check + lint + the existing e2e still pass.**

Run: `cd web && npx tsc -b --noEmit && npm run lint`
Expected: no errors. (The Biome import-sort rule may reorder the new
import — run `npx biome check --write src/` if it flags, then re-lint.)

- [ ] **Step 7: Commit.**

```bash
git add web/src/lib/labels.ts web/src/lib/labels.test.ts web/src/components/AlertsQueue/SlideOver.tsx web/src/components/AlertsQueue/QueueRow.tsx
git commit -m "$(cat <<'EOF'
refactor(web): extract shared humanTool() into lib/labels

Plan 03 T4. Plan 02 had two identical humanTool() copies (SlideOver +
QueueRow); Plan 03's fleet rows are a third consumer. One shared copy in
lib/labels.ts (covers the six tool strings + raw fallback) with a Vitest
truth table. No behavior change.

Co-Authored-By: Claude Opus 4.7 (1M context) <noreply@anthropic.com>
EOF
)"
```

---

## Task 5: Compliance status derivation

**Files:**
- ADD `web/src/lib/compliance.ts`
- ADD `web/src/lib/compliance.test.ts`

Pure function deriving the status pill from raw signals (contract §5.6),
single worst state by priority: Expired > Failing signature > Drift >
In sync.

- [ ] **Step 1: Write the failing test.** Create
  `web/src/lib/compliance.test.ts`:

```ts
import { describe, expect, it } from 'vitest';
import type { ComplianceRow } from '@/api/fleet';
import { deriveComplianceStatus } from './compliance';

function row(partial: Partial<ComplianceRow>): ComplianceRow {
  return {
    host_id: 'h',
    hostname: 'h',
    last_applied_policy_version: 18,
    server_current_policy_version: 18,
    version_drift: 0,
    policy_expired_active: false,
    last_policy_reload_ts: null,
    signature_failures_24h: 0,
    ...partial,
  };
}

describe('deriveComplianceStatus', () => {
  it('returns in_sync when all signals are clean', () => {
    expect(deriveComplianceStatus(row({}))).toBe('in_sync');
  });

  it('returns drift when version_drift > 0', () => {
    expect(deriveComplianceStatus(row({ version_drift: 2 }))).toBe('drift');
  });

  it('returns expired when policy_expired_active', () => {
    expect(deriveComplianceStatus(row({ policy_expired_active: true }))).toBe('expired');
  });

  it('returns failing_signature when signature_failures_24h > 0', () => {
    expect(deriveComplianceStatus(row({ signature_failures_24h: 1 }))).toBe('failing_signature');
  });

  it('prioritizes expired over everything', () => {
    expect(
      deriveComplianceStatus(
        row({ policy_expired_active: true, signature_failures_24h: 5, version_drift: 3 }),
      ),
    ).toBe('expired');
  });

  it('prioritizes failing_signature over drift', () => {
    expect(
      deriveComplianceStatus(row({ signature_failures_24h: 2, version_drift: 3 })),
    ).toBe('failing_signature');
  });
});
```

- [ ] **Step 2: Run it to verify it fails.**

Run: `cd web && npm test -- --run src/lib/compliance.test.ts`
Expected: FAIL — cannot resolve `./compliance`.

- [ ] **Step 3: Create `web/src/lib/compliance.ts`.**

```ts
import type { ComplianceRow } from '@/api/fleet';

export type ComplianceStatus = 'in_sync' | 'drift' | 'expired' | 'failing_signature';

/**
 * Derives the per-host compliance pill from raw signals (contract §5.6).
 * The server exposes no compliance_score (F13); the rule lives here.
 * Single worst state, priority: expired > failing_signature > drift > in_sync.
 */
export function deriveComplianceStatus(row: ComplianceRow): ComplianceStatus {
  if (row.policy_expired_active) return 'expired';
  if (row.signature_failures_24h > 0) return 'failing_signature';
  if (row.version_drift > 0) return 'drift';
  return 'in_sync';
}

/** Display metadata for each status — label + the severity token to color it. */
export const COMPLIANCE_META: Record<
  ComplianceStatus,
  { label: string; tone: 'healthy' | 'degraded' | 'down' }
> = {
  in_sync: { label: 'In sync', tone: 'healthy' },
  drift: { label: 'Drift', tone: 'degraded' },
  expired: { label: 'Expired', tone: 'down' },
  failing_signature: { label: 'Failing signature', tone: 'down' },
};
```

- [ ] **Step 4: Run the test to verify it passes.**

Run: `cd web && npm test -- --run src/lib/compliance.test.ts`
Expected: PASS (6 tests).

- [ ] **Step 5: Lint + type-check.**

Run: `cd web && npx tsc -b --noEmit && npm run lint`
Expected: clean.

- [ ] **Step 6: Commit.**

```bash
git add web/src/lib/compliance.ts web/src/lib/compliance.test.ts
git commit -m "$(cat <<'EOF'
feat(web): client-side compliance status derivation

Plan 03 T5. deriveComplianceStatus() maps raw signals (contract §5.6) to
a single worst-state pill: expired > failing_signature > drift > in_sync.
Server exposes no compliance_score (F13), so the rule lives consumer-side.
COMPLIANCE_META carries label + color tone. Vitest covers each state +
the two priority orderings.

Co-Authored-By: Claude Opus 4.7 (1M context) <noreply@anthropic.com>
EOF
)"
```

---

## Task 6: Fleet query hooks

**Files:**
- ADD `web/src/hooks/useFleetQuery.ts`
- ADD `web/src/hooks/useFleetRisk.ts`
- ADD `web/src/hooks/useFleetCompliance.ts`
- ADD `web/src/hooks/useFleetEvents.ts`

20s slow-poll (vs. the alert queue's 5s). A shared base centralizes the
interval + stale config.

- [ ] **Step 1: Create the shared base `web/src/hooks/useFleetQuery.ts`.**

```ts
import { type QueryKey, useQuery } from '@tanstack/react-query';

/** Fleet tabs poll slower than the alert queue (UI/UX §7.2 sets 5s for alerts). */
export const FLEET_POLL_INTERVAL_MS = 20_000;

/**
 * Thin wrapper over useQuery with the fleet-page polling defaults applied.
 * Keeps the three fleet hooks consistent and makes the interval one edit.
 */
export function useFleetQuery<T>(key: QueryKey, queryFn: () => Promise<T>) {
  return useQuery({
    queryKey: key,
    queryFn,
    refetchInterval: FLEET_POLL_INTERVAL_MS,
    refetchIntervalInBackground: false,
  });
}
```

- [ ] **Step 2: Create `web/src/hooks/useFleetRisk.ts`.**

```ts
import { type RiskParams, fetchFleetRisk } from '@/api/fleet';
import { useFleetQuery } from './useFleetQuery';

export interface RiskFilter {
  minBucket: 'low' | 'medium' | 'high' | 'critical';
  tool: string[]; // empty = all tools
}

export const DEFAULT_RISK_FILTER: RiskFilter = { minBucket: 'low', tool: [] };

export function useFleetRisk(filter: RiskFilter) {
  const params: RiskParams = {
    limit: 100,
    min_bucket: filter.minBucket,
    tool: filter.tool.length ? filter.tool : undefined,
  };
  const q = useFleetQuery(['fleet', 'risk', params], () => fetchFleetRisk(params));
  return {
    rows: q.data?.rows ?? [],
    isPending: q.isPending && !q.data,
    error: q.error,
    isFetching: q.isFetching,
    lastUpdatedAt: q.dataUpdatedAt,
    refetch: q.refetch,
  };
}
```

- [ ] **Step 3: Create `web/src/hooks/useFleetCompliance.ts`.**

```ts
import { fetchFleetCompliance } from '@/api/fleet';
import { useFleetQuery } from './useFleetQuery';

export function useFleetCompliance() {
  const q = useFleetQuery(['fleet', 'compliance'], () => fetchFleetCompliance({ limit: 100 }));
  return {
    rows: q.data?.rows ?? [],
    isPending: q.isPending && !q.data,
    error: q.error,
    isFetching: q.isFetching,
    lastUpdatedAt: q.dataUpdatedAt,
    refetch: q.refetch,
  };
}
```

- [ ] **Step 4: Create `web/src/hooks/useFleetEvents.ts`.** Fleet-wide
  timeline — no bucket filter, all severities/kinds; optional
  event-type / host / since filters.

```ts
import { type EventsParams, fleetEvents } from '@/api/fleet';
import { useFleetQuery } from './useFleetQuery';

export interface FleetEventsFilter {
  evidenceKinds: string[]; // empty = all kinds
  hostIDs: string[]; // empty = all hosts
  since: string | null;
}

export const DEFAULT_FLEET_EVENTS_FILTER: FleetEventsFilter = {
  evidenceKinds: [],
  hostIDs: [],
  since: null,
};

export function useFleetEvents(filter: FleetEventsFilter) {
  const params: EventsParams = {
    limit: 100,
    evidence_kind: filter.evidenceKinds.length ? filter.evidenceKinds : undefined,
    host_id: filter.hostIDs.length ? filter.hostIDs : undefined,
    since: filter.since ?? undefined,
  };
  const q = useFleetQuery(['fleet', 'events-timeline', params], () => fleetEvents(params));
  return {
    rows: q.data?.events ?? [],
    isPending: q.isPending && !q.data,
    error: q.error,
    isFetching: q.isFetching,
    lastUpdatedAt: q.dataUpdatedAt,
    refetch: q.refetch,
  };
}
```

> Note the `['fleet', 'events-timeline', ...]` key is deliberately
> distinct from the alert queue's `['fleet', 'events', ...]` so the two
> views don't share a cache entry (different filters, different polling).

- [ ] **Step 5: Verify type-check + lint.**

Run: `cd web && npx tsc -b --noEmit && npm run lint`
Expected: clean.

- [ ] **Step 6: Commit.**

```bash
git add web/src/hooks/useFleetQuery.ts web/src/hooks/useFleetRisk.ts web/src/hooks/useFleetCompliance.ts web/src/hooks/useFleetEvents.ts
git commit -m "$(cat <<'EOF'
feat(web): fleet query hooks (risk / compliance / events) at 20s poll

Plan 03 T6. useFleetQuery base applies the 20s fleet poll interval; the
three hooks wrap it. useFleetRisk takes min_bucket+tool, useFleetEvents is
the fleet-wide timeline (distinct query key from the alert queue so caches
don't collide), useFleetCompliance is param-free.

Co-Authored-By: Claude Opus 4.7 (1M context) <noreply@anthropic.com>
EOF
)"
```

---

## Task 7: Fleet tab shell + routing + TopNav activation

**Files:**
- ADD `web/src/routes/_authed/fleet/route.tsx`
- ADD `web/src/routes/_authed/fleet/index.tsx`
- ADD `web/src/components/Fleet/FleetTabs.tsx`
- MODIFY `web/src/components/Layout/TopNav.tsx` (activate Fleet link)

This task stands up the shell with a placeholder Risk body so navigation
works end-to-end; Tasks 8–10 fill in the three tab bodies.

- [ ] **Step 1: Create the tab bar `web/src/components/Fleet/FleetTabs.tsx`.**
  Route-based tabs: three `<Link>`s styled like a tab bar, active styling
  from TanStack Router's `activeProps`.

```tsx
import { Link } from '@tanstack/react-router';

const TABS = [
  { to: '/fleet/risk', label: 'Risk' },
  { to: '/fleet/events', label: 'Events' },
  { to: '/fleet/compliance', label: 'Compliance' },
] as const;

/** Route-based tab bar for the Fleet section (UI/UX §5.2). */
export function FleetTabs() {
  return (
    <nav className="flex items-center gap-1 border-b border-border-subtle">
      {TABS.map((t) => (
        <Link
          key={t.to}
          to={t.to}
          activeProps={{ className: 'text-text-primary border-accent' }}
          inactiveProps={{ className: 'text-text-muted border-transparent hover:text-text-primary' }}
          className="border-b-2 px-3 py-2 text-sm transition-colors -mb-px"
        >
          {t.label}
        </Link>
      ))}
    </nav>
  );
}
```

- [ ] **Step 2: Create the layout route
  `web/src/routes/_authed/fleet/route.tsx`.**

```tsx
import { createFileRoute, Outlet } from '@tanstack/react-router';
import { FleetTabs } from '@/components/Fleet/FleetTabs';

export const Route = createFileRoute('/_authed/fleet')({
  component: FleetLayout,
});

function FleetLayout() {
  return (
    <div className="flex flex-col py-4">
      <h1 className="mb-3 text-lg font-semibold text-text-primary">Fleet</h1>
      <FleetTabs />
      <div className="pt-4">
        <Outlet />
      </div>
    </div>
  );
}
```

- [ ] **Step 3: Create the index redirect
  `web/src/routes/_authed/fleet/index.tsx`.** Bare `/fleet` → `/fleet/risk`.

```tsx
import { createFileRoute, redirect } from '@tanstack/react-router';

export const Route = createFileRoute('/_authed/fleet/')({
  beforeLoad: () => {
    throw redirect({ to: '/fleet/risk' });
  },
});
```

- [ ] **Step 4: Create a temporary `risk.tsx` placeholder so the shell
  renders.** (Task 8 replaces the body.) Create
  `web/src/routes/_authed/fleet/risk.tsx`:

```tsx
import { createFileRoute } from '@tanstack/react-router';

export const Route = createFileRoute('/_authed/fleet/risk')({
  component: () => <div className="text-sm text-text-muted">Risk tab — coming in T8.</div>,
});
```

- [ ] **Step 5: Activate the Fleet link in
  `web/src/components/Layout/TopNav.tsx`.** Replace the
  `<NavStub label="Fleet" hint="Plan 03" />` line with a real `<Link>`
  mirroring the Alerts link (it should show active for any `/fleet/*`
  path — TanStack Router's `activeProps` matches path prefixes for nested
  routes by default):

```tsx
          <Link
            to="/fleet/risk"
            activeProps={{ className: 'text-text-primary bg-bg-elevated' }}
            inactiveProps={{ className: 'text-text-muted hover:text-text-primary' }}
            className="rounded px-2.5 py-1 transition-colors"
          >
            Fleet
          </Link>
```

  Keep the `Settings` `NavStub`. If `NavStub` becomes unused, leave it (it
  still renders Settings).

  > **Stale state (design §9):** the `ConnectionBanner` mounted in the
  > `_authed` layout (Plan 02) already surfaces upstream
  > stale/disconnected state globally — it polls the same `/fleet/healthz`
  > query. The Fleet tabs inherit it via the layout, so no per-tab stale
  > widget is added in this plan. Per-tab "Updated Ns ago" freshness is
  > deferred to Plan 06 polish.

- [ ] **Step 6: Boot the dev server and verify navigation by hand.**

Run (two terminals): `MOCK_FLEET=1 make dev-go` and `make dev-web`, then
open `http://localhost:5173`, log in (admin / the password whose bcrypt
hash is in your `.env`).
Expected: the `Fleet` nav item is now active; clicking it lands on
`/fleet/risk` with the tab bar (Risk active) and the placeholder text;
clicking `/fleet` in the address bar redirects to `/fleet/risk`. Clicking
Events/Compliance 404s for now (routes added in T9/T10) — that's expected.

- [ ] **Step 7: Type-check + lint.**

Run: `cd web && npx tsc -b --noEmit && npm run lint`
Expected: clean.

- [ ] **Step 8: Commit.**

```bash
git add web/src/routes/_authed/fleet/route.tsx web/src/routes/_authed/fleet/index.tsx web/src/routes/_authed/fleet/risk.tsx web/src/components/Fleet/FleetTabs.tsx web/src/components/Layout/TopNav.tsx
git commit -m "$(cat <<'EOF'
feat(web): fleet tab shell + routing + activate Fleet nav

Plan 03 T7. Nested _authed/fleet/ layout renders a route-based tab bar
(Risk/Events/Compliance as <Link>s) + <Outlet/>; /fleet redirects to
/fleet/risk. TopNav's Fleet stub becomes a real link, active on any
/fleet/* path. Risk body is a placeholder filled in T8.

Co-Authored-By: Claude Opus 4.7 (1M context) <noreply@anthropic.com>
EOF
)"
```

---

## Task 8: Risk tab

**Files:**
- MODIFY `web/src/routes/_authed/fleet/risk.tsx` (real body + search)
- ADD `web/src/components/Fleet/RiskTable.tsx`

Host list sorted by score desc. Columns: risk bar (bucket-colored),
hostname (plain text), score, top_tool, reasons_count, "Warn 24h" with the
issue-#21 caveat tooltip. Filters: `minBucket`, `tool` (in URL).

- [ ] **Step 1: Create `web/src/components/Fleet/RiskTable.tsx`.**

```tsx
import type { RiskRow } from '@/api/fleet';
import { humanTool } from '@/lib/labels';
import { cn } from '@/lib/utils';

interface Props {
  rows: RiskRow[];
  isPending: boolean;
}

/** Risk tab table (UI/UX §5.2). Hostnames are plain text — Plan 04 links them. */
export function RiskTable({ rows, isPending }: Props) {
  if (isPending) {
    return <SkeletonRows />;
  }
  if (rows.length === 0) {
    return (
      <div className="px-4 py-8 text-center text-sm text-text-muted">
        No hosts above the selected risk level.
      </div>
    );
  }
  return (
    <table className="w-full text-xs">
      <thead className="text-text-subtle">
        <tr className="border-b border-border-subtle text-left">
          <th className="px-3 py-2 font-medium">Risk</th>
          <th className="px-3 py-2 font-medium">Host</th>
          <th className="px-3 py-2 font-medium">Score</th>
          <th className="px-3 py-2 font-medium">Top tool</th>
          <th className="px-3 py-2 font-medium">Reasons</th>
          <th className="px-3 py-2 font-medium">
            <span title="Trailing-24h warn events; not alert-definition filtered (issue #21).">
              Warn 24h
            </span>
          </th>
        </tr>
      </thead>
      <tbody>
        {rows.map((row) => (
          <tr key={row.host_id} className="border-b border-border-subtle">
            <td className="px-3 py-2">
              <RiskBar bucket={row.bucket} score={row.score} />
            </td>
            <td className="px-3 py-2 font-mono text-text-primary" title={row.host_id}>
              {row.hostname ?? row.host_id.split('-')[0]}
            </td>
            <td className="px-3 py-2 font-mono">{row.score.toFixed(1)}</td>
            <td className="px-3 py-2 text-text-muted">{humanTool(row.top_tool)}</td>
            <td className="px-3 py-2 text-text-muted">{row.reasons_count}</td>
            <td className="px-3 py-2 text-text-muted">{row.open_alert_count_24h}</td>
          </tr>
        ))}
      </tbody>
    </table>
  );
}

function RiskBar({ bucket, score }: { bucket: string; score: number }) {
  const color = bucketBarColor(bucket);
  const pct = Math.max(0, Math.min(100, (score / 10) * 100));
  return (
    <div className="flex items-center gap-2">
      <div className="h-1.5 w-20 rounded bg-bg-elevated">
        <div className={cn('h-full rounded', color)} style={{ width: `${pct}%` }} />
      </div>
      <span className="uppercase tracking-wide text-[10px] text-text-subtle">{bucket}</span>
    </div>
  );
}

function bucketBarColor(bucket: string): string {
  switch (bucket) {
    case 'critical':
      return 'bg-sev-critical';
    case 'high':
      return 'bg-sev-high';
    case 'medium':
      return 'bg-sev-medium';
    case 'low':
      return 'bg-sev-low';
    default:
      return 'bg-sev-info';
  }
}

function SkeletonRows() {
  return (
    <div aria-hidden className="space-y-2 px-3 py-3">
      {Array.from({ length: 5 }).map((_, i) => (
        // biome-ignore lint/suspicious/noArrayIndexKey: static skeleton placeholders
        <div key={i} className="h-3 w-full animate-pulse rounded bg-bg-elevated" />
      ))}
    </div>
  );
}
```

- [ ] **Step 2: Replace the placeholder `web/src/routes/_authed/fleet/risk.tsx`
  with the real route.** Mirrors the all-optional `validateSearch` idiom
  from `_authed/alerts.tsx`.

```tsx
import { createFileRoute, useNavigate, useSearch } from '@tanstack/react-router';
import { RiskTable } from '@/components/Fleet/RiskTable';
import { DEFAULT_RISK_FILTER, type RiskFilter, useFleetRisk } from '@/hooks/useFleetRisk';

interface RiskSearch {
  minBucket?: RiskFilter['minBucket'];
  tool?: string[];
}

const VALID_BUCKETS: RiskFilter['minBucket'][] = ['low', 'medium', 'high', 'critical'];

export const Route = createFileRoute('/_authed/fleet/risk')({
  validateSearch: (raw: Record<string, unknown>): RiskSearch => {
    const out: RiskSearch = {};
    if (typeof raw.minBucket === 'string' && (VALID_BUCKETS as string[]).includes(raw.minBucket)) {
      out.minBucket = raw.minBucket as RiskFilter['minBucket'];
    }
    const tool = Array.isArray(raw.tool)
      ? raw.tool.filter((t): t is string => typeof t === 'string')
      : typeof raw.tool === 'string'
        ? raw.tool.split(',').filter(Boolean)
        : [];
    if (tool.length) out.tool = tool;
    return out;
  },
  component: RiskTab,
});

function RiskTab() {
  const search = useSearch({ from: '/_authed/fleet/risk' });
  const navigate = useNavigate();

  const filter: RiskFilter = {
    minBucket: search.minBucket ?? DEFAULT_RISK_FILTER.minBucket,
    tool: search.tool ?? DEFAULT_RISK_FILTER.tool,
  };
  const { rows, isPending, error } = useFleetRisk(filter);

  const setBucket = (minBucket: RiskFilter['minBucket']) =>
    navigate({ to: '/fleet/risk', search: { ...search, minBucket }, replace: true });

  return (
    <div>
      <div className="mb-3 flex items-center gap-1.5">
        {VALID_BUCKETS.map((b) => (
          <button
            key={b}
            type="button"
            onClick={() => setBucket(b)}
            className={
              filter.minBucket === b
                ? 'rounded border border-accent/40 bg-accent/10 px-2 py-0.5 text-xs text-accent'
                : 'rounded border border-border px-2 py-0.5 text-xs text-text-muted hover:text-text-primary'
            }
          >
            {b}
          </button>
        ))}
      </div>
      <div className="overflow-hidden rounded-md border border-border bg-bg-surface">
        {error ? (
          <div className="px-4 py-6 text-sm text-sev-critical">
            Failed to load risk: {error.message}
          </div>
        ) : (
          <RiskTable rows={rows} isPending={isPending} />
        )}
      </div>
    </div>
  );
}
```

- [ ] **Step 3: Boot dev + verify by hand.**

Run: `MOCK_FLEET=1 make dev-go` + `make dev-web`, open `/fleet/risk`.
Expected: rows render sorted by score desc; the bucket chips re-filter
(`low` shows all, `critical` narrows); hovering "Warn 24h" shows the
issue-#21 tooltip; hostnames are plain text (not links).

- [ ] **Step 4: Type-check + lint.**

Run: `cd web && npx tsc -b --noEmit && npm run lint`
Expected: clean.

- [ ] **Step 5: Commit.**

```bash
git add web/src/routes/_authed/fleet/risk.tsx web/src/components/Fleet/RiskTable.tsx
git commit -m "$(cat <<'EOF'
feat(web): fleet Risk tab

Plan 03 T8. RiskTable renders hosts sorted by score desc with a
bucket-colored risk bar, plain-text hostname (Plan 04 links it), score,
top_tool, reasons_count, and the "Warn 24h" column with the issue-#21
caveat tooltip. min_bucket chips filter via the URL (all-optional search,
same idiom as the alerts route).

Co-Authored-By: Claude Opus 4.7 (1M context) <noreply@anthropic.com>
EOF
)"
```

---

## Task 9: Compliance tab

**Files:**
- ADD `web/src/routes/_authed/fleet/compliance.tsx`
- ADD `web/src/components/Fleet/ComplianceTable.tsx`

Per-host raw signals + the derived status pill.

- [ ] **Step 1: Create `web/src/components/Fleet/ComplianceTable.tsx`.**

```tsx
import { formatDistanceToNowStrict } from 'date-fns';
import type { ComplianceRow } from '@/api/fleet';
import { COMPLIANCE_META, deriveComplianceStatus } from '@/lib/compliance';
import { cn } from '@/lib/utils';

interface Props {
  rows: ComplianceRow[];
  isPending: boolean;
}

export function ComplianceTable({ rows, isPending }: Props) {
  if (isPending) {
    return (
      <div aria-hidden className="space-y-2 px-3 py-3">
        {Array.from({ length: 5 }).map((_, i) => (
          // biome-ignore lint/suspicious/noArrayIndexKey: static skeleton placeholders
          <div key={i} className="h-3 w-full animate-pulse rounded bg-bg-elevated" />
        ))}
      </div>
    );
  }
  if (rows.length === 0) {
    return (
      <div className="px-4 py-8 text-center text-sm text-text-muted">
        No hosts reporting policy state yet.
      </div>
    );
  }
  return (
    <table className="w-full text-xs">
      <thead className="text-text-subtle">
        <tr className="border-b border-border-subtle text-left">
          <th className="px-3 py-2 font-medium">Host</th>
          <th className="px-3 py-2 font-medium">Status</th>
          <th className="px-3 py-2 font-medium">Policy version</th>
          <th className="px-3 py-2 font-medium">Sig failures 24h</th>
          <th className="px-3 py-2 font-medium">Last reload</th>
        </tr>
      </thead>
      <tbody>
        {rows.map((row) => {
          const status = deriveComplianceStatus(row);
          const meta = COMPLIANCE_META[status];
          return (
            <tr key={row.host_id} className="border-b border-border-subtle">
              <td className="px-3 py-2 font-mono text-text-primary" title={row.host_id}>
                {row.hostname ?? row.host_id.split('-')[0]}
              </td>
              <td className="px-3 py-2">
                <StatusPill tone={meta.tone} label={meta.label} />
              </td>
              <td className="px-3 py-2 font-mono text-text-muted">
                {row.last_applied_policy_version}
                {row.version_drift > 0 && (
                  <span className="text-status-degraded"> → {row.server_current_policy_version}</span>
                )}
              </td>
              <td className="px-3 py-2 font-mono text-text-muted">{row.signature_failures_24h}</td>
              <td className="px-3 py-2 text-text-muted">
                {row.last_policy_reload_ts
                  ? `${formatDistanceToNowStrict(new Date(row.last_policy_reload_ts))} ago`
                  : '—'}
              </td>
            </tr>
          );
        })}
      </tbody>
    </table>
  );
}

function StatusPill({ tone, label }: { tone: 'healthy' | 'degraded' | 'down'; label: string }) {
  const toneClass =
    tone === 'healthy'
      ? 'text-status-healthy border-status-healthy/40 bg-status-healthy/10'
      : tone === 'degraded'
        ? 'text-status-degraded border-status-degraded/40 bg-status-degraded/10'
        : 'text-status-down border-status-down/40 bg-status-down/10';
  return (
    <span
      className={cn(
        'inline-block rounded border px-1.5 py-px text-[10px] uppercase tracking-wide',
        toneClass,
      )}
    >
      {label}
    </span>
  );
}
```

- [ ] **Step 2: Create `web/src/routes/_authed/fleet/compliance.tsx`.**

```tsx
import { createFileRoute } from '@tanstack/react-router';
import { ComplianceTable } from '@/components/Fleet/ComplianceTable';
import { useFleetCompliance } from '@/hooks/useFleetCompliance';

export const Route = createFileRoute('/_authed/fleet/compliance')({
  component: ComplianceTab,
});

function ComplianceTab() {
  const { rows, isPending, error } = useFleetCompliance();
  return (
    <div className="overflow-hidden rounded-md border border-border bg-bg-surface">
      {error ? (
        <div className="px-4 py-6 text-sm text-sev-critical">
          Failed to load compliance: {error.message}
        </div>
      ) : (
        <ComplianceTable rows={rows} isPending={isPending} />
      )}
    </div>
  );
}
```

- [ ] **Step 3: Boot dev + verify by hand.**

Run dev, open `/fleet/compliance`.
Expected: 5 rows; status pills show In sync (alice/eve), Drift (bob),
Failing signature (carol — has both drift and 3 sig failures, so the
priority rule shows "Failing signature"), Expired (dave). The drift column
shows `applied → current` only when drift > 0.

- [ ] **Step 4: Type-check + lint.**

Run: `cd web && npx tsc -b --noEmit && npm run lint`
Expected: clean.

- [ ] **Step 5: Commit.**

```bash
git add web/src/routes/_authed/fleet/compliance.tsx web/src/components/Fleet/ComplianceTable.tsx
git commit -m "$(cat <<'EOF'
feat(web): fleet Compliance tab

Plan 03 T9. ComplianceTable renders per-host raw signals + the derived
status pill (deriveComplianceStatus). Drift shown as applied → current
when nonzero; signature failures + last reload surfaced. Plain-text
hostnames (Plan 04 links them).

Co-Authored-By: Claude Opus 4.7 (1M context) <noreply@anthropic.com>
EOF
)"
```

---

## Task 10: Events tab (fleet-wide timeline)

**Files:**
- ADD `web/src/routes/_authed/fleet/events.tsx`
- ADD `web/src/components/Fleet/EventsTable.tsx`

A **new** table distinct from the Alerts `QueueTable` — no triage columns;
columns are severity dot · age · kind · host · tool (when AI-guard) ·
summary. Filters: evidence-kind, host, since (in URL).

- [ ] **Step 1: Create `web/src/components/Fleet/EventsTable.tsx`.**

```tsx
import { formatDistanceToNowStrict } from 'date-fns';
import { type EventWithTriage, extractAiGuard } from '@/api/fleet';
import { humanTool } from '@/lib/labels';
import { cn } from '@/lib/utils';

interface Props {
  rows: EventWithTriage[];
  isPending: boolean;
}

/** Fleet-wide event timeline (UI/UX §5.2 Events tab). No triage columns. */
export function EventsTable({ rows, isPending }: Props) {
  if (isPending) {
    return (
      <div aria-hidden className="space-y-2 px-3 py-3">
        {Array.from({ length: 6 }).map((_, i) => (
          // biome-ignore lint/suspicious/noArrayIndexKey: static skeleton placeholders
          <div key={i} className="h-3 w-full animate-pulse rounded bg-bg-elevated" />
        ))}
      </div>
    );
  }
  if (rows.length === 0) {
    return (
      <div className="px-4 py-8 text-center text-sm text-text-muted">
        No events in the selected range.
      </div>
    );
  }
  return (
    <table className="w-full text-xs">
      <thead className="text-text-subtle">
        <tr className="border-b border-border-subtle text-left">
          <th className="px-3 py-2 font-medium">Sev</th>
          <th className="px-3 py-2 font-medium">Age</th>
          <th className="px-3 py-2 font-medium">Kind</th>
          <th className="px-3 py-2 font-medium">Host</th>
          <th className="px-3 py-2 font-medium">Tool</th>
        </tr>
      </thead>
      <tbody>
        {rows.map((ev) => {
          const ag = extractAiGuard(ev);
          return (
            <tr key={ev.event_id} className="border-b border-border-subtle">
              <td className="px-3 py-2">
                <span
                  className={cn(
                    'inline-block h-2 w-2 rounded-full',
                    ev.severity === 'warn' ? 'bg-sev-medium' : 'bg-sev-info',
                  )}
                />
              </td>
              <td className="px-3 py-2 font-mono text-text-muted">{relAge(ev.ts)}</td>
              <td className="px-3 py-2 text-text-primary">{humanKind(ev.evidence?.kind ?? '')}</td>
              <td className="px-3 py-2 font-mono text-text-muted" title={ev.host_id}>
                {ev.host_id.split('-')[0]}
              </td>
              <td className="px-3 py-2 text-text-muted">{ag ? humanTool(ag.tool) : '—'}</td>
            </tr>
          );
        })}
      </tbody>
    </table>
  );
}

function relAge(ts: string): string {
  try {
    return formatDistanceToNowStrict(new Date(ts));
  } catch {
    return '—';
  }
}

function humanKind(kind: string): string {
  return kind
    .split('_')
    .map((s) => (s.length ? s[0].toUpperCase() + s.slice(1) : s))
    .join(' ');
}
```

- [ ] **Step 2: Create `web/src/routes/_authed/fleet/events.tsx`.**

```tsx
import { createFileRoute, useNavigate, useSearch } from '@tanstack/react-router';
import { EventsTable } from '@/components/Fleet/EventsTable';
import {
  DEFAULT_FLEET_EVENTS_FILTER,
  type FleetEventsFilter,
  useFleetEvents,
} from '@/hooks/useFleetEvents';

interface EventsSearch {
  kind?: string[];
  host?: string[];
  since?: string;
}

export const Route = createFileRoute('/_authed/fleet/events')({
  validateSearch: (raw: Record<string, unknown>): EventsSearch => {
    const out: EventsSearch = {};
    const arr = (v: unknown): string[] =>
      Array.isArray(v)
        ? v.filter((x): x is string => typeof x === 'string')
        : typeof v === 'string'
          ? v.split(',').filter(Boolean)
          : [];
    const kind = arr(raw.kind);
    const host = arr(raw.host);
    if (kind.length) out.kind = kind;
    if (host.length) out.host = host;
    if (typeof raw.since === 'string' && raw.since.length > 0) out.since = raw.since;
    return out;
  },
  component: EventsTab,
});

function EventsTab() {
  const search = useSearch({ from: '/_authed/fleet/events' });
  const navigate = useNavigate();

  const filter: FleetEventsFilter = {
    evidenceKinds: search.kind ?? DEFAULT_FLEET_EVENTS_FILTER.evidenceKinds,
    hostIDs: search.host ?? DEFAULT_FLEET_EVENTS_FILTER.hostIDs,
    since: search.since ?? DEFAULT_FLEET_EVENTS_FILTER.since,
  };
  const { rows, isPending, error } = useFleetEvents(filter);

  const setKind = (kind: string | null) =>
    navigate({
      to: '/fleet/events',
      search: { ...search, kind: kind ? [kind] : undefined },
      replace: true,
    });

  return (
    <div>
      <div className="mb-3 flex items-center gap-1.5">
        <FilterChip active={filter.evidenceKinds.length === 0} onClick={() => setKind(null)}>
          All kinds
        </FilterChip>
        <FilterChip
          active={filter.evidenceKinds.includes('ai_guard_risk_assessed')}
          onClick={() => setKind('ai_guard_risk_assessed')}
        >
          AI Guard
        </FilterChip>
      </div>
      <div className="overflow-hidden rounded-md border border-border bg-bg-surface">
        {error ? (
          <div className="px-4 py-6 text-sm text-sev-critical">
            Failed to load events: {error.message}
          </div>
        ) : (
          <EventsTable rows={rows} isPending={isPending} />
        )}
      </div>
    </div>
  );
}

function FilterChip({
  active,
  onClick,
  children,
}: {
  active: boolean;
  onClick: () => void;
  children: React.ReactNode;
}) {
  return (
    <button
      type="button"
      onClick={onClick}
      className={
        active
          ? 'rounded border border-accent/40 bg-accent/10 px-2 py-0.5 text-xs text-accent'
          : 'rounded border border-border px-2 py-0.5 text-xs text-text-muted hover:text-text-primary'
      }
    >
      {children}
    </button>
  );
}
```

- [ ] **Step 3: Boot dev + verify by hand.**

Run dev, open `/fleet/events`.
Expected: a fleet-wide timeline of mixed-kind events (heartbeats,
ai_guard, host_meta, policy, etc.) across all hosts; the "AI Guard" chip
narrows to ai_guard_risk_assessed only; the "All kinds" chip restores.

- [ ] **Step 4: Type-check + lint.**

Run: `cd web && npx tsc -b --noEmit && npm run lint`
Expected: clean.

- [ ] **Step 5: Commit.**

```bash
git add web/src/routes/_authed/fleet/events.tsx web/src/components/Fleet/EventsTable.tsx
git commit -m "$(cat <<'EOF'
feat(web): fleet Events tab (fleet-wide timeline)

Plan 03 T10. EventsTable is a new component distinct from the triage-centric
Alerts QueueTable — columns are sev dot / age / kind / host / tool, no
ack/assignee. evidence-kind chips filter via the URL. Reuses the existing
fleetEvents fetch (no bucket filter = full timeline).

Co-Authored-By: Claude Opus 4.7 (1M context) <noreply@anthropic.com>
EOF
)"
```

---

## Task 11: E2E specs

**Files:**
- ADD `web/tests/e2e/fleet.spec.ts`

Cover navigation + tab switching + per-tab rendering against the
`MOCK_FLEET=1` binary. Reuse the `login()` helper pattern from
`web/tests/e2e/alerts.spec.ts`.

- [ ] **Step 1: Create `web/tests/e2e/fleet.spec.ts`.**

```ts
import { type Page, expect, test } from '@playwright/test';

const ADMIN = 'admin';
const PASSWORD = 'test-password';

async function login(page: Page) {
  await page.goto('/login');
  await page.getByLabel('Username').fill(ADMIN);
  await page.getByLabel('Password').fill(PASSWORD);
  await page.getByRole('button', { name: /^Sign in$/ }).click();
  await expect(page).toHaveURL(/\/alerts/);
}

test.describe('fleet pages', () => {
  test.beforeEach(async ({ context }) => {
    await context.clearCookies();
  });

  test('Fleet nav lands on /fleet/risk via redirect', async ({ page }) => {
    await login(page);
    await page.getByRole('link', { name: /^Fleet$/ }).click();
    await expect(page).toHaveURL(/\/fleet\/risk/);
    // Risk table paints rows from the mock.
    await expect(page.getByRole('cell', { name: /alice|bob|carol|eve/ }).first()).toBeVisible({
      timeout: 5_000,
    });
  });

  test('bare /fleet redirects to /fleet/risk', async ({ page }) => {
    await login(page);
    await page.goto('/fleet');
    await expect(page).toHaveURL(/\/fleet\/risk/);
  });

  test('tab switching changes URL and content', async ({ page }) => {
    await login(page);
    await page.goto('/fleet/risk');

    await page.getByRole('link', { name: /^Compliance$/ }).click();
    await expect(page).toHaveURL(/\/fleet\/compliance/);
    // Compliance derives pills across the mock's mixed hosts.
    await expect(page.getByText(/^In sync$/i).first()).toBeVisible({ timeout: 5_000 });
    await expect(page.getByText(/^Expired$/i).first()).toBeVisible();
    await expect(page.getByText(/^Failing signature$/i).first()).toBeVisible();

    await page.getByRole('link', { name: /^Events$/ }).click();
    await expect(page).toHaveURL(/\/fleet\/events/);
  });

  test('risk min_bucket chip narrows the row set', async ({ page }) => {
    await login(page);
    await page.goto('/fleet/risk');
    await expect(page.locator('tbody tr').first()).toBeVisible({ timeout: 5_000 });
    const before = await page.locator('tbody tr').count();

    await page.getByRole('button', { name: /^critical$/ }).click();
    await page.waitForTimeout(500);
    const after = await page.locator('tbody tr').count();
    expect(after).toBeLessThanOrEqual(before);
  });

  test('events tab AI Guard chip narrows the timeline', async ({ page }) => {
    await login(page);
    await page.goto('/fleet/events');
    await expect(page.locator('tbody tr').first()).toBeVisible({ timeout: 5_000 });
    const before = await page.locator('tbody tr').count();

    await page.getByRole('button', { name: /^AI Guard$/ }).click();
    await page.waitForTimeout(500);
    const after = await page.locator('tbody tr').count();
    expect(after).toBeLessThanOrEqual(before);
  });
});
```

- [ ] **Step 2: Build + run the full e2e suite.**

Run (from repo root): `make e2e`
Expected: the existing 11 specs + the 5 new fleet specs all pass.

> If a selector is ambiguous (strict-mode violation because a string also
> appears in another column), scope it with `.first()` or a more specific
> role/locator — same pattern used in `alerts.spec.ts`.

- [ ] **Step 3: Commit.**

```bash
git add web/tests/e2e/fleet.spec.ts
git commit -m "$(cat <<'EOF'
test(e2e): fleet pages — nav, redirect, tab switching, filters

Plan 03 T11. 5 specs against the MOCK_FLEET binary: Fleet nav → /fleet/risk,
bare /fleet redirect, tab switching changes URL + content (compliance pills
derive In sync / Expired / Failing signature), risk min_bucket chip narrows
rows, events AI-Guard chip narrows the timeline.

Co-Authored-By: Claude Opus 4.7 (1M context) <noreply@anthropic.com>
EOF
)"
```

---

## Task 12: CI / README / ship checklist

**Files:**
- MODIFY `README.md` (add Fleet pages to the "What's in" summary)
- (CI needs no change — `make e2e` already runs all specs incl. fleet)

- [ ] **Step 1:** Confirm CI needs no edit. The `e2e` job runs
  `npx playwright test`, which discovers `fleet.spec.ts` automatically.
  Re-read `.github/workflows/ci.yml` to confirm there's no per-spec
  allowlist (there isn't — it runs the whole suite).

- [ ] **Step 2:** Update the README "What's in Plan 02" section to add a
  short "Plan 03" note. In `README.md`, after the Plan 02 paragraph, add:

```markdown
## What's in Plan 03

The Fleet section: `/fleet/risk` (hosts sorted by AI Guard risk),
`/fleet/events` (fleet-wide event timeline), and `/fleet/compliance`
(per-host policy state with a client-derived status pill). Read-only,
20s polling. Host detail (`/hosts/:hostname`) lands in Plan 04.
```

- [ ] **Step 3:** Run the full local gate.

Run: `make test && make lint && make build && make e2e`
Expected: all green — Go tests, Vitest (now includes labels + compliance),
golangci-lint, Biome, build, and 16 Playwright specs.

- [ ] **Step 4:** Commit.

```bash
git add README.md
git commit -m "$(cat <<'EOF'
docs(readme): add Plan 03 Fleet pages summary

Plan 03 T12. README notes the three Fleet tabs (risk / events / compliance),
read-only + 20s polling, with host detail deferred to Plan 04. CI unchanged
— the e2e job already runs the whole Playwright suite.

Co-Authored-By: Claude Opus 4.7 (1M context) <noreply@anthropic.com>
EOF
)"
```

- [ ] **Step 5:** Push the branch and open a PR (mirror the Plan 02 / PR #4
  workflow). Refresh the cross-repo issue tracker per `CLAUDE.md` if a
  Plan 03 epic issue exists.

```bash
git push -u origin feat/plan-03-fleet-pages
gh pr create --repo Ju571nK/sigil-manager --base main \
  --title "Plan 03: Fleet pages (risk / events / compliance)" \
  --body "$(cat <<'BODY'
Implements docs/superpowers/specs/2026-05-21-plan-03-fleet-pages-design.md.

## Summary
- Two /api/v1 passthroughs (fleet/risk + fleet/compliance) over the
  existing FleetClient; compliance stays raw-signals-only (F13).
- Fleet section: nested _authed/fleet/ routes — Risk (sorted by score,
  "Warn 24h" with the issue-#21 caveat), Events (fleet-wide timeline,
  new EventsTable), Compliance (client-derived status pill). 20s polling.
- Shared humanTool() extracted to lib/labels; deriveComplianceStatus in
  lib/compliance, both unit-tested.
- Mock fixture tuned so all four compliance states appear.

## Test plan
- [ ] CI green: go / web / e2e (16 Playwright specs)
- [ ] Compliance pills derive In sync / Drift / Expired / Failing signature
- [ ] Hostnames are plain text (Plan 04 links them)
BODY
)"
```

Watch CI to green before requesting merge (merge to main needs explicit
per-session authorization per CLAUDE.md).

---

## Verification (end state of Plan 03)

A developer on `feat/plan-03-fleet-pages` with `MOCK_FLEET=1 make dev` can:

1. Log in, click `Fleet` in the top nav → lands on `/fleet/risk`.
2. See hosts sorted by score desc with bucket-colored risk bars; the
   "Warn 24h" header tooltip explains the issue-#21 caveat.
3. Filter risk by `min_bucket` chips; the URL updates and survives reload.
4. Switch to `/fleet/compliance` → status pills derive correctly:
   In sync (alice/eve), Drift (bob), Failing signature (carol),
   Expired (dave).
5. Switch to `/fleet/events` → fleet-wide timeline of all event kinds;
   the AI Guard chip narrows it.
6. Hostnames are plain text everywhere (Plan 04 makes them links).

CI green. `make test`, `make lint`, `make build`, `make e2e` all pass
locally (16 Playwright specs).

---

## What's out of Plan 03 (future plans)

- **Plan 04 — Host detail:** `/hosts/:hostname` with Alerts/Events/
  Compliance tabs; hostname→host_id resolver; making fleet hostnames
  clickable.
- **Plan 05 — Settings:** real settings page.
- **Plan 06 — Polish:** light theme, responsive, a11y, broader shortcuts.

Plan 03 adds only new routes/components/hooks + two passthrough handlers;
it does not modify the auth, `FleetClient`, or triage repo APIs — those
remain the stable interfaces from Plan 02.
