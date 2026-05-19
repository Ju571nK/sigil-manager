package fleet

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const testToken = "test-bearer-secret"

type capturedReq struct {
	Method  string
	Path    string
	Query   url.Values
	Auth    string
	Headers http.Header
}

// stubServer spins up an httptest server backed by a handler the test
// provides. Returns the recorded request alongside an HTTPClient pointed at
// it.
func stubServer(t *testing.T, handler http.HandlerFunc) (*HTTPClient, *capturedReq) {
	t.Helper()
	got := &capturedReq{}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		got.Method = r.Method
		got.Path = r.URL.Path
		got.Query = r.URL.Query()
		got.Auth = r.Header.Get("Authorization")
		got.Headers = r.Header.Clone()
		handler(w, r)
	}))
	t.Cleanup(srv.Close)
	return NewHTTPClient(srv.URL, testToken, 2*time.Second), got
}

func writeJSON(w http.ResponseWriter, status int, body any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(body)
}

// -----------------------------------------------------------------------------
// Happy path — every endpoint
// -----------------------------------------------------------------------------

func TestHTTP_Healthz_NoAuthHeader(t *testing.T) {
	c, got := stubServer(t, func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, 200, map[string]any{"status": "ok", "ts": "2026-05-19T12:00:00Z"})
	})
	out, err := c.Healthz(context.Background())
	require.NoError(t, err)
	assert.Equal(t, "ok", out.Status)
	assert.Empty(t, got.Auth, "Healthz must not send Authorization")
	assert.Equal(t, "/v1/healthz", got.Path)
}

func TestHTTP_Meta_SendsBearer(t *testing.T) {
	c, got := stubServer(t, func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, 200, map[string]any{
			"server_version": "0.5.0",
			"schema_version": 1,
			"ts":             "2026-05-19T12:00:00Z",
			"alerts_definition_default": map[string]any{
				"evidence_kinds":   []string{"ai_guard_risk_assessed"},
				"ai_guard_buckets": []string{"high", "critical"},
				"additional_kinds": []string{},
			},
		})
	})
	out, err := c.Meta(context.Background())
	require.NoError(t, err)
	assert.Equal(t, "0.5.0", out.ServerVersion)
	assert.Equal(t, "Bearer "+testToken, got.Auth)
}

func TestHTTP_EventsQueryParams(t *testing.T) {
	c, got := stubServer(t, func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, 200, map[string]any{"events": []any{}, "next_cursor": nil})
	})
	since := time.Date(2026, 5, 19, 8, 0, 0, 0, time.UTC)
	_, err := c.Events(context.Background(), EventsParams{
		Cursor:           "abc",
		Limit:            200,
		HostIDs:          []string{"a", "b"},
		Since:            since,
		EvidenceKinds:    []string{"foo", "bar"},
		Severity:         []string{"warn"},
		Source:           []string{"file_system"},
		MinAiGuardBucket: "high",
	})
	require.NoError(t, err)
	assert.Equal(t, "abc", got.Query.Get("cursor"))
	assert.Equal(t, "200", got.Query.Get("limit"))
	assert.Equal(t, []string{"a", "b"}, got.Query["host_id"]) // repeatable
	assert.Equal(t, "2026-05-19T08:00:00Z", got.Query.Get("since"))
	assert.Equal(t, "foo,bar", got.Query.Get("evidence_kind"))
	assert.Equal(t, "warn", got.Query.Get("severity"))
	assert.Equal(t, "file_system", got.Query.Get("source"))
	assert.Equal(t, "high", got.Query.Get("min_ai_guard_bucket"))
}

func TestHTTP_EventByID_Decodes(t *testing.T) {
	body := `{
		"schema_version": 1,
		"event_id": "019e0cea-42f1-7ef3-9a6a-1721e98ee2ba",
		"ts": "2026-05-19T12:33:55Z",
		"host_id": "5a7c3e91-aaaa-bbbb-cccc-dddddddddddd",
		"agent_version": "0.4.0",
		"severity": "warn",
		"source": {"kind":"file_system"},
		"subject": {"kind":"path","value":"/etc/passwd"},
		"evidence": {"kind":"ai_guard_risk_assessed","tool":"claude_code","scope":{"kind":"user_global"},"score":7.2,"bucket":"critical","reasons":[],"is_reattestation":false},
		"target_id": null
	}`
	c, got := stubServer(t, func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(body))
	})
	out, err := c.EventByID(context.Background(), "019e0cea-42f1-7ef3-9a6a-1721e98ee2ba")
	require.NoError(t, err)
	assert.Equal(t, "warn", out.Severity)
	assert.Equal(t, "ai_guard_risk_assessed", out.Evidence.Kind)
	assert.True(t, strings.HasSuffix(got.Path, "/019e0cea-42f1-7ef3-9a6a-1721e98ee2ba"))

	ag, err := out.Evidence.AsAiGuard()
	require.NoError(t, err)
	require.NotNil(t, ag)
	assert.Equal(t, "claude_code", ag.Tool)
	assert.Equal(t, "critical", ag.Bucket)
}

