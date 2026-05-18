# sigil-server Fleet Read API — Consumer Contract

- **Status:** **v1.0 — aligned with sigil producer spec 2026-05-17.** Producer has
  locked its own design (decisions A–I) and resolved all seven §10 open questions
  from the prior Draft v0. This doc now reflects the producer-side decisions
  verbatim; the consumer side (`sigil-manager`) implements against this.
- **Date:** 2026-05-16 (draft v0) · 2026-05-17 (aligned to v1.0)
- **Owner:** Justin Kwon
- **Repo of record:** `Ju571nK/sigil-manager` (consumer).
- **Cross-links:**
  - Consumer needs: `docs/superpowers/specs/2026-05-16-ui-ux-design.md`
  - **Producer spec (locked):** `Ju571nK/sigil` →
    `docs/superpowers/specs/2026-05-17-phase-3b4-fleet-aggregation-api-design.html`
  - **Producer plan:** `Ju571nK/sigil` →
    `docs/superpowers/plans/2026-05-17-phase-3b4-fleet-aggregation-api.html`
  - Producer evidence schema: `Ju571nK/sigil` → `crates/sigil-core/src/event.rs`
    (now includes `HostMetaSnapshot` shipped 2026-05-17 / merge `b34dbdd`)
  - Producer existing + new routes: `Ju571nK/sigil` →
    `crates/sigil-server/src/app.rs` (Phase 3b.4 in flight, issue **#18**, sub of #9)
  - Sigil epic: `Ju571nK/sigil` issue #9 (Phase 3b)
- **Producer lock target:** `sigil-server 0.5.0` (the release that ships 3b.4).
  When that release lands, change status to *"locked against sigil-server 0.5.0"*.

---

## 1. Why this doc exists

`sigil-manager` is the read-side consumer of `sigil-server`. Until Phase 3b.4,
the server exposed only two endpoints (`POST /v1/events` for agent ingest,
`GET /v1/policy` for agent policy fetch). Phase 3b.4 adds the read API.

This contract was originally drafted consumer-first (Draft v0, 2026-05-16) and
handed to producer as input. Producer then ran its own spec round and locked
9 decisions (A–I) plus answers to all 7 of our open questions on 2026-05-17.
**This v1.0 reflects those locked producer decisions verbatim.** Any
disagreement is now a follow-up issue against `Ju571nK/sigil`, not a unilateral
consumer-side override.

The doc still serves the consumer: it lists screens (`/alerts`, `/fleet/*`,
`/hosts/:hostname`) and traces them to the endpoints and fields the UI requires
(§9). When the producer's `sigil-server 0.5.0` ships, this doc is the
implementation contract for `sigil-manager`'s `FleetClient`.

**Not in scope of this contract:** triage state. Per UI/UX D5 and producer
Out-of-scope §8, ack/assign/resolve/notes live in `sigil-manager`'s own DB.
Producer recommends a `(host_id, event_id)` join key on the consumer side
since `event_id` alone can orphan if an event is rejected as
`EventUnprocessableLocal`; that's a sigil-manager Plan 02 detail, not this
contract.

---

## 2. Locked decisions

Producer's locked decisions A–I (sigil 3b.4 spec §1) are reflected here as
F-numbered consumer entries. The mapping column traces each row to the
producer-side decision letter so the lineage is auditable.

| # | Decision | Choice | Producer ref |
|---|---|---|---|
| F1  | Transport | HTTP/1.1 + JSON (same as existing `/v1/*`). gRPC rejected. | — (implicit) |
| F2  | Auth (read side) | **Bearer token from env var.** Server reads `SIGIL_SERVER_READ_TOKEN`; consumer sends `Authorization: Bearer <token>`. Constant-time compare. Single shared token. **Unset env ⇒ all read endpoints (except `/v1/healthz`) return `404 not_found`** — read API existence is hidden. | **C** |
| F3  | URL prefix | Flat `/v1/...`. No `/read/` subtree. | **D** |
| F4  | Ordering | Events: `ts desc` always (no `asc` in v1). Hosts: sortable via `sort=last_seen\|risk\|host_id`, default `last_seen`. | (producer §4.3, §4.7) |
| F5  | Pagination | **Cursor = opaque string carrying the last `event_id` (UUIDv7)** for `/v1/events`; for fleet endpoints, last `host_id` string. UUIDv7's lexicographic ordering = chronological. `limit` default 100, max 1000. `limit>1000` silently capped (see open issue note in §6.1). | **G** |
| F6  | Time format | RFC 3339 strings everywhere. Events carry `ts` from agent's clock; server does **not** add a `server_received_ts` in v1 (see §13 gap). | (producer §4) |
| F7  | Score scale | **0.0–10.0** float (CVSS-style, matches `AiGuardRiskAssessed.score`). **Locked — UI/UX spec §5.2 to be updated** from "0-100" to "0-10". | producer §9 Q1 |
| F8  | Streaming | Polling only in v1 (UI/UX §7.2 = 5s). SSE/WebSocket deferred (producer Out-of-scope §8 → 3b.4.1 or sigil-cloud). | producer §8 |
| F9  | Versioning | Path version (`/v1/`). Additive non-breaking (new fields, new endpoints, new evidence variants). Breaking → `/v2/`, both run ≥1 minor cycle. | (producer §7.3) |
| F10 | "Alerts" definition | Server has no first-class alert concept, but **`/v1/meta.alerts_definition_default` exports the producer's recommended set** so consumer + server agree on the default. Consumer may override client-side. **Note:** as of sigil-server `0dce160` (2026-05-17), `open_alert_count_24h` in `/v1/fleet/risk` is implemented as the trailing-24h `warn`-severity event sum and does **not** filter by `alerts_definition_default` — divergence from producer spec §4.5. Tracked in §13.1. | producer §4.2 / §4.5 |
| F11 | Server-side index backing | **In-memory per-host `HostSummary` HashMap** (`parking_lot::RwLock`). Rebuilt from JSONL on server boot. `/v1/events` filters do an **on-demand reverse JSONL scan** (no event-level index in v1). MVP target: ≤1000 hosts × 7 days × 30k events/day. | **B**, **F** |
| F12 | Index update timing | **Synchronous.** `POST /v1/events` ingest handler updates `HostSummary` inline before responding. No async indexer task in v1. | **H** |
| F13 | Compliance representation | `/v1/fleet/compliance` exposes **raw signals only**: `last_applied_policy_version`, `server_current_policy_version`, `version_drift`, `policy_expired_active`, `last_policy_reload_ts`, `signature_failures_24h`. **No `compliance_score`** — the UI derives status from raw signals. | **I**, producer §9 Q2 |
| F14 | Per-tool risk rollup | `/v1/fleet/risk` is **1-row-per-host** with `top_tool` field; per-tool drill-down is in `/v1/fleet/hosts/{host_id}.ai_guard.by_tool`. Consumer §10 Q3 ("emit one row per (host, tool)") was rejected. | producer §9 Q3 |
| F15 | Boot rebuild state | During the boot-time JSONL replay (5-10s for typical fleet), **all read endpoints except `/v1/healthz` return `503 service_unavailable` with `Retry-After: 5`**. Once rebuild completes, normal serving begins. | producer §4.1, §6 |

---

## 3. Auth

### 3.1 Token

- Server env var: `SIGIL_SERVER_READ_TOKEN` (required to enable read endpoints).
- Header: `Authorization: Bearer <token>`. Server uses **constant-time
  comparison** to dodge timing side channels.
- **Unset env** → all read endpoints (except `/v1/healthz`) return
  `404 not_found`. Read API existence is hidden, not just disabled.
- **Set env, missing/wrong header** → `401 unauthorized` with body
  `{"error":{"code":"unauthorized","message":"..."}}`.
- `/v1/healthz` is **no-auth** (liveness for ops tooling).
- Single shared token across all read endpoints in v1.

### 3.2 Token rotation

**Restart-only rotation in v1.** Changing `SIGIL_SERVER_READ_TOKEN` requires
restarting `sigil-server`. There is no graceful reload, no dual-token overlap
window. Operators rotating tokens must accept a brief read-API outage during
restart (agents continue to ingest via `POST /v1/events` over mTLS — that path
is unaffected).

`sigil-manager`'s `FleetClient` should treat any `401` as a hard
configuration error and surface it to the operator (Settings page banner),
not retry silently.

### 3.3 Threat model note

The token is a shared secret intended for **operator-deployed, self-hosted**
topologies where `sigil-manager` and `sigil-server` are typically on the same
network. For broader threat models (untrusted network paths, multiple
consumers), operators are expected to put `sigil-server` behind their own
reverse proxy + TLS. Strengthening this is `sigil-cloud`'s problem (not in this
repo per `CLAUDE.md`).

