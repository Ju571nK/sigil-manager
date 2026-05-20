package triage

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// newTestRepo opens an in-memory SQLite DB and pins the clock to mockNow so
// timestamps in tests are deterministic.
func newTestRepo(t *testing.T) *Repo {
	t.Helper()
	r, err := Open(":memory:")
	require.NoError(t, err)
	t.Cleanup(func() { _ = r.Close() })

	// Tick the clock by 1ms on each call so updated_at moves between updates.
	base := time.Date(2026, 5, 19, 12, 0, 0, 0, time.UTC)
	var tick int64
	r.SetClock(func() time.Time {
		t := base.Add(time.Duration(tick) * time.Millisecond)
		tick++
		return t
	})
	return r
}

const (
	host1 = "5a7c3e91-aaaa-bbbb-cccc-111111111111"
	host2 = "5a7c3e91-aaaa-bbbb-cccc-222222222222"
	ev1   = "019e0cea-0001-7000-8000-000000000001"
	ev2   = "019e0cea-0001-7000-8000-000000000002"
	ev3   = "019e0cea-0001-7000-8000-000000000003"
)

func snap(extra string) json.RawMessage {
	return json.RawMessage(`{"kind":"ai_guard_risk_assessed","tool":"claude_code","score":7.2,"bucket":"critical","extra":"` + extra + `"}`)
}

// -----------------------------------------------------------------------------
// Schema + Open
// -----------------------------------------------------------------------------

func TestOpen_RunsSchemaIdempotently(t *testing.T) {
	r := newTestRepo(t)
	// A second Open against the same in-memory should not blow up. We re-run
	// the schema directly via the underlying DB to simulate a repeat call.
	_, err := r.db.ExecContext(context.Background(), schemaSQL)
	require.NoError(t, err, "schema must be idempotent (CREATE ... IF NOT EXISTS)")
}

func TestUpsert_InsertDefaultsToOpen(t *testing.T) {
	r := newTestRepo(t)

	row, err := r.Upsert(context.Background(), UpsertInput{
		HostID:           host1,
		EventID:          ev1,
		EvidenceSnapshot: snap("hello"),
		Actor:            "admin",
	})
	require.NoError(t, err)
	assert.Equal(t, StatusOpen, row.Status)
	assert.Nil(t, row.Assignee)
	assert.Equal(t, "hello", extractExtra(t, row.EvidenceSnapshot))
	assert.False(t, row.CreatedAt.IsZero())
	assert.Equal(t, row.CreatedAt, row.UpdatedAt)
}

func TestUpsert_RequiresIdentityAndActor(t *testing.T) {
	r := newTestRepo(t)
	cases := []UpsertInput{
		{EventID: ev1, Actor: "admin", EvidenceSnapshot: snap("x")},
		{HostID: host1, Actor: "admin", EvidenceSnapshot: snap("x")},
		{HostID: host1, EventID: ev1, EvidenceSnapshot: snap("x")},
	}
	for _, in := range cases {
		_, err := r.Upsert(context.Background(), in)
		assert.ErrorIs(t, err, ErrMissingFields)
	}
}

func TestUpsert_InsertRequiresEvidenceSnapshot(t *testing.T) {
	r := newTestRepo(t)
	_, err := r.Upsert(context.Background(), UpsertInput{HostID: host1, EventID: ev1, Actor: "admin"})
	assert.ErrorIs(t, err, ErrMissingFields)
}

func TestUpsert_RejectsBadStatus(t *testing.T) {
	r := newTestRepo(t)
	_, err := r.Upsert(context.Background(), UpsertInput{
		HostID:           host1,
		EventID:          ev1,
		Status:           "bogus",
		EvidenceSnapshot: snap("x"),
		Actor:            "admin",
	})
	assert.ErrorIs(t, err, ErrInvalidStatus)
}

// -----------------------------------------------------------------------------
// Update path
// -----------------------------------------------------------------------------

func TestUpsert_UpdateBumpsUpdatedAtAndAppendsLog(t *testing.T) {
	r := newTestRepo(t)
	ctx := context.Background()

	row, err := r.Upsert(ctx, UpsertInput{HostID: host1, EventID: ev1, EvidenceSnapshot: snap("x"), Actor: "admin"})
	require.NoError(t, err)
	createdAt := row.CreatedAt

	updated, err := r.Upsert(ctx, UpsertInput{
		HostID:   host1,
		EventID:  ev1,
		Status:   StatusAcknowledged,
		Assignee: strPtr("alice"),
		Actor:    "admin",
	})
	require.NoError(t, err)
	assert.Equal(t, StatusAcknowledged, updated.Status)
	require.NotNil(t, updated.Assignee)
	assert.Equal(t, "alice", *updated.Assignee)
	assert.True(t, updated.UpdatedAt.After(createdAt), "updated_at must advance")
	assert.Equal(t, createdAt, updated.CreatedAt, "created_at must not change")

	log, err := r.ListLog(ctx, host1, ev1)
	require.NoError(t, err)
	require.Len(t, log, 2, "initial open + status change → 2 log rows")
	assert.Nil(t, log[0].FromStatus, "first log row records initial open (no from)")
	require.NotNil(t, log[0].ToStatus)
	assert.Equal(t, StatusOpen, *log[0].ToStatus)
	require.NotNil(t, log[1].FromStatus)
	assert.Equal(t, StatusOpen, *log[1].FromStatus)
	require.NotNil(t, log[1].ToStatus)
	assert.Equal(t, StatusAcknowledged, *log[1].ToStatus)
}

