package fleet

import (
	"context"
	"strconv"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// countingClient is a test [Client] double that counts upstream calls per
// method, can delay each call (to widen concurrency windows), can be told to
// return an error, and encodes a "generation" marker into its responses so
// tests can tell a fresh response from a stale one.
type countingClient struct {
	mu    sync.Mutex
	calls map[string]int
	gen   int           // bumped by setGen to simulate upstream data changing
	delay time.Duration // slept inside each call when > 0
	err   error         // returned by each call when non-nil

	// called receives the method name on every upstream call (buffered big);
	// tests use it to observe async refreshes.
	called chan string
}

func newCountingClient() *countingClient {
	return &countingClient{calls: map[string]int{}, called: make(chan string, 1024)}
}

func (c *countingClient) record(method string) (gen int, err error) {
	c.mu.Lock()
	c.calls[method]++
	g, d, e := c.gen, c.delay, c.err
	c.mu.Unlock()
	select {
	case c.called <- method:
	default:
	}
	if d > 0 {
		time.Sleep(d)
	}
	return g, e
}

func (c *countingClient) callCount(method string) int {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.calls[method]
}

func (c *countingClient) setGen(g int)             { c.mu.Lock(); c.gen = g; c.mu.Unlock() }
func (c *countingClient) setErr(e error)           { c.mu.Lock(); c.err = e; c.mu.Unlock() }
func (c *countingClient) setDelay(d time.Duration) { c.mu.Lock(); c.delay = d; c.mu.Unlock() }

// genMarker turns a generation int into a string we can stash in a response
// field (e.g. NextCursor / ServerVersion) and assert on.
func genMarker(g int) string { return "gen-" + strconv.Itoa(g) }

func (c *countingClient) Events(_ context.Context, _ EventsParams) (*EventsPage, error) {
	g, err := c.record("Events")
	if err != nil {
		return nil, err
	}
	m := genMarker(g)
	return &EventsPage{NextCursor: &m}, nil
}

func (c *countingClient) Meta(_ context.Context) (*Meta, error) {
	g, err := c.record("Meta")
	if err != nil {
		return nil, err
	}
	return &Meta{ServerVersion: genMarker(g)}, nil
}

func (c *countingClient) Healthz(_ context.Context) (*Healthz, error) {
	g, err := c.record("Healthz")
	if err != nil {
		return nil, err
	}
	return &Healthz{Status: genMarker(g)}, nil
}

func (c *countingClient) PolicyMeta(_ context.Context) (*PolicyMeta, error) {
	g, err := c.record("PolicyMeta")
	if err != nil {
		return nil, err
	}
	return &PolicyMeta{PolicyVersion: g}, nil
}

func (c *countingClient) EventByID(_ context.Context, _ string) (*Event, error) {
	g, err := c.record("EventByID")
	if err != nil {
		return nil, err
	}
	return &Event{EventID: genMarker(g)}, nil
}

func (c *countingClient) FleetHosts(_ context.Context, _ HostsParams) (*HostsPage, error) {
	g, err := c.record("FleetHosts")
	if err != nil {
		return nil, err
	}
	m := genMarker(g)
	return &HostsPage{NextCursor: &m}, nil
}

func (c *countingClient) FleetHostByID(_ context.Context, _ string) (*HostDetail, error) {
	g, err := c.record("FleetHostByID")
	if err != nil {
		return nil, err
	}
	hd := &HostDetail{}
	hd.HostID = genMarker(g)
	return hd, nil
}

func (c *countingClient) FleetRisk(_ context.Context, _ RiskParams) (*RiskPage, error) {
	g, err := c.record("FleetRisk")
	if err != nil {
		return nil, err
	}
	m := genMarker(g)
	return &RiskPage{NextCursor: &m}, nil
}

func (c *countingClient) FleetCompliance(_ context.Context, _ ComplianceParams) (*CompliancePage, error) {
	g, err := c.record("FleetCompliance")
	if err != nil {
		return nil, err
	}
	m := genMarker(g)
	return &CompliancePage{NextCursor: &m}, nil
}

var _ Client = (*countingClient)(nil)

// testCacheConfig returns a CacheConfig with a generous TTL and an injectable
// clock, suitable for deterministic tests.
func testCacheConfig() CacheConfig {
	return CacheConfig{
		TTL:        100 * time.Millisecond,
		MaxStale:   time.Second,
		MaxEntries: 128,
	}
}

// TestCache_CollapsesConcurrentMisses is RED #1: N goroutines requesting the
// same uncached key concurrently must collapse into exactly ONE upstream call
// (single-flight), not N. This is the thundering-herd guarantee.
func TestCache_CollapsesConcurrentMisses(t *testing.T) {
	up := newCountingClient()
	up.setDelay(50 * time.Millisecond) // hold the in-flight call so all N pile up
	c := NewCachingClient(up, testCacheConfig())

	const n = 50
	var wg sync.WaitGroup
	var failures atomic.Int64
	wg.Add(n)
	for i := 0; i < n; i++ {
		go func() {
			defer wg.Done()
			page, err := c.Events(context.Background(), EventsParams{})
			if err != nil || page == nil {
				failures.Add(1)
			}
		}()
	}
	wg.Wait()

	if f := failures.Load(); f != 0 {
		t.Fatalf("got %d failed concurrent calls, want 0", f)
	}
	if got := up.callCount("Events"); got != 1 {
		t.Fatalf("upstream Events called %d times, want 1 (single-flight should collapse the herd)", got)
	}
}

// TestCache_SecondCallWithinTTLHitsCache is RED #2: a second sequential call
// for the same key within the TTL window must be served from cache without
// touching the upstream again.
func TestCache_SecondCallWithinTTLHitsCache(t *testing.T) {
	up := newCountingClient()
	c := NewCachingClient(up, testCacheConfig()) // TTL = 100ms

	first, err := c.Events(context.Background(), EventsParams{})
	if err != nil {
		t.Fatalf("first call: %v", err)
	}
	second, err := c.Events(context.Background(), EventsParams{})
	if err != nil {
		t.Fatalf("second call: %v", err)
	}

	if got := up.callCount("Events"); got != 1 {
		t.Fatalf("upstream Events called %d times, want 1 (second call should hit cache within TTL)", got)
	}
	if *first.NextCursor != *second.NextCursor {
		t.Fatalf("cached response differs: first=%q second=%q", *first.NextCursor, *second.NextCursor)
	}
}

// manualClock is an injectable, advanceable clock for deterministic
// TTL/stale tests.
type manualClock struct {
	mu sync.Mutex
	t  time.Time
}

func (m *manualClock) now() time.Time {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.t
}

func (m *manualClock) advance(d time.Duration) {
	m.mu.Lock()
	m.t = m.t.Add(d)
	m.mu.Unlock()
}

// waitForCall blocks until n upstream calls for method have been observed, or
// fails the test on timeout.
func waitForCall(t *testing.T, up *countingClient, method string, n int) {
	t.Helper()
	deadline := time.After(2 * time.Second)
	for {
		if up.callCount(method) >= n {
			return
		}
		select {
		case <-up.called:
		case <-deadline:
			t.Fatalf("timed out waiting for %d %s calls (saw %d)", n, method, up.callCount(method))
		}
	}
}

// TestCache_StaleWhileRevalidate is RED #3: once an entry passes TTL but is
// still within MaxStale, the next read returns the STALE value immediately
// (no blocking) and kicks off a background refresh. A later read then sees the
// refreshed value.
func TestCache_StaleWhileRevalidate(t *testing.T) {
	mc := &manualClock{t: time.Unix(1_700_000_000, 0)}
	up := newCountingClient()
	cfg := testCacheConfig() // TTL=100ms, MaxStale=1s
	cfg.clock = mc.now
	c := NewCachingClient(up, cfg)

	// Prime the cache at gen 0.
	first, err := c.Events(context.Background(), EventsParams{})
	if err != nil {
		t.Fatalf("prime: %v", err)
	}
	if *first.NextCursor != genMarker(0) {
		t.Fatalf("prime returned %q, want gen-0", *first.NextCursor)
	}

	// Move past TTL but stay within MaxStale; upstream now serves gen 1.
	mc.advance(200 * time.Millisecond)
	up.setGen(1)

	// This read must return the STALE gen-0 immediately, not block for gen-1.
	stale, err := c.Events(context.Background(), EventsParams{})
	if err != nil {
		t.Fatalf("stale read: %v", err)
	}
	if *stale.NextCursor != genMarker(0) {
		t.Fatalf("stale read returned %q, want stale gen-0 (SWR should not block on refresh)", *stale.NextCursor)
	}

	// The stale read should have triggered a background refresh (2nd upstream call).
	waitForCall(t, up, "Events", 2)

	// A subsequent read now sees the refreshed gen-1 with no further upstream call.
	deadline := time.After(2 * time.Second)
	for {
		fresh, err := c.Events(context.Background(), EventsParams{})
		if err != nil {
			t.Fatalf("post-refresh read: %v", err)
		}
		if *fresh.NextCursor == genMarker(1) {
			break
		}
		select {
		case <-deadline:
			t.Fatalf("never observed refreshed gen-1 after background refresh")
		default:
			time.Sleep(time.Millisecond)
		}
	}
	if got := up.callCount("Events"); got != 2 {
		t.Fatalf("upstream Events called %d times, want exactly 2 (prime + one background refresh)", got)
	}
}

// TestCache_ServesStaleWhenUpstreamDown is RED #4: when an entry is past
// MaxStale and the upstream is unavailable, the cache must degrade gracefully
// to the last-known-good value instead of surfacing the error — the dashboard
// shows slightly old data rather than going blank.
func TestCache_ServesStaleWhenUpstreamDown(t *testing.T) {
	mc := &manualClock{t: time.Unix(1_700_000_000, 0)}
	up := newCountingClient()
	cfg := testCacheConfig() // TTL=100ms, MaxStale=1s
	cfg.clock = mc.now
	c := NewCachingClient(up, cfg)

	// Prime the cache at gen 0.
	first, err := c.Events(context.Background(), EventsParams{})
	if err != nil {
		t.Fatalf("prime: %v", err)
	}

	// Age the entry well past MaxStale, then take the upstream down.
	mc.advance(5 * time.Second)
	up.setErr(&ServiceUnavailableError{RetryAfter: time.Second})

	got, err := c.Events(context.Background(), EventsParams{})
	if err != nil {
		t.Fatalf("expected graceful stale serve, got error: %v", err)
	}
	if got == nil || got.NextCursor == nil || *got.NextCursor != *first.NextCursor {
		t.Fatalf("expected last-known-good value, got %+v", got)
	}
}

// TestCache_HealthzNotCached is RED #5: Healthz must never be cached — a
// health probe has to reflect the live upstream every time, or the console
// would report a dead server as healthy (and vice versa).
func TestCache_HealthzNotCached(t *testing.T) {
	up := newCountingClient()
	c := NewCachingClient(up, testCacheConfig())

	for i := 0; i < 3; i++ {
		if _, err := c.Healthz(context.Background()); err != nil {
			t.Fatalf("healthz call %d: %v", i, err)
		}
	}

	if got := up.callCount("Healthz"); got != 3 {
		t.Fatalf("upstream Healthz called %d times, want 3 (health must never be cached)", got)
	}
}

// TestCache_EvictsBeyondMaxEntries is RED #6: distinct keys past MaxEntries
// must not grow the store without bound — older entries are evicted so memory
// stays bounded under a flood of unique filter combinations.
func TestCache_EvictsBeyondMaxEntries(t *testing.T) {
	up := newCountingClient()
	cfg := testCacheConfig()
	cfg.MaxEntries = 4
	c := NewCachingClient(up, cfg)

	// 20 distinct keys (distinct host filters → distinct cache keys).
	for i := 0; i < 20; i++ {
		p := EventsParams{HostIDs: []string{"host-" + strconv.Itoa(i)}}
		if _, err := c.Events(context.Background(), p); err != nil {
			t.Fatalf("call %d: %v", i, err)
		}
	}

	c.mu.Lock()
	n := len(c.store)
	c.mu.Unlock()
	if n > cfg.MaxEntries {
		t.Fatalf("store holds %d entries, want ≤ %d (MaxEntries must bound memory)", n, cfg.MaxEntries)
	}
}

// TestCache_ZeroConfigStillCaches is RED #7: a zero-value CacheConfig must
// fall back to sane defaults rather than a TTL of 0 (which would make every
// entry instantly stale and defeat the cache entirely).
func TestCache_ZeroConfigStillCaches(t *testing.T) {
	up := newCountingClient()
	c := NewCachingClient(up, CacheConfig{}) // all zero

	if _, err := c.Events(context.Background(), EventsParams{}); err != nil {
		t.Fatalf("first call: %v", err)
	}
	if _, err := c.Events(context.Background(), EventsParams{}); err != nil {
		t.Fatalf("second call: %v", err)
	}
	if got := up.callCount("Events"); got != 1 {
		t.Fatalf("upstream Events called %d times with zero config, want 1 (defaults must give a non-zero TTL)", got)
	}
}

// TestCache_PerEndpointTTL is RED #8: slow-changing metadata (Meta) must get a
// longer TTL than live data (Events). Past the live TTL but within the meta
// TTL, a Meta read is still served fresh (no upstream refetch), while an Events
// read at the same instant is already stale.
func TestCache_PerEndpointTTL(t *testing.T) {
	mc := &manualClock{t: time.Unix(1_700_000_000, 0)}
	up := newCountingClient()
	cfg := CacheConfig{
		TTL:        20 * time.Millisecond, // live tier
		MaxStale:   time.Second,
		MaxEntries: 128,
		clock:      mc.now,
	}
	c := NewCachingClient(up, cfg)

	// Prime both endpoints at gen 0.
	if _, err := c.Meta(context.Background()); err != nil {
		t.Fatalf("prime meta: %v", err)
	}
	if _, err := c.Events(context.Background(), EventsParams{}); err != nil {
		t.Fatalf("prime events: %v", err)
	}

	// Advance past the live (Events) TTL but well within the meta TTL.
	mc.advance(100 * time.Millisecond)

	// Sanity: at this instant Events IS stale — a read triggers a refresh.
	if _, err := c.Events(context.Background(), EventsParams{}); err != nil {
		t.Fatalf("events read: %v", err)
	}
	waitForCall(t, up, "Events", 2) // confirms 100ms is past the live TTL

	// Meta must still be fresh → served from cache with NO background refresh.
	if _, err := c.Meta(context.Background()); err != nil {
		t.Fatalf("meta read: %v", err)
	}
	// Give any (erroneous) background refresh time to fire before asserting.
	time.Sleep(50 * time.Millisecond)
	if got := up.callCount("Meta"); got != 1 {
		t.Fatalf("upstream Meta called %d times, want 1 (metadata TTL must outlast the live TTL)", got)
	}
}