### 3.4 Why 404-not-401 when env is unset

The producer chose to hide read-API existence entirely when not configured
(rather than 401-ing with a "read API disabled" message). Trade-off:
- **Pro:** scanners can't trivially detect that this server has a read API.
- **Con:** operators may waste time debugging "404 — wrong URL?" vs.
  "404 — env var unset". `sigil-manager`'s setup flow must check the operator
  set the env var on the server side; a `404` on `/v1/meta` after correct
  base URL is a strong hint the token is unset.

---

## 4. Endpoint inventory

All endpoints are `GET` unless stated.

| Path | Purpose | Consumer screen |
|---|---|---|
| `/v1/healthz` | Liveness (no auth) | Settings connection indicator |
| `/v1/meta` | Server build, schema version, time | Settings, debug |
| `/v1/fleet/hosts` | Paged list of hosts with summary | `/fleet/risk` default tab |
| `/v1/fleet/hosts/{host_id}` | One host's current snapshot | `/hosts/:hostname` header |
| `/v1/fleet/risk` | Hosts sorted by current AI Guard risk | `/fleet/risk` |
| `/v1/fleet/compliance` | Per-host policy compliance state | `/fleet/compliance` |
| `/v1/events` | Paged events with filters | `/fleet/events`, alerts derivation |
| `/v1/events/{event_id}` | Single event detail | Alert slide-over body |
| `/v1/policy/meta` | Current server-side policy metadata | Settings, compliance derivation |

The existing `POST /v1/events` (agent ingest) and `GET /v1/policy` (agent
fetches full signed envelope) **stay as-is** — this contract is additive.

---

## 5. Response schemas

### 5.1 `GET /v1/healthz`

No auth. Static-ish.

```json
{
  "status": "ok",
  "ts": "2026-05-16T12:34:56Z"
}
```

Non-`ok` is reserved for future degraded states; v1 only emits `"ok"` or
returns non-200.

### 5.2 `GET /v1/meta`

```json
{
  "server_version": "0.5.0",
  "schema_version": 1,
  "ts": "2026-05-17T12:34:56Z",
  "alerts_definition_default": {
    "evidence_kinds": ["ai_guard_risk_assessed"],
    "ai_guard_buckets": ["high", "critical"],
    "additional_kinds": ["policy_signature_invalid", "tls_failure", "host_id_fingerprint_drift", "agent_dying", "sender_lag_critical"]
  }
}
```

`alerts_definition_default` is the **producer's recommended set**. The
consumer (`sigil-manager`) starts from this and may add/remove kinds in
client-side configuration. Two consequences flow from F10:
- `open_alert_count_24h` (in `/v1/fleet/risk` rows) is server-computed
  against this recommendation. If the consumer overrides, it recomputes from
  `/v1/events`.
