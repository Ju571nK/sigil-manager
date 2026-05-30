package v1

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestRateLimiter_AllowsUpToMaxThenBlocks(t *testing.T) {
	rl := newRateLimiter(3, time.Minute)
	for i := 1; i <= 3; i++ {
		ok, _ := rl.allow("203.0.113.7")
		assert.True(t, ok, "attempt %d should be allowed", i)
	}
	ok, retry := rl.allow("203.0.113.7")
	assert.False(t, ok, "4th attempt exceeds max=3")
	assert.Greater(t, retry, time.Duration(0), "blocked attempt reports time until reset")
}

func TestRateLimiter_WindowResets(t *testing.T) {
	now := time.Date(2026, 5, 30, 12, 0, 0, 0, time.UTC)
	rl := newRateLimiter(1, time.Minute)
	rl.now = func() time.Time { return now }

	ok, _ := rl.allow("203.0.113.7")
	assert.True(t, ok)
	ok, _ = rl.allow("203.0.113.7")
	assert.False(t, ok, "second attempt in the same window is blocked")

	now = now.Add(61 * time.Second) // window elapsed
	ok, _ = rl.allow("203.0.113.7")
	assert.True(t, ok, "a fresh window allows again")
}

func TestRateLimiter_PerIPIsolation(t *testing.T) {
	rl := newRateLimiter(1, time.Minute)
	ok, _ := rl.allow("203.0.113.1")
	assert.True(t, ok)
	ok, _ = rl.allow("203.0.113.2")
	assert.True(t, ok, "a different IP has its own window")
}

func TestRateLimiter_Middleware_ExemptsLoopback(t *testing.T) {
	rl := newRateLimiter(1, time.Minute)
	h := rl.middleware(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	// Loopback is never limited, even past the max.
	for i := 0; i < 5; i++ {
		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodPost, "/auth/login", nil)
		req.RemoteAddr = "127.0.0.1:54321"
		h.ServeHTTP(rec, req)
		assert.Equal(t, http.StatusOK, rec.Code, "loopback attempt %d must pass", i+1)
	}
}

func TestRateLimiter_Middleware_BlocksRemoteWith429(t *testing.T) {
	rl := newRateLimiter(1, time.Minute)
	h := rl.middleware(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	call := func() *httptest.ResponseRecorder {
		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodPost, "/auth/login", nil)
		req.RemoteAddr = "203.0.113.9:40000"
		h.ServeHTTP(rec, req)
		return rec
	}
	assert.Equal(t, http.StatusOK, call().Code, "first remote attempt allowed")
	blocked := call()
	assert.Equal(t, http.StatusTooManyRequests, blocked.Code, "second remote attempt over max=1")
	assert.NotEmpty(t, blocked.Header().Get("Retry-After"), "429 carries Retry-After")
	assert.Contains(t, blocked.Body.String(), "rate_limited")
}
