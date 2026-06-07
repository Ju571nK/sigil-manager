# Fleet read cache (Axis A) — design

**Status:** implemented on `feat/fleet-cache-layer` (engine + per-endpoint TTLs
+ factory wiring).
**Date:** 2026-05-31.

## Problem

`sigil-manager` reads fleet evidence from `sigil-server` and renders dashboards.
Until now the `/api/v1/fleet/*` handlers were a **thin pass-through**: every
browser request triggered a fresh `GET` to `sigil-server`. The SPA polls
(`FLEET_POLL_INTERVAL_SECONDS`, default 5s) per visible panel, so upstream load
scaled as `concurrent clients × panels × poll rate`. At enterprise scale that
is a thundering herd on the producer.

This is the **manager↔server pull link**. Pull is the correct model here (a
read-only console pulling an aggregation API; pushing would couple the producer
to every self-hosted consumer). The fix is not push — it is to **decouple
browser fan-out from upstream load** with a cache in the manager.

## Approach: stale-while-revalidate + single-flight

A `CachingClient` decorates the `fleet.Client` interface
(`internal/fleet/cache.go`). It is the only change to the read path — handlers,
`http.go`, `mock.go`, and the interface are untouched. `NewFromConfig` wraps the
chosen client unless `FLEET_CACHE_DISABLED=1`.

State machine per cache key (`method + normalized params`), by entry age:

| age | behavior |
| --- | --- |
| `< TTL` | fresh — return cached |
| `TTL ≤ age < MaxStale` | return **stale immediately**, refresh in background |
| `≥ MaxStale` | blocking refresh; fall back to stale **only on transient errors** |
| miss | blocking fetch |

The upstream fetch runs on a context **detached** from the caller
(`context.WithoutCancel`), so a background revalidation still completes after the
handler returns (`r.Context()` canceled), and a single-flight follower whose own
request was canceled does not fail the shared fetch. The upstream HTTP client
carries its own timeout, so the detached context cannot hang.

Degradation to stale fires **only for transient errors** (`isRetryable`: 503 /
network timeout). Terminal errors (401 token revoked, 404 gone, …) surface —
they are never masked by serving stale data.

Cache keys are the **canonical query encoding** (`buildEventsQuery(p).Encode()`
etc., the same builders `http.go` uses), not `fmt.Sprintf("%+v")`. This
percent-escapes values (so `["a","b"]` and `["a b"]` can't collide) and
normalizes `Since`/`Until` to UTC RFC3339 (no monotonic-clock / time-zone key
churn).

- **single-flight** (`golang.org/x/sync/singleflight`) collapses concurrent
  identical misses/refreshes into one upstream call — no herd on expiry.
- **SWR** means readers almost always get an instant cached answer; refreshes
  happen off the request path. Upstream load becomes a steady trickle
  (`keys / TTL`), independent of client count.
- **Graceful degradation**: if `sigil-server` blips, the dashboard shows
  last-known-good data instead of going blank. Dovetails with the existing
  503 / `Retry-After` handling in `retry.go`.

## Configuration

| env | meaning | default |
| --- | --- | --- |
| `FLEET_CACHE_DISABLED` | `1` → pass-through, no cache (kill-switch) | `0` |
| `FLEET_CACHE_MAX_ENTRIES` | LRU-ish bound on distinct keys | `512` |
| `FLEET_POLL_INTERVAL_SECONDS` | base live **TTL**; `MaxStale = 12×` | `5` |

### Per-endpoint freshness (two tiers)

Derived from the base live window (`buildPolicies`):

| tier | endpoints | TTL | MaxStale |
| --- | --- | --- | --- |
| live | `events`, `hosts`, `host_by_id`, `risk`, `compliance` | `base` | `12×base` |
| slow | `meta`, `policy_meta`, `event_by_id` (immutable past event) | `12×base` | `144×base` |
| — | `healthz` | not cached | — |

At the default `base=5s`: live = 5s / 60s, slow = 60s / 12m. The slow tier
avoids re-hitting `sigil-server` for metadata that barely changes, while live
aggregates stay as fresh as the SPA's poll.

## Scope decisions

- **`/v1/healthz` is never cached** — a health probe must reflect the live
  upstream every call.
- **Triage is not cached** — `handleFleetEvents` joins the triage SQLite *after*
  the fleet call, so triage state stays live; no invalidation needed.
- **Only successful (2xx) responses are cached.** Errors pass through (with the
  existing retry), except the stale-fallback degradation path.
- **Single tenant, single upstream** — one global cache, no per-tenant keys.
  Per-tenant caching is a `sigil-cloud` concern, deliberately absent here.
- **In-process memory cache only** — no Redis. Matches the single-binary,
  self-host model.
- Eviction is **oldest-`fetchedAt`** when at capacity, O(1) via a
  `container/list` ordered by fetch time (front = oldest; refreshes move to
  back). Not access-LRU, but the memory bound is enforced without an O(n) scan.
- `items`/`order` are guarded by an **RWMutex** so fresh-hit reads (the hot
  path the cache exists to serve) proceed concurrently; only stores/evictions
  take the write lock. Read values are copied out under the read lock.
- A background revalidation spawns **at most one goroutine per key** (a
  `refreshing` guard), so a fan-out burst of stale reads can't pile up
  goroutines waiting on the shared single-flight call.
- Returned values are **shared pointers** across concurrent readers of the same
  key — callers MUST treat them as read-only (the fleet handlers only serialize
  them). The cache does not deep-copy (it would cost on every hit for a hazard
  no current caller triggers).

## Tests (`internal/fleet/cache_test.go`, all TDD red→green)

1. Concurrent identical misses collapse to **one** upstream call (herd).
2. Second call within TTL hits cache (0 extra upstream calls).
3. SWR: post-TTL read returns stale immediately + triggers background refresh;
   a later read sees the refreshed value (exactly 2 upstream calls total).
4. Graceful degradation: past `MaxStale` + upstream down → serves last-good.
5. `Healthz` is never cached (3 calls → 3 upstream calls).
6. Distinct keys past `MaxEntries` evict — store stays bounded.
7. Zero-value config falls back to sane defaults (non-zero TTL).
8. Per-endpoint TTL: past the live TTL, `Meta` is still served fresh (slow
   tier) while `Events` is already stale.
9. Background refresh runs on a detached context — it completes even when the
   triggering request's context is already canceled.
10. Terminal errors (`ErrUnauthorized`) surface past MaxStale; only transient
    errors degrade to stale.
11. Distinct slice params (`["a","b"]` vs `["a b"]`) get distinct cache keys.

Race-clean (`go test -race`).

## Deliberately deferred (follow-ups)

- **Freshness exposure to the SPA** (`X-Sigil-Cache: hit|stale|miss` + `Age`),
  so the UI can badge "data N seconds old" / degraded state. The cache layer
  returns Go structs (not HTTP responses); surfacing this without changing the
  `fleet.Client` interface needs handler-level plumbing — out of this PR.
- **Background warmer** for hot canonical keys (excluded as YAGNI — SWR warms
  lazily on first access).
