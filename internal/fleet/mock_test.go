package fleet

import (
	"context"
	"sort"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// mockSeed is a fixed timestamp used by every Mock test so cursor walks and
// timestamp comparisons are deterministic.
var mockSeed = time.Date(2026, 5, 19, 12, 0, 0, 0, time.UTC)

func newTestMock(t *testing.T) *MockClient {
	t.Helper()
	return NewMock(mockSeed)
}

// -----------------------------------------------------------------------------
// Hosts fixture coverage
// -----------------------------------------------------------------------------

func TestMock_HostsHaveStatusMix(t *testing.T) {
	m := newTestMock(t)
	page, err := m.FleetHosts(context.Background(), HostsParams{})
	require.NoError(t, err)
	require.Len(t, page.Hosts, 5, "fixture promises 5 hosts")

	statuses := map[string]int{}
	for _, h := range page.Hosts {
		statuses[h.Status]++
	}
	assert.Greater(t, statuses["healthy"], 0)
	assert.Greater(t, statuses["stale"], 0)
	assert.Greater(t, statuses["disconnected"], 0)
}

func TestMock_OneHostHasFullHostMeta_AndOneHasNull(t *testing.T) {
	m := newTestMock(t)
	page, err := m.FleetHosts(context.Background(), HostsParams{})
	require.NoError(t, err)

	var withMeta, withoutMeta int
	for _, h := range page.Hosts {
		d, err := m.FleetHostByID(context.Background(), h.HostID)
		require.NoError(t, err)
		if d.HostMeta != nil {
			withMeta++
		} else {
			withoutMeta++
		}
	}
	assert.GreaterOrEqual(t, withMeta, 1, "at least one host has a HostMetaSnapshot")
	assert.GreaterOrEqual(t, withoutMeta, 1, "at least one host has null host_meta")
}

func TestMock_HostnameNullWhenNoHostMeta(t *testing.T) {
	m := newTestMock(t)
	page, err := m.FleetHosts(context.Background(), HostsParams{})
	require.NoError(t, err)
	var ok bool
	for _, h := range page.Hosts {
		d, _ := m.FleetHostByID(context.Background(), h.HostID)
		if d.HostMeta == nil && h.Hostname == nil {
			ok = true
			break
		}
	}
	assert.True(t, ok, "host with null HostMetaSnapshot must have hostname=nil per §5.3")
}

// -----------------------------------------------------------------------------
// Event fixture coverage (§14.5 aggregate checklist)
// -----------------------------------------------------------------------------

func TestMock_32EventsSpan24h(t *testing.T) {
	m := newTestMock(t)
	page, err := m.Events(context.Background(), EventsParams{Limit: 1000})
	require.NoError(t, err)
	require.Len(t, page.Events, 32)

	oldest := page.Events[len(page.Events)-1].TS
	newest := page.Events[0].TS
	assert.True(t, newest.Sub(oldest) <= 24*time.Hour, "events must fit in trailing 24h window")
	assert.True(t, newest.Sub(oldest) >= 1*time.Hour, "events shouldn't be clustered in a single hour")
}

func TestMock_AtLeast5HighOrCriticalAiGuardEvents(t *testing.T) {
	m := newTestMock(t)
	page, _ := m.Events(context.Background(), EventsParams{Limit: 1000})
	hits := 0
	for _, ev := range page.Events {
		if ev.Evidence.Kind != "ai_guard_risk_assessed" {
			continue
		}
		ag, err := ev.Evidence.AsAiGuard()
		require.NoError(t, err)
		require.NotNil(t, ag)
		if ag.Bucket == "high" || ag.Bucket == "critical" {
			hits++
		}
	}
	assert.GreaterOrEqual(t, hits, 5, "Plan 02 T4 fixture must include >= 5 high/critical AI guard events")
}

func TestMock_AllSixToolValues(t *testing.T) {
	m := newTestMock(t)
	page, _ := m.Events(context.Background(), EventsParams{Limit: 1000})
	seen := map[string]bool{}
	for _, ev := range page.Events {
		if ag, _ := ev.Evidence.AsAiGuard(); ag != nil {
			seen[ag.Tool] = true
		}
	}
	// gemini + cursor added per contract §14.7 (producer Phase 3b.7).
	for _, want := range []string{"claude_code", "codex", "claude_desktop", "continue_dev", "gemini", "cursor"} {
		assert.True(t, seen[want], "fixtures must include tool=%q (§14.5 aggregate checklist)", want)
	}
}

func TestMock_AllThreeScopeKinds(t *testing.T) {
	m := newTestMock(t)
	page, _ := m.Events(context.Background(), EventsParams{Limit: 1000})
	seen := map[string]bool{}
	for _, ev := range page.Events {
		ag, _ := ev.Evidence.AsAiGuard()
		if ag == nil {
			continue
		}
		scope, err := DecodeScope(ag.Scope)
		require.NoError(t, err)
		require.NotNil(t, scope)
		seen[scope.Kind] = true
		switch scope.Kind {
		case "project":
			assert.NotEmpty(t, scope.Path, "project scope must include path")
		case "application":
			assert.NotEmpty(t, scope.App, "application scope must include app")
		}
	}
	for _, want := range []string{"user_global", "project", "application"} {
		assert.True(t, seen[want], "fixtures must include scope.kind=%q (§14.5)", want)
	}
}

// -----------------------------------------------------------------------------
// Pagination
// -----------------------------------------------------------------------------

func TestMock_EventsPaginate(t *testing.T) {
	m := newTestMock(t)

	var collected []Event
	cursor := ""
	for i := 0; i < 5; i++ { // bound the walk
		page, err := m.Events(context.Background(), EventsParams{Limit: 10, Cursor: cursor})
		require.NoError(t, err)
		collected = append(collected, page.Events...)
		if page.NextCursor == nil {
			break
		}
		cursor = *page.NextCursor
	}
	require.Len(t, collected, 32, "walk must terminate after 32 events")

	// Reverse-chronological order across pages.
	for i := 1; i < len(collected); i++ {
		assert.False(t, collected[i].EventID > collected[i-1].EventID,
			"events must be sorted by event_id desc — i=%d", i)
	}

	// First page is exactly 10, cursor advances.
	page1, _ := m.Events(context.Background(), EventsParams{Limit: 10})
	require.Len(t, page1.Events, 10)
	require.NotNil(t, page1.NextCursor)
}

func TestMock_HostsPaginate(t *testing.T) {
	m := newTestMock(t)
	page, err := m.FleetHosts(context.Background(), HostsParams{Limit: 2})
	require.NoError(t, err)
	require.Len(t, page.Hosts, 2)
	require.NotNil(t, page.NextCursor)

	page2, err := m.FleetHosts(context.Background(), HostsParams{Limit: 10, Cursor: *page.NextCursor})
	require.NoError(t, err)
	assert.Equal(t, 3, len(page2.Hosts))
	assert.Nil(t, page2.NextCursor)
}

// -----------------------------------------------------------------------------
// Filters
// -----------------------------------------------------------------------------

func TestMock_EventsFilter_ByHostID(t *testing.T) {
	m := newTestMock(t)
	page, err := m.Events(context.Background(), EventsParams{
		Limit:   1000,
		HostIDs: []string{"5a7c3e91-aaaa-bbbb-cccc-111111111111"},
	})
	require.NoError(t, err)
	require.NotEmpty(t, page.Events)
	for _, ev := range page.Events {
		assert.Equal(t, "5a7c3e91-aaaa-bbbb-cccc-111111111111", ev.HostID)
	}
}

func TestMock_EventsFilter_ByEvidenceKind(t *testing.T) {
	m := newTestMock(t)
	page, err := m.Events(context.Background(), EventsParams{
		Limit:         1000,
		EvidenceKinds: []string{"ai_guard_risk_assessed"},
	})
	require.NoError(t, err)
	require.NotEmpty(t, page.Events)
	for _, ev := range page.Events {
		assert.Equal(t, "ai_guard_risk_assessed", ev.Evidence.Kind)
	}
}

func TestMock_EventsFilter_MinAiGuardBucket(t *testing.T) {
	m := newTestMock(t)
	page, err := m.Events(context.Background(), EventsParams{
		Limit:            1000,
		EvidenceKinds:    []string{"ai_guard_risk_assessed"},
		MinAiGuardBucket: "high",
	})
	require.NoError(t, err)
	require.NotEmpty(t, page.Events)
	for _, ev := range page.Events {
		ag, err := ev.Evidence.AsAiGuard()
		require.NoError(t, err)
		assert.True(t, bucketAtLeast(ag.Bucket, "high"),
			"min_ai_guard_bucket=high should exclude %s", ag.Bucket)
	}
}

func TestMock_RiskSortedDescByScore(t *testing.T) {
	m := newTestMock(t)
	page, err := m.FleetRisk(context.Background(), RiskParams{Limit: 100})
	require.NoError(t, err)
	require.NotEmpty(t, page.Rows)
	scores := make([]float64, len(page.Rows))
	for i, r := range page.Rows {
		scores[i] = r.Score
	}
	require.True(t, sort.SliceIsSorted(page.Rows, func(i, j int) bool { return page.Rows[i].Score > page.Rows[j].Score }),
		"risk rows must be sorted by max_score desc; got %v", scores)
}

func TestMock_RiskFilter_MinBucket(t *testing.T) {
	m := newTestMock(t)
	page, err := m.FleetRisk(context.Background(), RiskParams{MinBucket: "high"})
	require.NoError(t, err)
	for _, r := range page.Rows {
		assert.True(t, bucketAtLeast(r.Bucket, "high"))
	}
}

// -----------------------------------------------------------------------------
// Lookups + compliance
// -----------------------------------------------------------------------------

func TestMock_EventByID_Found(t *testing.T) {
	m := newTestMock(t)
	page, _ := m.Events(context.Background(), EventsParams{Limit: 1})
	require.NotEmpty(t, page.Events)
	id := page.Events[0].EventID

	got, err := m.EventByID(context.Background(), id)
	require.NoError(t, err)
	assert.Equal(t, id, got.EventID)
}

func TestMock_EventByID_NotFound(t *testing.T) {
	m := newTestMock(t)
	_, err := m.EventByID(context.Background(), "00000000-0000-0000-0000-000000000000")
	require.ErrorIs(t, err, ErrNotFound)
}

func TestMock_FleetHostByID_NotFound(t *testing.T) {
	m := newTestMock(t)
	_, err := m.FleetHostByID(context.Background(), "00000000-0000-0000-0000-000000000000")
	require.ErrorIs(t, err, ErrNotFound)
}

func TestMock_ComplianceRowsPerHost(t *testing.T) {
	m := newTestMock(t)
	page, err := m.FleetCompliance(context.Background(), ComplianceParams{Limit: 100})
	require.NoError(t, err)
	require.Len(t, page.Rows, 5)
	for _, r := range page.Rows {
		assert.Equal(t, 18, r.ServerCurrentPolicyVersion, "policy_meta.policy_version drives the server-current value")
	}
}

func TestMock_ComplianceCoversAllRawSignalStates(t *testing.T) {
	m := newTestMock(t)
	page, err := m.FleetCompliance(context.Background(), ComplianceParams{Limit: 100})
	require.NoError(t, err)

	var sawInSync, sawDrift, sawExpired, sawSigFail bool
	for _, row := range page.Rows {
		switch {
		case row.PolicyExpiredActive:
			sawExpired = true
		case row.SignatureFailures24h > 0:
			sawSigFail = true
		case row.VersionDrift > 0:
			sawDrift = true
		default:
			sawInSync = true
		}
	}
	require.True(t, sawInSync, "need a host with no drift/expiry/sig-failures")
	require.True(t, sawDrift, "need a host with version_drift > 0")
	require.True(t, sawExpired, "need a host with policy_expired_active")
	require.True(t, sawSigFail, "need a host with signature_failures_24h > 0")
}

// -----------------------------------------------------------------------------
// Healthz / Meta
// -----------------------------------------------------------------------------

func TestMock_HealthzOK(t *testing.T) {
	m := newTestMock(t)
	h, err := m.Healthz(context.Background())
	require.NoError(t, err)
	assert.Equal(t, "ok", h.Status)
	assert.Equal(t, mockSeed, h.TS)
}

func TestMock_MetaAlertsDefinition(t *testing.T) {
	m := newTestMock(t)
	meta, err := m.Meta(context.Background())
	require.NoError(t, err)
	assert.Equal(t, 1, meta.SchemaVersion)
	assert.Contains(t, meta.AlertsDefinitionDefault.EvidenceKinds, "ai_guard_risk_assessed")
	assert.Contains(t, meta.AlertsDefinitionDefault.AiGuardBuckets, "high")
	assert.Contains(t, meta.AlertsDefinitionDefault.AiGuardBuckets, "critical")
}
