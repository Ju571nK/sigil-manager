package fleet

import (
	"context"
	"errors"
	"math/rand/v2"
	"net"
	"time"
)

// RetryPolicy controls bounded retry around idempotent fleet GETs.
//
// Per Plan 02 T3 step 5: retry ONLY on [ErrServiceUnavailable] (the boot
// rebuild window, F15) and transient network errors (timeout / connection
// reset). 401 / 404 / 400 are configuration errors — caller fixes those, no
// retry.
type RetryPolicy struct {
	MaxAttempts int           // default 3 when zero
	BaseDelay   time.Duration // default 200ms when zero (used for transient net errors)
	MaxDelay    time.Duration // default 5s when zero
}

// DefaultRetryPolicy returns the policy Plan 02 ships: 3 attempts, 200ms
// base / 5s max delay, exponential backoff with jitter.
func DefaultRetryPolicy() RetryPolicy {
	return RetryPolicy{MaxAttempts: 3, BaseDelay: 200 * time.Millisecond, MaxDelay: 5 * time.Second}
}

// Do wraps fn with bounded retry. fn must be idempotent (only fleet GETs
// satisfy that — never wrap writes). Returns the last error if all attempts
// fail.
//
// Generic over T so callers can wrap a single fleet method without losing
// the typed return:
//
//	page, err := fleet.Do(ctx, fleet.DefaultRetryPolicy(), func(ctx context.Context) (*fleet.EventsPage, error) {
//	    return client.Events(ctx, params)
//	})
func Do[T any](ctx context.Context, p RetryPolicy, fn func(context.Context) (*T, error)) (*T, error) {
	if p.MaxAttempts <= 0 {
		p.MaxAttempts = 3
	}
	if p.BaseDelay <= 0 {
		p.BaseDelay = 200 * time.Millisecond
	}
	if p.MaxDelay <= 0 {
		p.MaxDelay = 5 * time.Second
	}

	var lastErr error
	for attempt := 1; attempt <= p.MaxAttempts; attempt++ {
		out, err := fn(ctx)
		if err == nil {
			return out, nil
		}
		lastErr = err
		if !isRetryable(err) || attempt == p.MaxAttempts {
			return nil, err
		}
		if err := sleepFor(ctx, p.backoffFor(attempt, err)); err != nil {
			return nil, err
		}
	}
	return nil, lastErr
}

// isRetryable returns true for 503 and transient network errors. Everything
// else (401, 404, 400, decode errors, ctx canceled) is terminal.
func isRetryable(err error) bool {
	if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		return false
	}
	if errors.Is(err, ErrServiceUnavailable) {
		return true
	}
	var netErr net.Error
	if errors.As(err, &netErr) && netErr.Timeout() {
		return true
	}
	return false
}

// backoffFor picks the wait before attempt+1. For 503 responses we honor
// the parsed Retry-After exactly; for transient errors we use jittered
// exponential backoff bounded by MaxDelay.
func (p RetryPolicy) backoffFor(attempt int, err error) time.Duration {
	var sue *ServiceUnavailableError
	if errors.As(err, &sue) {
		d := sue.RetryAfter
		if d <= 0 || d > p.MaxDelay {
			d = p.MaxDelay
		}
		return d
	}
	d := p.BaseDelay * time.Duration(1<<uint(attempt-1))
	if d > p.MaxDelay {
		d = p.MaxDelay
	}
	jitter := time.Duration(rand.Int64N(int64(p.BaseDelay)))
	return d + jitter
}

// sleepFor sleeps for d unless ctx fires first, in which case it returns
// the ctx error.
func sleepFor(ctx context.Context, d time.Duration) error {
	t := time.NewTimer(d)
	defer t.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-t.C:
		return nil
	}
}
