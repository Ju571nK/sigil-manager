package v1

import (
	"net"
	"net/http"
	"strconv"
	"sync"
	"time"
)

// rateLimiter is a per-IP fixed-window limiter for the login endpoint, to slow
// online brute force on top of the constant-time compare + bcrypt + fail-delay
// already in handleLogin. Loopback is exempt (the local operator / e2e), so the
// limit only constrains remote clients. RealIP middleware resolves the client
// address from X-Forwarded-For when behind a proxy, so the key reflects the
// real caller in production.
type rateLimiter struct {
	mu     sync.Mutex
	max    int
	window time.Duration
	hits   map[string]*hitWindow
	now    func() time.Time // injectable for tests
}

type hitWindow struct {
	count int
	reset time.Time
}

func newRateLimiter(max int, window time.Duration) *rateLimiter {
	return &rateLimiter{
		max:    max,
		window: window,
		hits:   make(map[string]*hitWindow),
		now:    time.Now,
	}
}

// allow records an attempt for key and reports whether it is under the limit.
// On rejection it returns the time remaining until the window resets.
func (rl *rateLimiter) allow(key string) (bool, time.Duration) {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	now := rl.now()
	w := rl.hits[key]
	if w == nil || now.After(w.reset) {
		// Opportunistically drop expired windows so the map can't grow without
		// bound as distinct source IPs come and go.
		if len(rl.hits) > 1024 {
			for k, hw := range rl.hits {
				if now.After(hw.reset) {
					delete(rl.hits, k)
				}
			}
		}
		rl.hits[key] = &hitWindow{count: 1, reset: now.Add(rl.window)}
		return true, 0
	}
	if w.count >= rl.max {
		return false, w.reset.Sub(now)
	}
	w.count++
	return true, 0
}

// middleware rejects requests from a remote IP that has exceeded the limit with
// 429 + Retry-After. Loopback callers are never limited.
func (rl *rateLimiter) middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ip := clientIP(r)
		if ip == "" || isLoopback(ip) {
			next.ServeHTTP(w, r)
			return
		}
		ok, retry := rl.allow(ip)
		if !ok {
			w.Header().Set("Retry-After", strconv.Itoa(int(retry.Seconds())+1))
			writeError(w, http.StatusTooManyRequests, "rate_limited",
				"too many login attempts — slow down and try again shortly")
			return
		}
		next.ServeHTTP(w, r)
	})
}

// clientIP extracts the host portion of r.RemoteAddr (already resolved from
// X-Forwarded-For by the RealIP middleware upstream).
func clientIP(r *http.Request) string {
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr // no port (already a bare host)
	}
	return host
}

func isLoopback(ip string) bool {
	parsed := net.ParseIP(ip)
	return parsed != nil && parsed.IsLoopback()
}
