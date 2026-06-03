package fleet

import (
	"context"
	"sync"
	"time"

	"golang.org/x/sync/singleflight"
)

// CacheConfig tunes the [CachingClient]. Zero values fall back to sane
// defaults in [NewCachingClient].
type CacheConfig struct {
	// TTL is how long a cached entry is considered fresh.
	TTL time.Duration
	// MaxStale is how far past TTL an entry may still be served (stale) while
	// a background refresh runs, or when the upstream is unavailable.
	MaxStale time.Duration
	// MaxEntries bounds the number of distinct cache keys held.
	MaxEntries int

	// clock is injectable for deterministic tests; nil uses time.Now.
	clock func() time.Time
}

// entry is one cached response plus the instant it was fetched.
type entry struct {
	val       any
	fetchedAt time.Time
}

// cachePolicy is the freshness window for one logical endpoint.
type cachePolicy struct {
	ttl      time.Duration // fresh below this age
	maxStale time.Duration // served stale (and revalidated) up to this age
}

// slowTierMultiplier scales the live TTL up for slow-changing / immutable
// endpoints (meta, policy_meta, event_by_id).
const slowTierMultiplier = 12

// buildPolicies derives per-endpoint freshness from the base (live) window.
// Two tiers: live data tracks the poll interval; metadata and immutable
// (past) events get a much longer TTL.
func buildPolicies(live cachePolicy) map[string]cachePolicy {
	slow := cachePolicy{ttl: slowTierMultiplier * live.ttl, maxStale: slowTierMultiplier * live.maxStale}
	return map[string]cachePolicy{
		"meta":        slow,
		"policy_meta": slow,
		"event_by_id": slow, // a past event is immutable
		"events":      live,
		"hosts":       live,
		"host_by_id":  live,
		"risk":        live,
		"compliance":  live,
	}
}

// CachingClient decorates a [Client] with single-flight de-duplication and a
// TTL cache (stale-while-revalidate is layered on in later cycles).
type CachingClient struct {
	next     Client
	cfg      CacheConfig
	sf       singleflight.Group
	clock    func() time.Time
	policies map[string]cachePolicy
	deflt    cachePolicy

	mu    sync.Mutex
	store map[string]*entry
}

// Cache defaults applied by [NewCachingClient] for zero-value config fields.
const (
	defaultCacheTTL        = 5 * time.Second
	defaultCacheMaxStale   = 60 * time.Second
	defaultCacheMaxEntries = 512
)

// NewCachingClient wraps next with caching per cfg. Zero-value fields fall
// back to sane defaults.
func NewCachingClient(next Client, cfg CacheConfig) *CachingClient {
	if cfg.TTL <= 0 {
		cfg.TTL = defaultCacheTTL
	}
	if cfg.MaxStale <= 0 {
		cfg.MaxStale = defaultCacheMaxStale
	}
	if cfg.MaxEntries <= 0 {
		cfg.MaxEntries = defaultCacheMaxEntries
	}
	clock := cfg.clock
	if clock == nil {
		clock = time.Now
	}
	live := cachePolicy{ttl: cfg.TTL, maxStale: cfg.MaxStale}
	return &CachingClient{
		next:     next,
		cfg:      cfg,
		clock:    clock,
		policies: buildPolicies(live),
		deflt:    live,
		store:    make(map[string]*entry),
	}
}

// policyFor returns the freshness policy for a logical endpoint, falling back
// to the live (default) policy for any unmapped endpoint.
func (c *CachingClient) policyFor(endpoint string) cachePolicy {
	if p, ok := c.policies[endpoint]; ok {
		return p
	}
	return c.deflt
}

var _ Client = (*CachingClient)(nil)

// getCached implements stale-while-revalidate with single-flight:
//
//	age < TTL          → fresh; return cached.
//	TTL ≤ age < MaxStale → return stale immediately, refresh in background.
//	age ≥ MaxStale      → blocking refresh; fall back to stale on error.
//	miss                → blocking fetch.
//
// Concurrent misses/refreshes for the same key collapse into one upstream call.
// The freshness window is the policy for endpoint (see [buildPolicies]).
//
// The cache key is `endpoint + "|" + suffix`, so the endpoint name picks both
// the freshness policy and the key namespace from one argument (no chance of
// the two drifting apart). suffix is "" for parameterless endpoints.
//
// The upstream fetch runs on a context detached from the caller's
// (context.WithoutCancel), so a background revalidation — or a single-flight
// follower whose own request was canceled — still completes. The upstream
// HTTP client carries its own timeout, so the detached context cannot hang.
//
// The returned *T is shared with every concurrent reader of the same key;
// callers MUST treat it as read-only (the fleet handlers only serialize it).
func getCached[T any](c *CachingClient, ctx context.Context, endpoint, suffix string, fetch func(context.Context) (*T, error)) (*T, error) {
	pol := c.policyFor(endpoint)
	key := endpoint + "|" + suffix
	fctx := context.WithoutCancel(ctx)

	// fetchStore performs the upstream call and, on success, stores the result.
	fetchStore := func() (any, error) {
		out, ferr := fetch(fctx)
		if ferr != nil {
			return nil, ferr
		}
		c.mu.Lock()
		c.evictIfNeededLocked(key)
		c.store[key] = &entry{val: out, fetchedAt: c.clock()}
		c.mu.Unlock()
		return out, nil
	}

	now := c.clock()
	c.mu.Lock()
	e, ok := c.store[key]
	c.mu.Unlock()

	if ok {
		age := now.Sub(e.fetchedAt)
		switch {
		case age < pol.ttl:
			return e.val.(*T), nil // fresh
		case age < pol.maxStale:
			// Serve stale now; refresh in the background. Single-flight makes
			// concurrent stale reads share one refresh.
			go func() { _, _, _ = c.sf.Do(key, fetchStore) }()
			return e.val.(*T), nil
		default:
			// Too stale: refresh synchronously. Degrade to the last-known value
			// ONLY for transient upstream errors (503 / network timeout) — the
			// dashboard shows old data rather than going blank. Terminal errors
			// (401 token revoked, 404 gone, etc.) must surface, never be masked.
			v, ferr, _ := c.sf.Do(key, fetchStore)
			if ferr != nil {
				if isRetryable(ferr) {
					return e.val.(*T), nil
				}
				return nil, ferr
			}
			return v.(*T), nil
		}
	}

	// Cold miss: block on the fetch.
	v, err, _ := c.sf.Do(key, fetchStore)
	if err != nil {
		return nil, err
	}
	return v.(*T), nil
}

