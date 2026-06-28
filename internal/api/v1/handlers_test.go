package v1

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/Ju571nK/sigil-manager/internal/auth"
	"github.com/Ju571nK/sigil-manager/internal/fleet"
	"github.com/Ju571nK/sigil-manager/internal/triage"
)

// testHarness wires a real Mock fleet client + in-memory triage repo + JWT
// signer behind an httptest server so tests can hit the routes via HTTP.
type testHarness struct {
	t      *testing.T
	srv    *httptest.Server
	signer *auth.Signer
	repo   *triage.Repo
	mock   *fleet.MockClient
	v1     *Server
}

// Bcrypt hash of "test-password" — generated once so tests don't bcrypt at
// every run. (cost 4 to keep hash generation cheap if regenerating.)
const (
	testAdmin    = "admin"
	testPassword = "test-password"
	// Generated via bcrypt.GenerateFromPassword([]byte("test-password"), 10).
	testAdminHash = "$2a$10$wfv94g2jgl.5WmoEZHXfw.WMQSDL6HzwEq0avcAT1QmtrqBWUf7wu"

	testJWTSecret = "0123456789abcdefghijklmnopqrstuv" // 32 bytes
)

func newHarness(t *testing.T) *testHarness {
	t.Helper()

	signer, err := auth.NewSigner(testJWTSecret, 12*time.Hour)
	require.NoError(t, err)

	repo, err := triage.Open(":memory:")
	require.NoError(t, err)
	t.Cleanup(func() { _ = repo.Close() })

	mockSeed := time.Date(2026, 5, 19, 12, 0, 0, 0, time.UTC)
	mock := fleet.NewMock(mockSeed)

	v1 := &Server{
		Fleet:  mock,
		Triage: repo,
		Signer: signer,
		Auth: AuthConfig{
			AdminUsername:       testAdmin,
			AdminPasswordBcrypt: testAdminHash,
			CookieSecure:        false,
		},
	}

	srv := httptest.NewServer(v1.Routes())
	t.Cleanup(srv.Close)

	return &testHarness{t: t, srv: srv, signer: signer, repo: repo, mock: mock, v1: v1}
}

