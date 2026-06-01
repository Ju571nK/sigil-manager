package fleet

import (
	"context"
	"fmt"
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
func getCached[T any](c *CachingClient, endpoint, key string, fetch func() (*T, error)) (*T, error) {
	pol := c.policyFor(endpoint)

	// fetchStore performs the upstream call and, on success, stores the result.
	fetchStore := func() (any, error) {
		out, ferr := fetch()
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
			// Too stale: refresh synchronously, but degrade to the last-known
			// value if the upstream is unavailable (graceful degradation — the
			// dashboard shows old data rather than going blank).
			v, ferr, _ := c.sf.Do(key, fetchStore)
			if ferr != nil {
				return e.val.(*T), nil
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

func (c *CachingClient) Events(ctx context.Context, p EventsParams) (*EventsPage, error) {
	return getCached(c, "events", "events|"+fmt.Sprintf("%+v", p), func() (*EventsPage, error) { return c.next.Events(ctx, p) })
}

func (c *CachingClient) Meta(ctx context.Context) (*Meta, error) {
	return getCached(c, "meta", "meta", func() (*Meta, error) { return c.next.Meta(ctx) })
}

func (c *CachingClient) Healthz(ctx context.Context) (*Healthz, error) {
	// Never cached: a health probe must reflect the live upstream every call.
	return c.next.Healthz(ctx)
}

func (c *CachingClient) PolicyMeta(ctx context.Context) (*PolicyMeta, error) {
	return getCached(c, "policy_meta", "policy_meta", func() (*PolicyMeta, error) { return c.next.PolicyMeta(ctx) })
}

func (c *CachingClient) EventByID(ctx context.Context, id string) (*Event, error) {
	return getCached(c, "event_by_id", "event_by_id|"+id, func() (*Event, error) { return c.next.EventByID(ctx, id) })
}

func (c *CachingClient) FleetHosts(ctx context.Context, p HostsParams) (*HostsPage, error) {
	return getCached(c, "hosts", "hosts|"+fmt.Sprintf("%+v", p), func() (*HostsPage, error) { return c.next.FleetHosts(ctx, p) })
}

func (c *CachingClient) FleetHostByID(ctx context.Context, id string) (*HostDetail, error) {
	return getCached(c, "host_by_id", "host_by_id|"+id, func() (*HostDetail, error) { return c.next.FleetHostByID(ctx, id) })
}

func (c *CachingClient) FleetRisk(ctx context.Context, p RiskParams) (*RiskPage, error) {
	return getCached(c, "risk", "risk|"+fmt.Sprintf("%+v", p), func() (*RiskPage, error) { return c.next.FleetRisk(ctx, p) })
}

func (c *CachingClient) FleetCompliance(ctx context.Context, p ComplianceParams) (*CompliancePage, error) {
	return getCached(c, "compliance", "compliance|"+fmt.Sprintf("%+v", p), func() (*CompliancePage, error) { return c.next.FleetCompliance(ctx, p) })
}