// -----------------------------------------------------------------------------
// Error mapping
// -----------------------------------------------------------------------------

func TestHTTP_401_ReturnsErrUnauthorized(t *testing.T) {
	c, _ := stubServer(t, func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, 401, map[string]any{"error": map[string]any{"code": "unauthorized", "message": "nope"}})
	})
	_, err := c.Meta(context.Background())
	require.Error(t, err)
	assert.True(t, errors.Is(err, ErrUnauthorized))
}

func TestHTTP_404_OnMeta_ReturnsReadAPIDisabled(t *testing.T) {
	c, _ := stubServer(t, func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(404)
	})
	_, err := c.Meta(context.Background())
	require.Error(t, err)
	assert.True(t, errors.Is(err, ErrReadAPIDisabled))
}

func TestHTTP_404_OnEventByID_ReturnsNotFound(t *testing.T) {
	c, _ := stubServer(t, func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(404)
	})
	_, err := c.EventByID(context.Background(), "missing")
	require.Error(t, err)
	assert.True(t, errors.Is(err, ErrNotFound))
	assert.False(t, errors.Is(err, ErrReadAPIDisabled),
		"id-lookup 404 must surface as ErrNotFound, not ErrReadAPIDisabled")
}

func TestHTTP_404_OnFleetHostByID_ReturnsNotFound(t *testing.T) {
	c, _ := stubServer(t, func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(404)
	})
	_, err := c.FleetHostByID(context.Background(), "missing")
	require.Error(t, err)
	assert.True(t, errors.Is(err, ErrNotFound))
}

func TestHTTP_503_ParsesRetryAfter(t *testing.T) {
	c, _ := stubServer(t, func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Retry-After", "7")
		writeJSON(w, 503, map[string]any{"error": map[string]any{"code": "service_unavailable", "message": "rebuilding"}})
	})
	_, err := c.Meta(context.Background())
	require.Error(t, err)
	assert.True(t, errors.Is(err, ErrServiceUnavailable))
	var sue *ServiceUnavailableError
	require.True(t, errors.As(err, &sue))
	assert.Equal(t, 7*time.Second, sue.RetryAfter)
}

func TestHTTP_503_DefaultRetryAfterWhenMissing(t *testing.T) {
	c, _ := stubServer(t, func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, 503, map[string]any{"error": map[string]any{"code": "service_unavailable", "message": "rebuilding"}})
	})
	_, err := c.Meta(context.Background())
	var sue *ServiceUnavailableError
	require.True(t, errors.As(err, &sue))
	assert.Equal(t, 5*time.Second, sue.RetryAfter, "missing Retry-After defaults to 5s per F15 doc")
}

func TestHTTP_400_ReturnsAPIError(t *testing.T) {
	c, _ := stubServer(t, func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, 400, map[string]any{"error": map[string]any{"code": "invalid_query", "message": "bad cursor"}})
	})
	_, err := c.Events(context.Background(), EventsParams{Cursor: "garbage"})
	require.Error(t, err)
	var apiErr *APIError
	require.True(t, errors.As(err, &apiErr))
	assert.Equal(t, 400, apiErr.Status)
	assert.Equal(t, "invalid_query", apiErr.Code)
}

// -----------------------------------------------------------------------------
// Retry layer
// -----------------------------------------------------------------------------

func TestRetry_SucceedsAfter503(t *testing.T) {
	var calls int
	c, _ := stubServer(t, func(w http.ResponseWriter, _ *http.Request) {
		calls++
		if calls == 1 {
			w.Header().Set("Retry-After", "0")
			writeJSON(w, 503, map[string]any{"error": map[string]any{"code": "service_unavailable", "message": "rebuilding"}})
			return
		}
		writeJSON(w, 200, map[string]any{
			"server_version": "0.5.0", "schema_version": 1, "ts": "2026-05-19T12:00:00Z",
			"alerts_definition_default": map[string]any{"evidence_kinds": []string{}, "ai_guard_buckets": []string{}, "additional_kinds": []string{}},
		})
	})

	policy := RetryPolicy{MaxAttempts: 3, BaseDelay: 1 * time.Millisecond, MaxDelay: 10 * time.Millisecond}
	out, err := Do(context.Background(), policy, func(ctx context.Context) (*Meta, error) {
		return c.Meta(ctx)
	})
	require.NoError(t, err)
	assert.Equal(t, "0.5.0", out.ServerVersion)
	assert.Equal(t, 2, calls)
}