func TestUpsert_NoStatusChange_NoLogRow(t *testing.T) {
	r := newTestRepo(t)
	ctx := context.Background()

	_, err := r.Upsert(ctx, UpsertInput{HostID: host1, EventID: ev1, EvidenceSnapshot: snap("x"), Actor: "admin"})
	require.NoError(t, err)

	_, err = r.Upsert(ctx, UpsertInput{HostID: host1, EventID: ev1, Assignee: strPtr("bob"), Actor: "admin"})
	require.NoError(t, err)

	log, err := r.ListLog(ctx, host1, ev1)
	require.NoError(t, err)
	assert.Len(t, log, 1, "assignee-only change must not append a status-transition log row")
}

func TestUpsert_ClearAssignee(t *testing.T) {
	r := newTestRepo(t)
	ctx := context.Background()

	_, err := r.Upsert(ctx, UpsertInput{HostID: host1, EventID: ev1, EvidenceSnapshot: snap("x"), Actor: "admin", Assignee: strPtr("alice")})
	require.NoError(t, err)
	_, err = r.Upsert(ctx, UpsertInput{HostID: host1, EventID: ev1, ClearAssignee: true, Actor: "admin"})
	require.NoError(t, err)

	got, err := r.GetByEventKey(ctx, host1, ev1)
	require.NoError(t, err)
	assert.Nil(t, got.Assignee, "ClearAssignee must NULL the column")
}

func TestUpsert_EvidenceSnapshotImmutableOnUpdate(t *testing.T) {
	r := newTestRepo(t)
	ctx := context.Background()

	_, err := r.Upsert(ctx, UpsertInput{HostID: host1, EventID: ev1, EvidenceSnapshot: snap("first"), Actor: "admin"})
	require.NoError(t, err)
	_, err = r.Upsert(ctx, UpsertInput{HostID: host1, EventID: ev1, EvidenceSnapshot: snap("second"), Status: StatusResolved, Actor: "admin"})
	require.NoError(t, err)

	got, err := r.GetByEventKey(ctx, host1, ev1)
	require.NoError(t, err)
	assert.Equal(t, "first", extractExtra(t, got.EvidenceSnapshot),
		"update must NOT overwrite the original snapshot (contract §13 retention mitigation)")
}

// -----------------------------------------------------------------------------
// Get + List
// -----------------------------------------------------------------------------

func TestGetByEventKey_NotFound(t *testing.T) {
	r := newTestRepo(t)
	_, err := r.GetByEventKey(context.Background(), "missing", "missing")
	assert.ErrorIs(t, err, ErrNotFound)
}

func TestListByStatus_FiltersAndPages(t *testing.T) {
	r := newTestRepo(t)
	ctx := context.Background()

	// Three rows across two statuses.
	mustUpsert(t, r, host1, ev1, snap("a"), "admin")
	mustUpsert(t, r, host1, ev2, snap("b"), "admin")
	mustUpsert(t, r, host2, ev3, snap("c"), "admin")

	// Move ev2 to resolved so we can filter it out.
	_, err := r.Upsert(ctx, UpsertInput{HostID: host1, EventID: ev2, Status: StatusResolved, Actor: "admin"})
	require.NoError(t, err)

	openPage, err := r.ListByStatus(ctx, ListOpts{Statuses: []Status{StatusOpen}, Limit: 10})
	require.NoError(t, err)
	require.Len(t, openPage.Rows, 2, "two rows are still open")
	for _, row := range openPage.Rows {
		assert.Equal(t, StatusOpen, row.Status)
	}

	resolvedPage, err := r.ListByStatus(ctx, ListOpts{Statuses: []Status{StatusResolved}, Limit: 10})
	require.NoError(t, err)
	require.Len(t, resolvedPage.Rows, 1)
	assert.Equal(t, ev2, resolvedPage.Rows[0].EventID)
}

func TestListByStatus_OrderedByUpdatedAtDesc(t *testing.T) {
	r := newTestRepo(t)
	ctx := context.Background()

	mustUpsert(t, r, host1, ev1, snap("a"), "admin")
	mustUpsert(t, r, host1, ev2, snap("b"), "admin")
	mustUpsert(t, r, host2, ev3, snap("c"), "admin")

	page, err := r.ListByStatus(ctx, ListOpts{Limit: 10})
	require.NoError(t, err)
	require.Len(t, page.Rows, 3)

	for i := 1; i < len(page.Rows); i++ {
		prev := page.Rows[i-1].UpdatedAt
		curr := page.Rows[i].UpdatedAt
		assert.False(t, curr.After(prev), "rows must be sorted by updated_at desc (i=%d, prev=%v, curr=%v)", i, prev, curr)
	}
}