- This is **not** a versioned schema field — extending the recommendation does
  not bump `schema_version`. Consumers MUST tolerate `additional_kinds` they
  do not recognize (treat as "unknown alert sources, include if my override
  rules say so").

### 5.3 `GET /v1/fleet/hosts`

**Query params:**
- `cursor` — opaque, from previous response (`next_cursor`); the server
  encodes the last `host_id` of the previous page (see F5).
- `limit` — int, default 100, max 1000.
- `status` — `healthy,stale,disconnected` (comma-list). Defaults: all.
- `bucket` — `low,medium,high,critical` filter on **max** per-tool current
  bucket. Defaults: all.
- `sort` — `last_seen|risk|host_id`. Default `last_seen`. Unrecognised
  values silently fall back to `last_seen` (not validated server-side in v1).

**Status semantics** (computed against agent-side `event.ts`; producer chose
not to use a separate server-received timestamp in v1 — see §13 gap on clock
skew):
- `healthy`: last_seen within 5 minutes.
- `stale`: last_seen 5 min – 1 hour.
- `disconnected`: last_seen > 1 hour or never connected.

**Response:**

```json
{
  "hosts": [
    {
      "host_id": "5a7c3e91-aaaa-bbbb-cccc-dddddddddddd",
      "hostname": "alice-mbp",
      "agent_version": "0.4.0",
      "last_seen_ts": "2026-05-17T12:34:01Z",
      "status": "healthy",
      "current_risk": {
        "max_score": 7.2,
        "max_bucket": "critical",
        "by_tool": {
          "claude_code": { "score": 7.2, "bucket": "critical", "assessed_ts": "2026-05-17T12:33:55Z" },
          "codex":       { "score": 2.1, "bucket": "medium",   "assessed_ts": "2026-05-17T12:30:11Z" }
        }
      },
      "open_event_counts_24h": {
        "warn": 14,
        "info": 1402
      }
    }
  ],
  "next_cursor": "5a7c3e91-...",
  "total_estimated": 47
}
```

Notes:
- `host_id` is the agent's UUIDv4 (`event.host_id`).
- **`hostname`** is sourced from the latest `Evidence::HostMetaSnapshot` for
  this host (Phase 3b.4-pre, shipped 2026-05-17 / merge `b34dbdd`).
  `hostname: null` if the host has emitted no `HostMetaSnapshot` yet.
- `current_risk` is `null` if the host has emitted no `AiGuardRiskAssessed`
  events yet.
- `by_tool` only includes tools that have been assessed. Missing key = no
  data, not "low".
- `open_event_counts_24h` counts events in the trailing 24-hour sliding
  window grouped by `severity`. `info` is exposed for context even though
  it's not in the default alert set.
- `total_estimated` is the producer's in-memory `HashMap.len()` and is
  **exact** in v1 (in-memory index per F11). The field name preserves room
  for backends that estimate.

### 5.4 `GET /v1/fleet/hosts/{host_id}`

`404 not_found` if `host_id` is not in the in-memory index. (This happens if
the host has never sent an event, or if its events have rotated out of the
JSONL retention window before any boot rebuild — see §13 retention gap.)

**Response** is `hosts[*]` from 5.3 plus four detail blocks (any of which
may be `null` if the host hasn't emitted the relevant evidence yet):

```json
{
  "host_id": "5a7c3e91-aaaa-bbbb-cccc-dddddddddddd",
  "hostname": "alice-mbp",
  "agent_version": "0.4.0",
  "last_seen_ts": "2026-05-17T12:34:01Z",
  "status": "healthy",
  "current_risk": { ... },
  "open_event_counts_24h": { ... },

  "host_meta": {
    "os_name": "macOS",
    "os_version": "14.5",
    "kernel_version": "23.5.0",
    "architecture": "arm64",
    "interfaces": [
      {
        "name": "en0",
        "mac": "00:1b:44:11:3a:b7",
        "ipv4": ["192.168.1.42/24"],
        "ipv6": ["fe80::1/64"]
      }
    ],
    "default_gateway_v4": "192.168.1.1",
    "default_gateway_v6": null,
    "dns_servers": ["1.1.1.1", "8.8.8.8"]
  },

  "policy_state": {
    "last_applied_policy_version": 17,
    "policy_expired_active": false,
    "last_policy_reload_ts": "2026-05-17T08:00:00Z"
  },

  "agent_health": {
    "recent_channel_stalls_24h": 0,
    "recent_watcher_degraded_24h": 0,
    "recent_sender_lag_critical_24h": 0,
    "last_heartbeat_ts": "2026-05-17T12:33:00Z",
    "hash_p99_ms_latest": 4,
    "jsonl_above_soft_floor_latest": false
  },

  "ai_guard": {
    "by_tool": {
      "claude_code": {
        "score": 7.2,
        "bucket": "critical",
        "assessed_ts": "2026-05-17T12:33:55Z",
        "is_reattestation": false,
        "scope": { "kind": "user_global" },
        "reasons": [
          { "kind": "destructive_in_inline_command", "pattern": "rm -rf", "hook_event": "PreToolUse", "snippet": "rm -rf /" }
        ]
      }
    }
  }
}
```

Notes:
- **`host_meta`** is the latest `Evidence::HostMetaSnapshot` payload verbatim
  (3b.4-pre wire-stable types from `sigil-core`). Producer overwrites on each
  `is_reattestation: false` snapshot.
- `policy_state.signature_failures_24h` from the original draft was removed
  and replaced with a top-level `signature_failures_24h` in
  `/v1/fleet/compliance` (see §5.6 / F13).
- `null` semantics in every block: the host hasn't yet emitted that variant.
  UI should distinguish "no data yet" from "data unavailable".

This is the host-detail page's primary fetch. UI tabs (`Alerts`/`Events`/
`Compliance`) fetch from `/v1/events` and `/v1/fleet/compliance` with
`host_id` filter applied.

### 5.5 `GET /v1/fleet/risk`

Per F14, this is **1-row-per-host** sorted by `max_score desc`. Per-tool
breakdown is in `/v1/fleet/hosts/{host_id}.ai_guard.by_tool`.

**Query params:**
- `cursor`, `limit` (same as 5.3).
- `tool` — `claude_code,codex` filter (applies to `top_tool`). Default: all.
- `min_bucket` — `low|medium|high|critical`. Hosts whose `max_bucket` is
  below this are excluded. Default: `low`.

**Response:**

```json
{
  "rows": [
    {
      "host_id": "5a7c3e91-aaaa-...",
      "hostname": "alice-mbp",
      "score": 7.2,
      "bucket": "critical",
      "top_tool": "claude_code",
      "reasons_count": 3,
      "assessed_ts": "2026-05-17T12:33:55Z",
      "open_alert_count_24h": 5
    }
  ],
  "next_cursor": null
}
```

- `hostname: null` allowed (see §5.3).
- `open_alert_count_24h` — **producer spec §4.5 says this is the count
  matching `alerts_definition_default`. As of sigil-server commit `0dce160`
  (2026-05-17), the actual implementation returns
  `h.counts_24h.sum_warn()` — i.e., every `severity=warn` event in the
  trailing 24h, regardless of whether it matches the alerts definition.**
  Consumer should treat the field as a coarse "warn events in 24h"
  indicator and recompute precise alert counts client-side from
  `/v1/events` if the UI needs a tighter number. Tracked as a follow-up on
  `Ju571nK/sigil` (see §13.1).
- Hosts with no `AiGuardRiskAssessed` yet are omitted entirely (not returned
  with `null` score).

### 5.6 `GET /v1/fleet/compliance`

Per F13, this endpoint exposes **raw signals only**. No `compliance_score`.
The UI derives a status pill (✅ In sync / ⚠️ Drift / ❌ Expired) from the
raw fields client-side.

**Query params:** `cursor`, `limit`.

**Response:**

```json
{
  "rows": [
    {
      "host_id": "5a7c3e91-aaaa-...",
      "hostname": "alice-mbp",
      "last_applied_policy_version": 17,
      "server_current_policy_version": 18,
      "version_drift": 1,
      "policy_expired_active": false,
      "last_policy_reload_ts": "2026-05-17T08:00:00Z",
      "signature_failures_24h": 0
    }
  ],
  "next_cursor": null
}
```

**UI derivation rule** (sigil-manager Plan 02 will implement):
- `In sync` ✅ when `version_drift == 0 AND !policy_expired_active AND
  signature_failures_24h == 0`
- `Expired` ❌ when `policy_expired_active`
- `Failing signature` ❌ when `signature_failures_24h > 0`
- `Drift` ⚠️ when `version_drift > 0` (and not in above states)

If the producer needs to refine the rule, the **rule lives in
`sigil-manager`** (not in the server). Producer never exposes a numeric
score.

### 5.7 `GET /v1/events`

The core paged query. Per F11, this endpoint is served by an **on-demand
reverse JSONL scan** — no event-level index in v1. JSONL files are organized
as `events_out_dir/<host_id>/received-YYYY-MM-DD.jsonl`, so date partitioning
comes for free.

**Query params:**
- `cursor` — opaque string. Server encodes the last `event_id` (UUIDv7) of
  the previous page. Walk continues with events whose `event_id` is
  **lexicographically less** than the cursor (= chronologically earlier).
- `limit` — int, default 100, max 1000.
- `host_id` — UUIDv4 string filter. Repeatable as `?host_id=a&host_id=b`.
- `since` — RFC 3339. Inclusive (agent's `event.ts`).
- `until` — RFC 3339. Exclusive (agent's `event.ts`).
- `evidence_kind` — comma-list. Snake-case match of `Evidence`'s
  `#[serde(tag = "kind")]`. Examples: `file_change`, `ai_guard_risk_assessed`,
  `host_meta_snapshot`, `policy_signature_invalid`, `agent_dying`.
- `severity` — `info,warn` (current values; see §13 severity expansion gap).
- `source` — `file_system,agent`.
- `min_ai_guard_bucket` — `low|medium|high|critical`. Only applies when
  `evidence_kind` includes `ai_guard_risk_assessed`. Lower buckets are
  filtered out.

**Ordering:** `ts desc` always (UUIDv7 lexicographic walk reverse). No `asc`
in v1.

**Cursor stability across writes:** Because the cursor names a specific
`event_id` and the walk only returns events with smaller `event_id` (older),
new events written while a consumer is paging will appear on page 1 of the
next walk, **not** somewhere in the middle of the current walk. The consumer
gets a self-consistent reverse-time snapshot per cursor walk without the
server holding any read transaction.

**Corruption handling:** A malformed JSONL line is skipped with a
`tracing::warn!` on the server and a corruption counter increment (not yet
exposed on a read endpoint — see §13 observability gap). A partial line
during file rotation behaves the same; the line will deserialize on the next
page request.

**Response:**

```json
{
  "events": [
    {
      "schema_version": 1,
      "event_id": "019e0cea-42f1-7ef3-9a6a-1721e98ee2ba",
      "ts": "2026-05-17T12:33:55Z",
      "host_id": "5a7c3e91-aaaa-...",
      "agent_version": "0.4.0",
      "severity": "warn",
      "source": { "kind": "file_system" },
      "subject": { "kind": "path", "value": "/Users/x/.claude/settings.json" },
      "evidence": {
        "kind": "ai_guard_risk_assessed",
        "tool": "claude_code",
        "scope": { "kind": "user_global" },
        "score": 7.2,
        "bucket": "critical",
        "reasons": [ ... ],
        "is_reattestation": false
      },
      "target_id": null
    }
  ],
  "next_cursor": "019e0ce8-..."
}
```

Each item is the **untouched on-the-wire `Event` JSON** from `sigil-core`.
The fleet API does not re-shape, redact, or recompose event fields. Consumers
trust the producer schema and tolerate additive evolution (new
`Evidence::*` variants, new optional fields).

**Time semantics caveat:** `ts` is the agent's clock at event emission.
Producer chose not to add a `server_received_ts` in v1 (see §13 gap on clock
skew). Operators with hosts whose clocks drift significantly should treat
`/v1/events?since=...` results as approximate near the window boundary.

### 5.8 `GET /v1/events/{event_id}`

`404 not_found` if not found.

Producer's lookup mechanism (per spec §4.8): extract the embedded timestamp
from the UUIDv7 `event_id`, identify the JSONL date partition (and adjacent
ones to cover skew at midnight boundaries), and scan those date files
across all host dirs for a matching line. **No global event_id index** —
the UUIDv7 timestamp embed is the index.

Response: the same shape as one element of `5.7.events`.

This is the alert slide-over's primary fetch. The consumer's triage row
(stored locally in `sigil-manager`'s DB) should key on
`(host_id, event_id)` — not `event_id` alone — to avoid orphans if an
event is later rejected via `EventUnprocessableLocal` or replayed.

### 5.9 `GET /v1/policy/meta`

```json
{
  "policy_version": 18,
  "signing_pubkey_id": "k1",
  "signed_at": "2026-05-15T20:00:00Z",
  "valid_until": "2026-05-22T20:00:00Z"
}
```

Lightweight companion to the existing `/v1/policy` which serves the full
signed envelope. The UI Settings page calls this; the existing `/v1/policy`
stays for agents.

---

## 6. Error model

### 6.1 Wire shape

All errors:

```json
{
  "error": {
    "code": "unauthorized | not_found | invalid_query | rate_limited | service_unavailable | internal",
    "message": "human-readable",
    "details": { /* optional, code-specific */ }
  }
}
```

| Status | `code` | When |
|---|---|---|
| `400` | `invalid_query` | Bad cursor (unparseable UUID for `/v1/events`), unknown filter value, malformed timestamp. |
| `401` | `unauthorized` | Missing/wrong bearer (env var IS set). |
| `404` | `not_found` | `{host_id}`/`{event_id}` not in set; OR read endpoints when `SIGIL_SERVER_READ_TOKEN` is unset (read API hidden). |
| `429` | `rate_limited` | Reserved; not enforced in v1. |
| `503` | `service_unavailable` | Boot rebuild in progress (per F15). Response includes `Retry-After: 5` header. |
| `500` | `internal` | Producer bug. |

### 6.2 Silent `limit` clamp

Producer **silently clamps `limit` to `[1, 1000]`** in both directions:
`limit=0`, negative values, and `limit>1000` are all coerced — no `400`
returned. Implementation: `clamp(1, 1000)` in every route's
query-parser. `sigil-manager`'s `FleetClient` should always send a value
inside the range so client and server agree on the page size. A
`limit=5000` request that returns 1000 rows is otherwise indistinguishable
from a genuine 1000-row page; similarly `limit=0` returning a non-empty
page is silently rounded up to 1.

---

## 7. Pagination semantics

- **Cursor is opaque to the consumer.** Even though producer's v1 encodes
  the last `event_id` (for `/v1/events`) or last `host_id` (for fleet
  endpoints), the consumer MUST NOT parse the cursor — producer reserves the
  right to change the encoding within v1.
- **Cursor stability across writes** (per F5/F11):
  - `/v1/events` walks events with `event_id < cursor` (UUIDv7 lexicographic).
    New events arriving mid-walk have larger `event_id`s and are therefore
    invisible to the current walk. They appear at page 1 of the next walk.
  - `/v1/fleet/*` snapshots are read against a `parking_lot::RwLock` read
    lock — internally consistent for a single response, but a multi-page
    walk does NOT hold the lock across requests. New hosts joining
    mid-walk may shift cursor positions.
- **`next_cursor: null`** means no more pages.
- **`limit`** → silently clamped to `[1, 1000]` (see §6.2); no `400`.
- **Stale cursor** (e.g., the `host_id` referenced by a `/v1/fleet/*`
  cursor no longer appears in the current snapshot, or a `/v1/events`
  cursor's UUID is older than the JSONL retention window) → server returns
  an empty page with `next_cursor: null`. No `400`, no error. Consumer
  detects this as "0 rows + null cursor" and either restarts the walk from
  the top or treats the prior walk as complete.

---

## 8. Versioning & schema evolution

- Adding new optional response fields is non-breaking.
- Adding new `Evidence` variants is non-breaking (already wire-stable per sigil
  spec §3.3). `sigil-manager` must tolerate unknown `evidence.kind`.
- Adding new endpoints under `/v1/` is non-breaking.
- Removing fields, renaming fields, changing types, removing endpoints →
  bump to `/v2/` and run both for at least one minor release cycle.

`/v1/meta.schema_version` lets consumers detect when the event schema has
been bumped.

---

## 9. Mapping: UI screens → endpoints

| UI surface | Endpoint(s) called |
|---|---|
| `/alerts` queue | `/v1/events?evidence_kind=ai_guard_risk_assessed&min_ai_guard_bucket=high&since=...` plus the additional kinds from `meta.alerts_definition_default`. Polled every 5s (UI/UX §7.2). |
| Alert slide-over | `/v1/events/{event_id}` for the body. Triage state read/written to local sigil-manager DB. |
| `/fleet/risk` (default tab) | `/v1/fleet/risk?limit=100`. Click row → `/hosts/:hostname`. |
| `/fleet/events` | `/v1/events` with current filters. |
| `/fleet/compliance` | `/v1/fleet/compliance`. |
| `/hosts/:hostname` header + metadata | `/v1/fleet/hosts/{host_id}`. **Note**: the URL uses `:hostname` for human readability; `sigil-manager` resolves hostname → `host_id` via a local cache populated from `/v1/fleet/hosts` (hostname comes from latest HostMetaSnapshot). When the cache is cold the URL falls back to showing `host_id` until the cache fills. |
| `/hosts/:hostname` Alerts tab | `/v1/events?host_id={host_id}&evidence_kind=...&...` |
| `/hosts/:hostname` Events tab | `/v1/events?host_id={host_id}` |
| `/hosts/:hostname` Compliance tab | `/v1/fleet/hosts/{host_id}` (policy_state block already present); a separate `/v1/fleet/compliance?host_id=...` filter is **not** in v1. |
| `/settings` connection indicator | `/v1/healthz` (no auth) every 5s + `/v1/meta` (authed) on load. |

If a screen calls an endpoint that doesn't exist in the table above, that's
a contract gap and must be resolved before implementation.

---

## 10. Resolution of prior open questions (producer-decided)

All seven open questions from Draft v0 §10 are now closed by the producer
spec (sigil 3b.4 §9). Recorded here so future readers don't reopen them:

| # | Draft v0 question | Producer resolution | Where applied |
|---|---|---|---|
| Q1 | Score scale 0-100 vs. 0.0-10.0? | **0.0-10.0 locked.** UI/UX spec §5.2 to be updated to display 0-10. | F7 |
| Q2 | `compliance_score` formula? | **Removed.** Raw signals only; UI derives status. | F13, §5.6 |
| Q3 | Per-tool risk rollup (1 row per host or per (host,tool))? | **1 row per host with `top_tool`.** Drill via host detail. | F14, §5.5 |
| Q4 | `open_event_counts_24h` fixed vs. configurable window? | **24h fixed.** Longer retrospectives done client-side or in SIEM. | §5.3 |
| Q5 | Index backing (SQLite vs. on-demand scan)? | **In-memory per-host HashMap + on-demand JSONL scan** for `/v1/events`. | F11, F12 |
| Q6 | Per-consumer tokens with rotation? | **Defer.** Single shared bearer in v1. | F2, §3 |
| Q7 | "Latest" semantics for Heartbeat-derived fields? | **Most recent event with that `Evidence` variant**, regardless of severity/bucket. | §5.4 `agent_health` |

---

## 11. Out of scope for this contract

Aligned with producer spec §8. Items here are explicitly **not** in
`sigil-server 0.5.0`; any push to add them needs a new contract version.

| Item | Why | Follow-up venue |
|---|---|---|
| Write endpoints (mutate fleet state) | Triage state is consumer-local; no mutation from outside. | Permanent reject |
| SSE / WebSocket streaming | Polling sufficient (UI/UX §7.2: 5s). | 3b.4.1 or sigil-cloud |
| Per-consumer auth tokens + rotation | Single shared bearer in v1. | 3b.4.1 |
| mTLS on the read side | Reverse proxy + TLS = operator responsibility. | sigil-cloud |
| Aggregations > 24h window | Longer retrospectives via client-side or SIEM. | 3b.4.1 |
| Saved queries / hunts / case grouping | YAGNI per UI/UX §11. | Permanent reject |
| Multi-tenancy / org-team scoping | `sigil-cloud` territory; explicit per `CLAUDE.md`. | sigil-cloud |
| Server-side rate limiting | `429` code reserved but not enforced in v1. | 3b.4.1 |
| Full event index (host_id, kind, severity) | Per-host summary is enough at MVP; `/v1/events` is JSONL scan. | Driven by profiling |
| Per-host fine-grained lock | Single `RwLock<HashMap>` at MVP. | Driven by contention measurement |

---

## 12. Producer alignment trace

This contract's v1.0 was produced by reconciling Draft v0 (consumer-side,
2026-05-16) with the producer spec (`Ju571nK/sigil` Phase 3b.4, 2026-05-17).
The reconciliation moves were:

| Consumer-side change | Driver |
|---|---|
| Added F11 (in-memory index) and F12 (sync update) | Producer decisions B, F, H |
| Added F13 (raw compliance signals) | Producer decision I |
| Added F14 (1-row-per-host `top_tool`) | Producer §9 Q3 resolution |
| Added F15 (boot rebuild → 503 + Retry-After) | Producer §4.1 / §6 |
| §3 split into 3.1–3.4 (token + rotation + threat model + 404 rationale) | Producer §7.1 |
| §5.3/§5.5/§5.6 added `hostname` field | Consumer-driven; HostMetaSnapshot (3b.4-pre) shipped 2026-05-17 |
| §5.4 added full `host_meta` block | HostMetaSnapshot wire-stable types |
| §5.6 removed `compliance_score`; UI derivation rule moved into this contract | F13 |
| §6 added `503 service_unavailable` | F15 |
| §10 replaced with resolution table | Producer §9 |
| §11 expanded with `Follow-up venue` column | Producer §8 |
| §13 (new) records gaps NOT addressed by producer | Codex challenge 2026-05-17 |

### 12.1 After-ship checklist (mirrors producer §11.4)

When `sigil-server 0.5.0` ships:
1. Update this doc's status header to *"locked against sigil-server 0.5.0"*.
2. Start `sigil-manager` Plan 02 (`FleetClient` Go interface + Mock client +
   UI wiring + triage SQLite + single-admin JWT).
3. Update `sigil-manager` README quickstart to mention
   `SIGIL_SERVER_BASE_URL` and `SIGIL_SERVER_READ_TOKEN` env vars.

Until `0.5.0` ships, Plan 02 implements against this v1.0 contract behind a
`FleetClient` trait + Mock for development.

---

## 13. Known gaps (not addressed in v1)

These are issues codex's adversarial review (2026-05-17) flagged that the
producer's locked spec does **not** address. They are recorded here so the
consumer side knows what to compensate for, and so follow-up issues against
`Ju571nK/sigil` are easy to file later.

| Gap | What's missing | Consumer-side impact / mitigation |
|---|---|---|
| **Clock skew** | No `server_received_ts` on events. `last_seen_ts` / `since` / `until` all rely on agent's `event.ts`. | Hosts with bad clocks may flip status. Mitigation: surface "host clock drifted from server" warning if a HostMetaSnapshot's `ts` is materially behind the consumer's wall clock at fetch time. Follow-up issue on sigil. |
| **Search** | `/v1/events` has no `q=`, no full-text. UI/UX §7.3 wants `q=injection`. | Plan 02 implements client-side filter on response bodies (limited to current page). Server-side search is post-v1. |
| **Severity expansion** | Filter only accepts `info,warn`. Future variants with `error/critical` are undefined. | Mitigation: consumer treats unknown severity as `warn` for display, includes the raw string in the event detail panel. |
| **Unknown evidence variant display** | Wire-stable additive — server returns it; consumer must render. | Mitigation: alert queue row template has a generic-evidence fallback (kind text + ts + host + "Unknown evidence — view raw"). |
| **Retention** | Server JSONL retention not in contract. UI may link to a triage row whose source event has rotated out. | Mitigation: triage rows in `sigil-manager` DB record a snapshot of the event's evidence payload at triage time, not just a reference. |
| **Observability surface** | No endpoint reports index lag, parse error count, scan time. | Mitigation: Settings page shows last response latency + connection state; deep ops are operator's `journalctl` problem. Follow-up issue on sigil. |
| **Batch fetch** | Slide-over fetches per event. | Mitigation: aggressive client-side caching (LRU, 200 events). Acceptable for MVP analyst. |
| **`open_event_counts_24h` definition** | Field name says "open" but server has no triage state — really means "events emitted in 24h". | Mitigation: in UI tooltip clarify it's a "raw count", not "outstanding alerts". |
| **Alerts definition drift** | Server-computed `open_alert_count_24h` uses producer's recommendation; consumer overrides recompute. | Mitigation: UI labels server-side count differently from consumer-derived count when the override is non-default. |
| **`open_alert_count_24h` spec/impl divergence** | Spec §4.5 says "events matching `alerts_definition_default`"; impl `0dce160` returns plain `sum_warn()`. | Mitigation: treat the field as "warn events in 24h", recompute precise alert counts client-side from `/v1/events`. Tracked as a follow-up issue (see §13.1). |

### 13.1 Suggested follow-up issues for `Ju571nK/sigil`

- **`/v1/fleet/risk.open_alert_count_24h` should respect
  `alerts_definition_default`.** Current impl (`0dce160`,
  `crates/sigil-server/src/routes/fleet_risk.rs:116`) returns
  `h.counts_24h.sum_warn()`, which is the trailing-24h warn-severity
  event sum and includes every warn-emitting Evidence variant
  (`WatcherDegraded`, `ChannelStall`, `SenderLagCritical`,
  `HostIdFingerprintDrift`, `AgentDying`, etc.). Producer spec §4.5
  states the field should be the count matching
  `alerts_definition_default` (`ai_guard_risk_assessed` with bucket
  high/critical, plus the 5 additional kinds listed in §4.2). The
  divergence misleads SOC analysts: a host with 100 `WatcherDegraded`
  events and zero actionable alerts will show `open_alert_count_24h: 100`.
- Add `server_received_ts` to event ingest path; expose via `/v1/events`
  and use for `last_seen_ts` in fleet endpoints (codex #7, #21).
- Add `/v1/observability/index` returning index lag, parse error count,
  last rebuild timestamp, last ingest timestamp (codex #22).
- Consider `?q=` (substring search on `subject.value` + `evidence.kind`)
  for `/v1/events` once profiling justifies the index cost (codex #15).

These are not blocking Plan 02 — `sigil-manager` will ship against the
contract as locked, then file these as Phase 3b.4.1 candidates.

## 14. Post-lock additive schema notes

The producer has shipped four phases since this contract was locked
(3b.4 read API, 3b.6 application-form parsers, 3b.6.1 Continue
per-repo discovery, 3b.6.2 Claude Code + Codex per-repo discovery).
**Every one is wire-additive** — no existing fields renamed or removed,
no endpoint behaviour changed — so this v1.0 contract still describes
the correct shape for fields it covers. Consumer-side display code
(when implemented) should be ready for the additional enum values and
scope shapes recorded below.

### 14.1 Phase 3b.4 — fleet aggregation API (shipped 2026-05-17, sigil main `d495ea3`)

This is the read API this contract was built against. Producer side
shipped exactly the 9 endpoints, scopes, and schemas locked in §4-§7.
No consumer-visible deltas vs this contract.

### 14.2 Phase 3b.6 — application-form AI agent coverage (shipped 2026-05-17, sigil main `d495ea3`)

Two new parsers (Claude Desktop, Continue.dev) emit through the same
`Evidence::AiGuardRiskAssessed` path documented in §5. Wire additions:

**New `tool` enum values** (string field inside `evidence`, snake_case):

| Wire string | Producer enum | Description |
|---|---|---|
| `"claude_code"` | `AiTool::ClaudeCode` | (already in contract — CLI form, Phase 3b.1) |
| `"codex"` | `AiTool::Codex` | (already in contract — CLI form, Phase 3b.1) |
| `"claude_desktop"` | `AiTool::ClaudeDesktop` | **NEW (3b.6)** — Anthropic.app desktop config |
| `"continue_dev"` | `AiTool::ContinueDev` | **NEW (3b.6)** — Continue.dev VSCode/JetBrains extension |

**New `scope.kind` value**: in addition to the existing `"user_global"` and
`"project"` shapes, application-form parsers emit:

```json
{
  "scope": {
    "kind": "application",
    "app": "claude_desktop"
  }
}
```

The `app` string is a stable, snake_case identifier matching the parser.
For 3b.6 it's `"claude_desktop"` for Claude Desktop or `"continue"` for
Continue.dev (note the latter drops the `_dev` that the tool enum carries
— this is intentional and reflects the vendor's own product name).

**Reason variants**: no new variants — 3b.6 reuses `no_sandbox`,
`mcp_server_remote`, `destructive_in_inline_command`,
`destructive_in_hook_script`, `external_script_unscanned` from Phase 3b.1.
A new `executor` string value `"mcp_command"` (in addition to the
existing `"host_shell"`) appears in `NoSandbox` reasons emitted by
app-form parsers, and `hook_event` may now take the values
`"mcp_command"`, `"slash_command"`, or `"custom_command"` from the
Continue.dev parser.

**Consumer impact when sigil-manager builds the fleet UI:**
- Render new tool strings with appropriate icons / labels — recommended:
  Claude Desktop = Anthropic icon variant, Continue.dev = Continue logo,
  fall back to generic AI badge for unknown values.
- Handle `scope.kind === "application"` alongside `"user_global"` and
  `"project"` in any scope-aware grouping or filter UI.
- No alerts-definition change — `ai_guard_risk_assessed` is already in
  `/v1/meta.alerts_definition_default.evidence_kinds` and continues to
  cover both CLI- and application-form emissions.

**Reference:** producer spec at
`Ju571nK/sigil:docs/superpowers/specs/2026-05-17-phase-3b6-app-form-coverage-design.html`
(local-only — gitignored on producer side).

### 14.3 Phase 3b.6.1 — Continue.dev per-repo discovery (shipped 2026-05-18, sigil main `f49b6de`)

Adds operator-configurable per-repository discovery for Continue.dev:
the operator lists workspace roots in the signed policy envelope's new
`continue_workspaces: Vec<String>` field, the agent walks each root
one level deep, and for every direct subdir containing
`.continue/config.json` it spawns a `ContinueDevProjectParser`
emitting `AiGuardRiskAssessed` events with:

```json
{
  "evidence": {
    "tool": "continue_dev",
    "scope": { "kind": "project", "path": "/abs/path/to/repo" },
    ...
  }
}
```

**Wire changes vs §14.2**: none new. `AiTool::ContinueDev` and
`AiGuardScope::Project { path }` already existed; this phase just
emits **more events with `tool=continue_dev` + `scope.kind=project`**.

**Consumer impact:**
- `/v1/fleet/hosts/{host_id}.ai_guard.by_tool.continue_dev` may now
  hold a `scope.kind="project"` entry. The host-detail rendering MUST
  display the `path` (UI can show the repo basename + tooltip with
  full path).
- `/v1/events` filtered by `evidence_kind=ai_guard_risk_assessed` will
  return both user-global (from 3b.6) and per-project (from 3b.6.1)
  variants intermixed; the SOC analyst should be able to distinguish
  by `scope`.
- The `current_risk.by_tool.continue_dev` block in `/v1/fleet/hosts`
  list responses is overwritten per `(tool, scope)` key at the
  producer side — meaning a user-global Continue.dev assessment and a
  per-project one for the **same tool string** can stomp each other.
  Plan 03 (host detail page) should fetch the host's full event
  history when the operator drills into Continue.dev specifically, to
  surface both scopes.

**Reference:** producer spec at
`Ju571nK/sigil:docs/superpowers/specs/2026-05-17-phase-3b6.1-continue-per-repo-discovery-design.html`
and plan
`Ju571nK/sigil:docs/superpowers/plans/2026-05-17-phase-3b6.1-continue-per-repo-discovery.html`.

### 14.4 Phase 3b.6.2 — Claude Code + Codex per-repo discovery (in flight 2026-05-18, branch `feat/phase-3b6.2-claude-codex-per-repo-discovery`)

Extends the 3b.6.1 pattern to two more tools. Two new policy envelope
fields on the producer side — `claude_code_workspaces: Vec<String>`
and `codex_workspaces: Vec<String>` — let operators point the agent
at workspace roots for Claude Code (`<repo>/.claude/settings.json`,
`settings.local.json`, `.claude/hooks/`) and Codex
(`<repo>/.codex/config.toml`) respectively. For each marker-bearing
subdir the agent emits `AiGuardRiskAssessed` events with:

```json
{
  "evidence": {
    "tool": "claude_code",   // or "codex"
    "scope": { "kind": "project", "path": "/abs/path/to/repo" },
    ...
  }
}
```

**Wire changes vs §14.3**: none new. **No new enum variants on either
`AiTool` or `AiGuardScope`.** Just more `scope.kind="project"` events
under tool strings that have existed since Phase 3b.1.

**Consumer impact:**
- Same `(tool, scope)` overwrite risk as §14.3 — a per-project
  `claude_code` assessment will overwrite the user-global one in
  `current_risk.by_tool.claude_code` (and vice versa). Mitigation is
  the same: pull host's recent events when the analyst drills into a
  specific tool, or wait for producer to expose per-(tool, scope)
  history in a future read-API extension.
- Hooks scan (Claude Code per-repo only) means hook-script-containing
  reasons (`destructive_in_hook_script`, `external_script_unscanned`)
  may now appear with `scope.kind="project"`. Existing reason variants
  are reused — no rendering changes required.

**Reference:** producer spec at
`Ju571nK/sigil:docs/superpowers/specs/2026-05-18-phase-3b6.2-claude-codex-per-repo-discovery-design.html`
and plan
`Ju571nK/sigil:docs/superpowers/plans/2026-05-18-phase-3b6.2-claude-codex-per-repo-discovery.html`.
Producer explicitly requested this §14 entry be added (per the spec's
producer-first §0 paragraph).

### 14.5 Aggregate consumer-side checklist (across 3b.4 → 3b.6.2)

For Plan 02 and beyond, sigil-manager's rendering paths MUST handle:

- **Four `tool` values:** `claude_code`, `codex`, `claude_desktop`,
  `continue_dev` — plus a generic fallback for future additions
  (e.g., Gemini CLI + Cursor when Phase 3b.2 ships).
- **Three `scope.kind` shapes:**
  - `{"kind": "user_global"}` — no extra fields.
  - `{"kind": "project", "path": "/abs/..."}` — display path.
  - `{"kind": "application", "app": "claude_desktop"\|"continue"\|...}` —
    display app name.
- **The `(tool, scope)` overwrite issue** in
  `current_risk.by_tool` for any tool emitted under multiple scopes.
- **HostMetaSnapshot** wire payload everywhere `host_meta` is exposed.

Mock fixtures in `internal/fleet/mock.go` (Plan 02 Task 4) MUST cover
at least one event from each of these surfaces so the SPA's rendering
paths get exercised against realistic producer-side shapes.