// evictIfNeededLocked makes room for a new key when the store is at capacity,
// evicting the entry with the oldest fetch time. Caller must hold c.mu. A
// no-op when key already exists (an update, not a new entry) or MaxEntries
// is unbounded.
func (c *CachingClient) evictIfNeededLocked(key string) {
	if c.cfg.MaxEntries <= 0 {
		return
	}
	if _, exists := c.store[key]; exists {
		return
	}
	for len(c.store) >= c.cfg.MaxEntries {
		var oldestKey string
		var oldestAt time.Time
		first := true
		for k, ent := range c.store {
			if first || ent.fetchedAt.Before(oldestAt) {
				oldestKey, oldestAt, first = k, ent.fetchedAt, false
			}
		}
		delete(c.store, oldestKey)
	}
}

// Events implements [Client.Events] with caching.
func (c *CachingClient) Events(ctx context.Context, p EventsParams) (*EventsPage, error) {
	return getCached(c, ctx, "events", buildEventsQuery(p).Encode(),
		func(fctx context.Context) (*EventsPage, error) { return c.next.Events(fctx, p) })
}

// Meta implements [Client.Meta] with caching.
func (c *CachingClient) Meta(ctx context.Context) (*Meta, error) {
	return getCached(c, ctx, "meta", "",
		func(fctx context.Context) (*Meta, error) { return c.next.Meta(fctx) })
}

// Healthz implements [Client.Healthz]; it is intentionally not cached.
func (c *CachingClient) Healthz(ctx context.Context) (*Healthz, error) {
	// Never cached: a health probe must reflect the live upstream every call.
	return c.next.Healthz(ctx)
}

// PolicyMeta implements [Client.PolicyMeta] with caching.
func (c *CachingClient) PolicyMeta(ctx context.Context) (*PolicyMeta, error) {
	return getCached(c, ctx, "policy_meta", "",
		func(fctx context.Context) (*PolicyMeta, error) { return c.next.PolicyMeta(fctx) })
}

// EventByID implements [Client.EventByID] with caching.
func (c *CachingClient) EventByID(ctx context.Context, id string) (*Event, error) {
	return getCached(c, ctx, "event_by_id", id,
		func(fctx context.Context) (*Event, error) { return c.next.EventByID(fctx, id) })
}

// FleetHosts implements [Client.FleetHosts] with caching.
func (c *CachingClient) FleetHosts(ctx context.Context, p HostsParams) (*HostsPage, error) {
	return getCached(c, ctx, "hosts", buildHostsQuery(p).Encode(),
		func(fctx context.Context) (*HostsPage, error) { return c.next.FleetHosts(fctx, p) })
}

// FleetHostByID implements [Client.FleetHostByID] with caching.
func (c *CachingClient) FleetHostByID(ctx context.Context, id string) (*HostDetail, error) {
	return getCached(c, ctx, "host_by_id", id,
		func(fctx context.Context) (*HostDetail, error) { return c.next.FleetHostByID(fctx, id) })
}

// FleetRisk implements [Client.FleetRisk] with caching.
func (c *CachingClient) FleetRisk(ctx context.Context, p RiskParams) (*RiskPage, error) {
	return getCached(c, ctx, "risk", buildRiskQuery(p).Encode(),
		func(fctx context.Context) (*RiskPage, error) { return c.next.FleetRisk(fctx, p) })
}

// FleetCompliance implements [Client.FleetCompliance] with caching.
func (c *CachingClient) FleetCompliance(ctx context.Context, p ComplianceParams) (*CompliancePage, error) {
	return getCached(c, ctx, "compliance", buildComplianceQuery(p).Encode(),
		func(fctx context.Context) (*CompliancePage, error) { return c.next.FleetCompliance(fctx, p) })
}