func TestRetry_DoesNotRetry401(t *testing.T) {
	var calls int
	c, _ := stubServer(t, func(w http.ResponseWriter, _ *http.Request) {
		calls++
		writeJSON(w, 401, map[string]any{"error": map[string]any{"code": "unauthorized", "message": "nope"}})
	})

	_, err := Do(context.Background(), DefaultRetryPolicy(), func(ctx context.Context) (*Meta, error) {
		return c.Meta(ctx)
	})
	require.Error(t, err)
	assert.True(t, errors.Is(err, ErrUnauthorized))
	assert.Equal(t, 1, calls, "401 is a config error, must not retry")
}

func TestRetry_DoesNotRetry404(t *testing.T) {
	var calls int
	c, _ := stubServer(t, func(w http.ResponseWriter, _ *http.Request) {
		calls++
		w.WriteHeader(404)
	})

	_, err := Do(context.Background(), DefaultRetryPolicy(), func(ctx context.Context) (*Meta, error) {
		return c.Meta(ctx)
	})
	require.Error(t, err)
	assert.True(t, errors.Is(err, ErrReadAPIDisabled))
	assert.Equal(t, 1, calls)
}

func TestRetry_ExhaustsAttempts(t *testing.T) {
	var calls int
	c, _ := stubServer(t, func(w http.ResponseWriter, _ *http.Request) {
		calls++
		w.Header().Set("Retry-After", "0")
		writeJSON(w, 503, map[string]any{"error": map[string]any{"code": "service_unavailable", "message": "rebuilding"}})
	})

	policy := RetryPolicy{MaxAttempts: 3, BaseDelay: 1 * time.Millisecond, MaxDelay: 5 * time.Millisecond}
	_, err := Do(context.Background(), policy, func(ctx context.Context) (*Meta, error) {
		return c.Meta(ctx)
	})
	require.Error(t, err)
	assert.True(t, errors.Is(err, ErrServiceUnavailable))
	assert.Equal(t, 3, calls)
}

func TestRetry_RespectsContextCancel(t *testing.T) {
	c, _ := stubServer(t, func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Retry-After", "60")
		writeJSON(w, 503, map[string]any{"error": map[string]any{"code": "service_unavailable", "message": "rebuilding"}})
	})

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer cancel()

	policy := RetryPolicy{MaxAttempts: 5, BaseDelay: 50 * time.Millisecond, MaxDelay: 50 * time.Millisecond}
	_, err := Do(ctx, policy, func(ctx context.Context) (*Meta, error) {
		return c.Meta(ctx)
	})
	require.Error(t, err)
	assert.True(t,
		errors.Is(err, context.DeadlineExceeded) ||
			errors.Is(err, context.Canceled) ||
			errors.Is(err, ErrServiceUnavailable),
		"got %v", err)
}

// -----------------------------------------------------------------------------
// Misc: Healthz timeout shouldn't include Authorization, Meta should
// -----------------------------------------------------------------------------

func TestHTTP_HostsQuery_BuildsCorrectFilters(t *testing.T) {
	c, got := stubServer(t, func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, 200, map[string]any{"hosts": []any{}, "next_cursor": nil, "total_estimated": 0})
	})
	_, err := c.FleetHosts(context.Background(), HostsParams{
		Limit:  50,
		Status: []string{"healthy", "stale"},
		Bucket: []string{"high", "critical"},
		Sort:   "risk",
	})
	require.NoError(t, err)
	assert.Equal(t, "50", got.Query.Get("limit"))
	assert.Equal(t, "healthy,stale", got.Query.Get("status"))
	assert.Equal(t, "high,critical", got.Query.Get("bucket"))
	assert.Equal(t, "risk", got.Query.Get("sort"))
}

func TestHTTP_RiskQuery_BuildsCorrectFilters(t *testing.T) {
	c, got := stubServer(t, func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, 200, map[string]any{"rows": []any{}, "next_cursor": nil})
	})
	_, err := c.FleetRisk(context.Background(), RiskParams{
		Limit:     25,
		Tool:      []string{"claude_code", "codex"},
		MinBucket: "high",
	})
	require.NoError(t, err)
	assert.Equal(t, "25", got.Query.Get("limit"))
	assert.Equal(t, "claude_code,codex", got.Query.Get("tool"))
	assert.Equal(t, "high", got.Query.Get("min_bucket"))
}