func TestListByStatus_Pagination(t *testing.T) {
	r := newTestRepo(t)
	ctx := context.Background()

	// Make 7 rows.
	for i := 0; i < 7; i++ {
		mustUpsert(t, r, host1, fmtEvent(i), snap("e"), "admin")
	}

	page1, err := r.ListByStatus(ctx, ListOpts{Limit: 3})
	require.NoError(t, err)
	require.Len(t, page1.Rows, 3)
	require.NotEmpty(t, page1.NextCursor)

	page2, err := r.ListByStatus(ctx, ListOpts{Limit: 3, Cursor: page1.NextCursor})
	require.NoError(t, err)
	require.Len(t, page2.Rows, 3)
	require.NotEmpty(t, page2.NextCursor)

	page3, err := r.ListByStatus(ctx, ListOpts{Limit: 3, Cursor: page2.NextCursor})
	require.NoError(t, err)
	require.Len(t, page3.Rows, 1, "last page should have 1 row, not 3")
	assert.Empty(t, page3.NextCursor, "last page must have empty cursor")

	// Total rows seen = 7, no duplicates.
	seen := map[string]bool{}
	for _, pg := range []ListResult{page1, page2, page3} {
		for _, row := range pg.Rows {
			key := row.HostID + "|" + row.EventID
			assert.False(t, seen[key], "duplicate row in pagination walk: %s", key)
			seen[key] = true
		}
	}
	assert.Len(t, seen, 7)
}

// -----------------------------------------------------------------------------
// Notes
// -----------------------------------------------------------------------------

func TestNotes_AppendAndListChronological(t *testing.T) {
	r := newTestRepo(t)
	ctx := context.Background()

	mustUpsert(t, r, host1, ev1, snap("x"), "admin")

	_, err := r.AppendNote(ctx, host1, ev1, "alice", "first")
	require.NoError(t, err)
	_, err = r.AppendNote(ctx, host1, ev1, "bob", "second")
	require.NoError(t, err)

	notes, err := r.ListNotes(ctx, host1, ev1)
	require.NoError(t, err)
	require.Len(t, notes, 2)
	assert.Equal(t, "first", notes[0].Body)
	assert.Equal(t, "second", notes[1].Body)
}

func TestNotes_RequiresExistingTriageRow(t *testing.T) {
	r := newTestRepo(t)
	_, err := r.AppendNote(context.Background(), host1, ev1, "alice", "hi")
	assert.ErrorIs(t, err, ErrNotFound)
}

func TestNotes_ListReturnsEmptyForNoNotes(t *testing.T) {
	r := newTestRepo(t)
	mustUpsert(t, r, host1, ev1, snap("x"), "admin")

	notes, err := r.ListNotes(context.Background(), host1, ev1)
	require.NoError(t, err)
	assert.Empty(t, notes, "row exists, no notes → empty slice, no error")
}

// -----------------------------------------------------------------------------
// Evidence snapshot fidelity
// -----------------------------------------------------------------------------

func TestEvidenceSnapshot_RoundTripsRawJSON(t *testing.T) {
	r := newTestRepo(t)
	ctx := context.Background()

	// Use a payload with a mix of types and nested structures.
	raw := json.RawMessage(`{"kind":"ai_guard_risk_assessed","tool":"claude_code","scope":{"kind":"project","path":"/abs/foo"},"score":7.2,"bucket":"critical","reasons":[{"kind":"destructive_in_inline_command","pattern":"rm -rf"}],"is_reattestation":false}`)

	in := UpsertInput{HostID: host1, EventID: ev1, EvidenceSnapshot: raw, Actor: "admin"}
	_, err := r.Upsert(ctx, in)
	require.NoError(t, err)

	got, err := r.GetByEventKey(ctx, host1, ev1)
	require.NoError(t, err)

	// Round-trip via Go json to compare semantically (whitespace etc.).
	var want, have map[string]any
	require.NoError(t, json.Unmarshal(raw, &want))
	require.NoError(t, json.Unmarshal(got.EvidenceSnapshot, &have))
	assert.Equal(t, want, have, "evidence_snapshot must round-trip raw JSON unchanged")
}

// -----------------------------------------------------------------------------
// helpers
// -----------------------------------------------------------------------------

func strPtr(s string) *string { return &s }

func mustUpsert(t *testing.T, r *Repo, hostID, eventID string, snapshot json.RawMessage, actor string) Row {
	t.Helper()
	row, err := r.Upsert(context.Background(), UpsertInput{
		HostID:           hostID,
		EventID:          eventID,
		EvidenceSnapshot: snapshot,
		Actor:            actor,
	})
	require.NoError(t, err)
	return row
}

func fmtEvent(i int) string {
	// Format a UUIDv7-shaped id with an incrementing counter in the low bits.
	return "019e0cea-0001-7000-8000-00000000000" + string(rune('0'+i))
}

func extractExtra(t *testing.T, raw json.RawMessage) string {
	t.Helper()
	var probe struct {
		Extra string `json:"extra"`
	}
	require.NoError(t, json.Unmarshal(raw, &probe))
	return probe.Extra
}