// loginCookie performs POST /auth/login and returns the session cookie so
// follow-up requests can attach it.
func (h *testHarness) loginCookie() *http.Cookie {
	h.t.Helper()
	body, _ := json.Marshal(loginRequest{Username: testAdmin, Password: testPassword})
	req, _ := http.NewRequest(http.MethodPost, h.srv.URL+"/auth/login", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	require.NoError(h.t, err)
	defer func() { _ = resp.Body.Close() }()
	require.Equal(h.t, http.StatusOK, resp.StatusCode, "login must succeed for the canned admin creds")
	for _, c := range resp.Cookies() {
		if c.Name == CookieName {
			return c
		}
	}
	h.t.Fatalf("login response did not set %q cookie", CookieName)
	return nil
}

// do builds a request, attaches the cookie if non-nil, and returns the
// recorder-equivalent: status code + decoded JSON body.
func (h *testHarness) do(method, path string, body any, cookie *http.Cookie) (int, json.RawMessage, http.Header) {
	h.t.Helper()
	var rdr io.Reader
	if body != nil {
		b, _ := json.Marshal(body)
		rdr = bytes.NewReader(b)
	}
	req, _ := http.NewRequest(method, h.srv.URL+path, rdr)
	if rdr != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if cookie != nil {
		req.AddCookie(cookie)
	}
	resp, err := http.DefaultClient.Do(req)
	require.NoError(h.t, err)
	defer func() { _ = resp.Body.Close() }()
	raw, err := io.ReadAll(resp.Body)
	require.NoError(h.t, err)
	return resp.StatusCode, json.RawMessage(raw), resp.Header
}

// -----------------------------------------------------------------------------
// /auth
// -----------------------------------------------------------------------------

func TestLogin_WrongPassword(t *testing.T) {
	h := newHarness(t)
	code, body, _ := h.do(http.MethodPost, "/auth/login", loginRequest{
		Username: testAdmin, Password: "wrong",
	}, nil)
	assert.Equal(t, http.StatusUnauthorized, code)
	assert.Contains(t, string(body), "invalid_credentials")
}

func TestLogin_WrongUsername(t *testing.T) {
	h := newHarness(t)
	code, body, _ := h.do(http.MethodPost, "/auth/login", loginRequest{
		Username: "not-admin", Password: testPassword,
	}, nil)
	assert.Equal(t, http.StatusUnauthorized, code,
		"wrong username must give the SAME response shape as wrong password")
	assert.Contains(t, string(body), "invalid_credentials")
}

func TestLogin_HappyPath_SetsCookie(t *testing.T) {
	h := newHarness(t)
	code, body, hdr := h.do(http.MethodPost, "/auth/login", loginRequest{
		Username: testAdmin, Password: testPassword,
	}, nil)
	require.Equal(t, http.StatusOK, code, "body=%s", string(body))

	var resp loginResponse
	require.NoError(t, json.Unmarshal(body, &resp))
	assert.Equal(t, testAdmin, resp.Username)
	assert.True(t, resp.ExpiresAt.After(time.Now()), "expires_at must be in the future")

	setCookie := hdr.Get("Set-Cookie")
	assert.Contains(t, setCookie, CookieName+"=")
	assert.Contains(t, strings.ToLower(setCookie), "httponly")
}

func TestMe_RequiresCookie(t *testing.T) {
	h := newHarness(t)
	code, body, _ := h.do(http.MethodGet, "/auth/me", nil, nil)
	assert.Equal(t, http.StatusUnauthorized, code)
	assert.Contains(t, string(body), "unauthorized")
}

func TestMe_ReturnsSubjectAndExpiry(t *testing.T) {
	h := newHarness(t)
	c := h.loginCookie()
	code, body, _ := h.do(http.MethodGet, "/auth/me", nil, c)
	require.Equal(t, http.StatusOK, code)

	var resp meResponse
	require.NoError(t, json.Unmarshal(body, &resp))
	assert.Equal(t, testAdmin, resp.Username)
	assert.True(t, resp.ExpiresAt.After(time.Now()))
}

func TestLogout_ClearsCookie(t *testing.T) {
	h := newHarness(t)
	c := h.loginCookie()
	code, _, hdr := h.do(http.MethodPost, "/auth/logout", nil, c)
	assert.Equal(t, http.StatusOK, code)
	assert.Contains(t, hdr.Get("Set-Cookie"), CookieName+"=;")
	assert.Contains(t, strings.ToLower(hdr.Get("Set-Cookie")), "max-age=0")
}

// -----------------------------------------------------------------------------
// /fleet
// -----------------------------------------------------------------------------

func TestFleet_RequiresAuth(t *testing.T) {
	h := newHarness(t)
	code, _, _ := h.do(http.MethodGet, "/fleet/meta", nil, nil)
	assert.Equal(t, http.StatusUnauthorized, code)
}

func TestFleet_Meta_Passthrough(t *testing.T) {
	h := newHarness(t)
	c := h.loginCookie()
	code, body, _ := h.do(http.MethodGet, "/fleet/meta", nil, c)
	require.Equal(t, http.StatusOK, code)

	var meta fleet.Meta
	require.NoError(t, json.Unmarshal(body, &meta))
	assert.Equal(t, 1, meta.SchemaVersion)
	assert.Contains(t, meta.AlertsDefinitionDefault.EvidenceKinds, "ai_guard_risk_assessed")

	// License block passes through verbatim (contract §14.9.3 / issue #7).
	require.NotNil(t, meta.License, "license block should pass through")
	assert.Equal(t, "ok", meta.License.State)
	assert.Equal(t, 50, meta.License.EffectiveMaxHosts)
	assert.GreaterOrEqual(t, meta.License.CurrentHostCount, 0)
}

func TestFleet_Events_QueryPassthrough(t *testing.T) {
	h := newHarness(t)
	c := h.loginCookie()
	code, body, _ := h.do(http.MethodGet,
		"/fleet/events?limit=10&evidence_kind=ai_guard_risk_assessed&min_ai_guard_bucket=high", nil, c)
	require.Equal(t, http.StatusOK, code)

	var resp eventsResponse
	require.NoError(t, json.Unmarshal(body, &resp))
	require.NotEmpty(t, resp.Events)
	for _, ev := range resp.Events {
		assert.Equal(t, "ai_guard_risk_assessed", ev.Evidence.Kind)
		ag, err := ev.Evidence.AsAiGuard()
		require.NoError(t, err)
		require.NotNil(t, ag)
		assert.Contains(t, []string{"high", "critical"}, ag.Bucket)
	}
}

func TestFleet_Events_JoinsTriage(t *testing.T) {
	h := newHarness(t)
	c := h.loginCookie()

	// Fetch an event and create a triage row for it.
	code, body, _ := h.do(http.MethodGet, "/fleet/events?limit=1", nil, c)
	require.Equal(t, http.StatusOK, code)
	var page eventsResponse
	require.NoError(t, json.Unmarshal(body, &page))
	require.Len(t, page.Events, 1)
	assert.Nil(t, page.Events[0].Triage, "no triage row yet → triage:null")

	ev := page.Events[0]
	upsertCode, upsertBody, _ := h.do(http.MethodPost, "/triage/upsert", upsertRequest{
		HostID: ev.HostID, EventID: ev.EventID,
		Status:           triage.StatusAcknowledged,
		EvidenceSnapshot: json.RawMessage(`{"snap":"ok"}`),
	}, c)
	require.Equal(t, http.StatusOK, upsertCode, "body=%s", string(upsertBody))

	// Refetch the same event; triage block should now populate.
	code, body, _ = h.do(http.MethodGet, "/fleet/events?limit=1", nil, c)
	require.Equal(t, http.StatusOK, code)
	require.NoError(t, json.Unmarshal(body, &page))
	require.NotNil(t, page.Events[0].Triage, "triage row exists → triage object populated")
	assert.Equal(t, triage.StatusAcknowledged, page.Events[0].Triage.Status)
}

func TestFleet_EventByID(t *testing.T) {
	h := newHarness(t)
	c := h.loginCookie()
	code, body, _ := h.do(http.MethodGet, "/fleet/events?limit=1", nil, c)
	require.Equal(t, http.StatusOK, code)
	var page eventsResponse
	require.NoError(t, json.Unmarshal(body, &page))
	require.Len(t, page.Events, 1)

	id := page.Events[0].EventID
	code, body, _ = h.do(http.MethodGet, "/fleet/events/"+id, nil, c)
	require.Equal(t, http.StatusOK, code)
	var single eventWithTriage
	require.NoError(t, json.Unmarshal(body, &single))
	assert.Equal(t, id, single.EventID)
}

func TestFleet_EventByID_NotFound(t *testing.T) {
	h := newHarness(t)
	c := h.loginCookie()
	code, body, _ := h.do(http.MethodGet, "/fleet/events/00000000-0000-0000-0000-000000000000", nil, c)
	assert.Equal(t, http.StatusNotFound, code)
	assert.Contains(t, string(body), "not_found")
}

// -----------------------------------------------------------------------------
// /fleet/risk + /fleet/compliance
// -----------------------------------------------------------------------------

func TestHandleFleetRisk_PassThroughSortedRows(t *testing.T) {
	h := newHarness(t)
	c := h.loginCookie()

	code, body, _ := h.do(http.MethodGet, "/fleet/risk?limit=100", nil, c)
	require.Equal(t, http.StatusOK, code)

	var resp struct {
		Rows []struct {
			HostID            string  `json:"host_id"`
			Score             float64 `json:"score"`
			Bucket            string  `json:"bucket"`
			OpenAlertCount24h int     `json:"open_alert_count_24h"`
		} `json:"rows"`
		NextCursor *string `json:"next_cursor"`
	}
	require.NoError(t, json.Unmarshal(body, &resp))
	require.NotEmpty(t, resp.Rows, "mock seeds >=1 host with AI guard risk")
	// §5.5: rows are sorted by score desc.
	for i := 1; i < len(resp.Rows); i++ {
		require.GreaterOrEqual(t, resp.Rows[i-1].Score, resp.Rows[i].Score)
	}
}

func TestHandleFleetRisk_BadLimitIs400(t *testing.T) {
	h := newHarness(t)
	c := h.loginCookie()
	code, _, _ := h.do(http.MethodGet, "/fleet/risk?limit=abc", nil, c)
	require.Equal(t, http.StatusBadRequest, code)
}

func TestHandleFleetRisk_Unauthed401(t *testing.T) {
	h := newHarness(t)
	code, _, _ := h.do(http.MethodGet, "/fleet/risk", nil, nil)
	require.Equal(t, http.StatusUnauthorized, code)
}

func TestHandleFleetCompliance_PassThroughRawSignals(t *testing.T) {
	h := newHarness(t)
	c := h.loginCookie()

	code, body, _ := h.do(http.MethodGet, "/fleet/compliance?limit=100", nil, c)
	require.Equal(t, http.StatusOK, code)

	var resp struct {
		Rows []struct {
			HostID               string `json:"host_id"`
			VersionDrift         int    `json:"version_drift"`
			PolicyExpiredActive  bool   `json:"policy_expired_active"`
			SignatureFailures24h int    `json:"signature_failures_24h"`
		} `json:"rows"`
	}
	require.NoError(t, json.Unmarshal(body, &resp))
	require.NotEmpty(t, resp.Rows)
	// Server exposes raw signals only — no derived status field (F13).
	require.NotContains(t, string(body), "compliance_score")
	require.NotContains(t, string(body), `"status"`)
}

func TestFleet_HostByID_Passthrough(t *testing.T) {
	h := newHarness(t)
	c := h.loginCookie()
	// alice-mbp — the mock seeds host_meta + current_risk + ai_guard for it.
	code, body, _ := h.do(http.MethodGet, "/fleet/hosts/5a7c3e91-aaaa-bbbb-cccc-111111111111", nil, c)
	require.Equal(t, http.StatusOK, code)

	var hd struct {
		HostID   string          `json:"host_id"`
		Hostname string          `json:"hostname"`
		HostMeta json.RawMessage `json:"host_meta"`
		AiGuard  *struct {
			ByTool map[string]json.RawMessage `json:"by_tool"`
		} `json:"ai_guard"`
	}
	require.NoError(t, json.Unmarshal(body, &hd))
	assert.Equal(t, "5a7c3e91-aaaa-bbbb-cccc-111111111111", hd.HostID)
	assert.Equal(t, "alice-mbp", hd.Hostname)
	assert.NotNil(t, hd.HostMeta, "alice has a HostMetaSnapshot")
	require.NotNil(t, hd.AiGuard)
	assert.Contains(t, hd.AiGuard.ByTool, "claude_code")
}

func TestFleet_HostByID_NotFound(t *testing.T) {
	h := newHarness(t)
	c := h.loginCookie()
	code, body, _ := h.do(http.MethodGet, "/fleet/hosts/00000000-0000-0000-0000-000000000000", nil, c)
	require.Equal(t, http.StatusNotFound, code)
	assert.Contains(t, string(body), "not_found")
}

// -----------------------------------------------------------------------------
// /triage
// -----------------------------------------------------------------------------

func TestTriage_UpsertRequiresAuth(t *testing.T) {
	h := newHarness(t)
	code, _, _ := h.do(http.MethodPost, "/triage/upsert", upsertRequest{
		HostID: "h1", EventID: "e1", Status: triage.StatusOpen,
		EvidenceSnapshot: json.RawMessage(`{"x":1}`),
	}, nil)
	assert.Equal(t, http.StatusUnauthorized, code)
}

func TestTriage_StateMachine(t *testing.T) {
	h := newHarness(t)
	c := h.loginCookie()

	steps := []triage.Status{
		triage.StatusOpen,
		triage.StatusAcknowledged,
		triage.StatusInvestigating,
		triage.StatusResolved,
		triage.StatusInvestigating, // backwards transitions allowed in v1
		triage.StatusOpen,
	}
	for i, s := range steps {
		body := upsertRequest{
			HostID: "h1", EventID: "e1", Status: s,
		}
		if i == 0 {
			body.EvidenceSnapshot = json.RawMessage(`{"first":true}`)
		}
		code, respBody, _ := h.do(http.MethodPost, "/triage/upsert", body, c)
		require.Equal(t, http.StatusOK, code, "step %d → %s, body=%s", i, s, string(respBody))
	}

	// Final state must reflect the last step.
	code, body, _ := h.do(http.MethodGet, "/triage/h1/e1", nil, c)
	require.Equal(t, http.StatusOK, code)
	var got getResponse
	require.NoError(t, json.Unmarshal(body, &got))
	assert.Equal(t, triage.StatusOpen, got.Row.Status)
	// log should record open → acknowledged → investigating → resolved → investigating → open
	// plus the initial open insert log row = 6 transitions total (first row is the initial open).
	assert.Equal(t, 6, len(got.Log))
}

func TestTriage_NoteAndGet(t *testing.T) {
	h := newHarness(t)
	c := h.loginCookie()

	// Upsert first so the FK is satisfied.
	code, _, _ := h.do(http.MethodPost, "/triage/upsert", upsertRequest{
		HostID: "h2", EventID: "e2", Status: triage.StatusOpen,
		EvidenceSnapshot: json.RawMessage(`{"x":2}`),
	}, c)
	require.Equal(t, http.StatusOK, code)

	code, body, _ := h.do(http.MethodPost, "/triage/note", noteRequest{
		HostID: "h2", EventID: "e2", Body: "first note",
	}, c)
	require.Equal(t, http.StatusOK, code)
	var note noteResponse
	require.NoError(t, json.Unmarshal(body, &note))
	assert.Equal(t, testAdmin, note.Author, "author must come from the auth context")
	assert.Equal(t, "first note", note.Body)

	code, body, _ = h.do(http.MethodGet, "/triage/h2/e2", nil, c)
	require.Equal(t, http.StatusOK, code)
	var got getResponse
	require.NoError(t, json.Unmarshal(body, &got))
	require.Len(t, got.Notes, 1)
	assert.Equal(t, "first note", got.Notes[0].Body)
}

func TestTriage_RejectsBadStatus(t *testing.T) {
	h := newHarness(t)
	c := h.loginCookie()
	code, body, _ := h.do(http.MethodPost, "/triage/upsert", map[string]any{
		"host_id":           "h3",
		"event_id":          "e3",
		"status":            "bogus",
		"evidence_snapshot": json.RawMessage(`{"x":3}`),
	}, c)
	assert.Equal(t, http.StatusBadRequest, code)
	assert.Contains(t, string(body), "invalid_query")
}

// -----------------------------------------------------------------------------
// Expired cookie
// -----------------------------------------------------------------------------

func TestRequireAuth_ExpiredCookie_SurfacesSessionExpired(t *testing.T) {
	h := newHarness(t)

	// Forge a token whose iat/exp are far in the past so the real server's
	// signer (real wall clock) sees it as expired.
	forgedSigner, err := auth.NewSigner(testJWTSecret, 1*time.Second)
	require.NoError(t, err)
	forgedSigner.SetClock(func() time.Time { return time.Now().Add(-25 * time.Hour) })
	tok, _, err := forgedSigner.Sign(testAdmin)
	require.NoError(t, err)

	c := &http.Cookie{Name: CookieName, Value: tok}
	code, body, _ := h.do(http.MethodGet, "/auth/me", nil, c)
	assert.Equal(t, http.StatusUnauthorized, code)
	assert.Contains(t, string(body), "session_expired", "expired tokens get a distinct error code")
}

// -----------------------------------------------------------------------------
// Upstream retry wiring (withFleetRetry)
// -----------------------------------------------------------------------------

// flakyRiskClient embeds the Mock but fails the first FleetRisk call with a
// retryable 503, proving the handler's withFleetRetry wiring actually retries.
// (The retry layer existed but was never wired into handlers — bug hunt M2.)
type flakyRiskClient struct {
	*fleet.MockClient
	riskCalls int
}

func (f *flakyRiskClient) FleetRisk(ctx context.Context, p fleet.RiskParams) (*fleet.RiskPage, error) {
	f.riskCalls++
	if f.riskCalls == 1 {
		// 1ms Retry-After keeps the test fast while exercising the real backoff.
		return nil, &fleet.ServiceUnavailableError{RetryAfter: time.Millisecond}
	}
	return f.MockClient.FleetRisk(ctx, p)
}

func TestFleet_Risk_RetriesTransient503(t *testing.T) {
	h := newHarness(t)
	flaky := &flakyRiskClient{MockClient: h.mock}
	h.v1.Fleet = flaky // handlers read s.Fleet at call time
	c := h.loginCookie()

	code, _, _ := h.do(http.MethodGet, "/fleet/risk", nil, c)
	assert.Equal(t, http.StatusOK, code, "a transient 503 must be retried, not surfaced")
	assert.Equal(t, 2, flaky.riskCalls, "expected one failed attempt + one retry")
}
