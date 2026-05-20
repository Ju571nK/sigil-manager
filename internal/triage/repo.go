// Package triage owns sigil-manager's local SQLite store for alert
// triage state — `(status, assignee, notes, log)` keyed by
// `(host_id, event_id)` per the fleet API contract §5.8. The store is
// consumer-local; producer never reads from or writes to it. See
// schema.go for the schema and the contract §13 retention rationale.
package triage

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	_ "modernc.org/sqlite" // pure-Go SQLite driver, no cgo
)

// Repo is the triage store. Safe for concurrent use; the underlying
// [sql.DB] handles connection pooling.
type Repo struct {
	db  *sql.DB
	now func() time.Time // injected so tests can pin timestamps
}

// Open opens (or creates) a SQLite DB at path and runs the schema. Pass
// `:memory:` (or `file::memory:?cache=shared`) for tests.
//
// modernc.org/sqlite uses the driver name `sqlite` (NOT `sqlite3`).
func Open(path string) (*Repo, error) {
	dsn := path
	if !strings.HasPrefix(dsn, "file:") && dsn != ":memory:" {
		// Plain file path → wrap in file: URI so we can append query params later.
		dsn = "file:" + dsn + "?_pragma=foreign_keys(1)&_pragma=journal_mode(WAL)&_pragma=busy_timeout(5000)"
	}
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, fmt.Errorf("triage: open %q: %w", path, err)
	}
	// SQLite is single-writer; cap connections so we don't fight ourselves.
	db.SetMaxOpenConns(1)
	if _, err := db.ExecContext(context.Background(), schemaSQL); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("triage: apply schema: %w", err)
	}
	return &Repo{db: db, now: func() time.Time { return time.Now().UTC() }}, nil
}

// Close releases the underlying DB. Safe to call multiple times.
func (r *Repo) Close() error {
	if r == nil || r.db == nil {
		return nil
	}
	return r.db.Close()
}

// SetClock overrides the timestamp source. Tests use this to make
// `created_at` / `updated_at` predictable; production never calls it.
func (r *Repo) SetClock(now func() time.Time) { r.now = now }

// -----------------------------------------------------------------------------
// Upsert + state transitions
// -----------------------------------------------------------------------------

// Upsert creates or updates a triage row. On insert, every field except
// Assignee is required (StatusOpen if Status is empty). On update, only
// fields explicitly set are changed:
//   - in.Status == "" → keep existing status
//   - in.Assignee == nil && !in.ClearAssignee → keep existing assignee
//   - in.ClearAssignee == true → set assignee to NULL
//
// Every successful state transition appends a row to `triage_log`.
// EvidenceSnapshot is ONLY honored on insert; updating an existing row
// never changes the snapshot (the source event is immutable).
func (r *Repo) Upsert(ctx context.Context, in UpsertInput) (Row, error) {
	if in.HostID == "" || in.EventID == "" || in.Actor == "" {
		return Row{}, fmt.Errorf("%w: HostID/EventID/Actor", ErrMissingFields)
	}
	if in.Status != "" && !in.Status.IsValid() {
		return Row{}, fmt.Errorf("%w: %q", ErrInvalidStatus, in.Status)
	}

	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return Row{}, fmt.Errorf("triage: begin: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	existing, err := getRowTx(ctx, tx, in.HostID, in.EventID)
	if err != nil && !errors.Is(err, ErrNotFound) {
		return Row{}, err
	}
	now := r.now().UTC()

	if errors.Is(err, ErrNotFound) {
		// Insert path.
		if len(in.EvidenceSnapshot) == 0 {
			return Row{}, fmt.Errorf("%w: EvidenceSnapshot required on insert", ErrMissingFields)
		}
		status := in.Status
		if status == "" {
			status = StatusOpen
		}
		row := Row{
			HostID:           in.HostID,
			EventID:          in.EventID,
			Status:           status,
			Assignee:         in.Assignee,
			EvidenceSnapshot: cloneJSON(in.EvidenceSnapshot),
			CreatedAt:        now,
			UpdatedAt:        now,
		}
		if _, err := tx.ExecContext(ctx, `
			INSERT INTO triage (host_id, event_id, status, assignee, evidence_snapshot, created_at, updated_at)
			VALUES (?, ?, ?, ?, ?, ?, ?)
		`, row.HostID, row.EventID, string(row.Status), row.Assignee, string(row.EvidenceSnapshot), formatTS(row.CreatedAt), formatTS(row.UpdatedAt)); err != nil {
			return Row{}, fmt.Errorf("triage: insert: %w", err)
		}
		if err := appendLogTx(ctx, tx, row.HostID, row.EventID, in.Actor, nil, &row.Status, now); err != nil {
			return Row{}, err
		}
		if err := tx.Commit(); err != nil {
			return Row{}, fmt.Errorf("triage: commit: %w", err)
		}
		return row, nil
	}

	// Update path.
	updated := existing
	updated.UpdatedAt = now
	var fromStatus *Status
	if in.Status != "" && in.Status != existing.Status {
		s := existing.Status
		fromStatus = &s
		updated.Status = in.Status
	}
	if in.ClearAssignee {
		updated.Assignee = nil
	} else if in.Assignee != nil {
		v := *in.Assignee
		updated.Assignee = &v
	}

	if _, err := tx.ExecContext(ctx, `
		UPDATE triage SET status = ?, assignee = ?, updated_at = ?
		WHERE host_id = ? AND event_id = ?
	`, string(updated.Status), updated.Assignee, formatTS(updated.UpdatedAt), updated.HostID, updated.EventID); err != nil {
		return Row{}, fmt.Errorf("triage: update: %w", err)
	}
	if fromStatus != nil {
		to := updated.Status
		if err := appendLogTx(ctx, tx, updated.HostID, updated.EventID, in.Actor, fromStatus, &to, now); err != nil {
			return Row{}, err
		}
	}
	if err := tx.Commit(); err != nil {
		return Row{}, fmt.Errorf("triage: commit: %w", err)
	}
	return updated, nil
}

// -----------------------------------------------------------------------------
// Reads
// -----------------------------------------------------------------------------

// GetByEventKey returns the row for (host_id, event_id) or [ErrNotFound].
func (r *Repo) GetByEventKey(ctx context.Context, hostID, eventID string) (Row, error) {
	return getRow(ctx, r.db, hostID, eventID)
}

// ListByStatus pages triage rows ordered by updated_at desc, then by
// (host_id, event_id) for tiebreakers (so the cursor is unambiguous).
//
// Cursor encoding is opaque to callers but trivially `updated_at|host|event`
// so the server can resume at the boundary without a stored cursor table.
func (r *Repo) ListByStatus(ctx context.Context, opts ListOpts) (ListResult, error) {
	limit := clampLimit(opts.Limit)
	statuses := opts.Statuses
	if len(statuses) == 0 {
		statuses = AllStatuses
	}
	for _, s := range statuses {
		if !s.IsValid() {
			return ListResult{}, fmt.Errorf("%w: %q", ErrInvalidStatus, s)
		}
	}

	// Build `?, ?, ...` placeholders for the IN clause.
	placeholders := strings.Repeat("?,", len(statuses))
	placeholders = strings.TrimSuffix(placeholders, ",")
	args := make([]any, 0, len(statuses)+4)
	for _, s := range statuses {
		args = append(args, string(s))
	}

	var cursorClause string
	if opts.Cursor != "" {
		curTS, curHost, curEvent, err := decodeCursor(opts.Cursor)
		if err != nil {
			return ListResult{}, err
		}
		cursorClause = ` AND (
			updated_at < ?
			OR (updated_at = ? AND (host_id, event_id) > (?, ?))
		)`
		args = append(args, formatTS(curTS), formatTS(curTS), curHost, curEvent)
	}

	q := `
		SELECT host_id, event_id, status, assignee, evidence_snapshot, created_at, updated_at
		FROM triage
		WHERE status IN (` + placeholders + `)` + cursorClause + `
		ORDER BY updated_at DESC, host_id ASC, event_id ASC
		LIMIT ?
	`
	args = append(args, limit+1) // fetch one extra so we can tell if there's more

	rows, err := r.db.QueryContext(ctx, q, args...)
	if err != nil {
		return ListResult{}, fmt.Errorf("triage: list: %w", err)
	}
	defer func() { _ = rows.Close() }()

	out := make([]Row, 0, limit)
	for rows.Next() {
		row, err := scanRow(rows)
		if err != nil {
			return ListResult{}, err
		}
		out = append(out, row)
	}
	if err := rows.Err(); err != nil {
		return ListResult{}, fmt.Errorf("triage: list scan: %w", err)
	}

	var nextCursor string
	if len(out) > limit {
		out = out[:limit]
		last := out[len(out)-1]
		nextCursor = encodeCursor(last.UpdatedAt, last.HostID, last.EventID)
	}
	return ListResult{Rows: out, NextCursor: nextCursor}, nil
}

// -----------------------------------------------------------------------------
// Notes
// -----------------------------------------------------------------------------

// AppendNote attaches a note to an existing triage row. Returns
// [ErrNotFound] if the triage row doesn't exist (the FK would also fire,
// but we surface a typed error).
func (r *Repo) AppendNote(ctx context.Context, hostID, eventID, author, body string) (Note, error) {
	if hostID == "" || eventID == "" || author == "" || body == "" {
		return Note{}, fmt.Errorf("%w: hostID/eventID/author/body", ErrMissingFields)
	}
	if _, err := r.GetByEventKey(ctx, hostID, eventID); err != nil {
		return Note{}, err
	}
	now := r.now().UTC()
	res, err := r.db.ExecContext(ctx, `
		INSERT INTO triage_notes (host_id, event_id, author, body, created_at)
		VALUES (?, ?, ?, ?, ?)
	`, hostID, eventID, author, body, formatTS(now))
	if err != nil {
		return Note{}, fmt.Errorf("triage: insert note: %w", err)
	}
	id, _ := res.LastInsertId()
	return Note{ID: id, HostID: hostID, EventID: eventID, Author: author, Body: body, CreatedAt: now}, nil
}

// ListNotes returns notes for (host_id, event_id) in chronological order.
// Returns ([]Note{}, nil) when the triage row exists but has no notes;
// [ErrNotFound] when the triage row doesn't exist.
func (r *Repo) ListNotes(ctx context.Context, hostID, eventID string) ([]Note, error) {
	if _, err := r.GetByEventKey(ctx, hostID, eventID); err != nil {
		return nil, err
	}
	rows, err := r.db.QueryContext(ctx, `
		SELECT id, host_id, event_id, author, body, created_at
		FROM triage_notes
		WHERE host_id = ? AND event_id = ?
		ORDER BY created_at ASC, id ASC
	`, hostID, eventID)
	if err != nil {
		return nil, fmt.Errorf("triage: list notes: %w", err)
	}
	defer func() { _ = rows.Close() }()

	out := make([]Note, 0)
	for rows.Next() {
		var n Note
		var createdAt string
		if err := rows.Scan(&n.ID, &n.HostID, &n.EventID, &n.Author, &n.Body, &createdAt); err != nil {
			return nil, fmt.Errorf("triage: scan note: %w", err)
		}
		n.CreatedAt = parseTS(createdAt)
		out = append(out, n)
	}
	return out, rows.Err()
}

// ListLog returns the state-transition log for (host_id, event_id) in
// chronological order. Useful for the slide-over's audit display.
func (r *Repo) ListLog(ctx context.Context, hostID, eventID string) ([]LogEntry, error) {
	if _, err := r.GetByEventKey(ctx, hostID, eventID); err != nil {
		return nil, err
	}
	rows, err := r.db.QueryContext(ctx, `
		SELECT id, host_id, event_id, actor, from_status, to_status, at
		FROM triage_log
		WHERE host_id = ? AND event_id = ?
		ORDER BY at ASC, id ASC
	`, hostID, eventID)
	if err != nil {
		return nil, fmt.Errorf("triage: list log: %w", err)
	}
	defer func() { _ = rows.Close() }()

	out := make([]LogEntry, 0)
	for rows.Next() {
		var e LogEntry
		var from, to sql.NullString
		var atStr string
		if err := rows.Scan(&e.ID, &e.HostID, &e.EventID, &e.Actor, &from, &to, &atStr); err != nil {
			return nil, fmt.Errorf("triage: scan log: %w", err)
		}
		if from.Valid {
			s := Status(from.String)
			e.FromStatus = &s
		}
		if to.Valid {
			s := Status(to.String)
			e.ToStatus = &s
		}
		e.At = parseTS(atStr)
		out = append(out, e)
	}
	return out, rows.Err()
}

// -----------------------------------------------------------------------------
// Internal helpers
// -----------------------------------------------------------------------------

type rowScanner interface {
	Scan(dest ...any) error
}

func scanRow(s rowScanner) (Row, error) {
	var r Row
	var assignee sql.NullString
	var snapshot string
	var createdAt, updatedAt string
	if err := s.Scan(&r.HostID, &r.EventID, (*string)(&r.Status), &assignee, &snapshot, &createdAt, &updatedAt); err != nil {
		return Row{}, fmt.Errorf("triage: scan: %w", err)
	}
	if assignee.Valid {
		v := assignee.String
		r.Assignee = &v
	}
	r.EvidenceSnapshot = json.RawMessage(snapshot)
	r.CreatedAt = parseTS(createdAt)
	r.UpdatedAt = parseTS(updatedAt)
	return r, nil
}

func getRow(ctx context.Context, db *sql.DB, hostID, eventID string) (Row, error) {
	row := db.QueryRowContext(ctx, `
		SELECT host_id, event_id, status, assignee, evidence_snapshot, created_at, updated_at
		FROM triage
		WHERE host_id = ? AND event_id = ?
	`, hostID, eventID)
	out, err := scanRow(row)
	if errors.Is(err, sql.ErrNoRows) {
		return Row{}, ErrNotFound
	}
	return out, err
}

func getRowTx(ctx context.Context, tx *sql.Tx, hostID, eventID string) (Row, error) {
	row := tx.QueryRowContext(ctx, `
		SELECT host_id, event_id, status, assignee, evidence_snapshot, created_at, updated_at
		FROM triage
		WHERE host_id = ? AND event_id = ?
	`, hostID, eventID)
	out, err := scanRow(row)
	if errors.Is(err, sql.ErrNoRows) {
		return Row{}, ErrNotFound
	}
	return out, err
}

func appendLogTx(ctx context.Context, tx *sql.Tx, hostID, eventID, actor string, from, to *Status, at time.Time) error {
	var fromArg, toArg any
	if from != nil {
		fromArg = string(*from)
	}
	if to != nil {
		toArg = string(*to)
	}
	if _, err := tx.ExecContext(ctx, `
		INSERT INTO triage_log (host_id, event_id, actor, from_status, to_status, at)
		VALUES (?, ?, ?, ?, ?, ?)
	`, hostID, eventID, actor, fromArg, toArg, formatTS(at)); err != nil {
		return fmt.Errorf("triage: insert log: %w", err)
	}
	return nil
}

func cloneJSON(in json.RawMessage) json.RawMessage {
	out := make(json.RawMessage, len(in))
	copy(out, in)
	return out
}

func clampLimit(n int) int {
	if n <= 0 {
		return 50
	}
	if n > 1000 {
		return 1000
	}
	return n
}

// Cursor format: "<rfc3339-nano>|<host>|<event>". Opaque to callers, but
// implementation lives here so [Repo.ListByStatus] can advance.
const cursorSep = "|"

func encodeCursor(t time.Time, host, event string) string {
	return formatTS(t) + cursorSep + host + cursorSep + event
}

func decodeCursor(s string) (time.Time, string, string, error) {
	parts := strings.SplitN(s, cursorSep, 3)
	if len(parts) != 3 {
		return time.Time{}, "", "", fmt.Errorf("triage: malformed cursor")
	}
	t, err := time.Parse(time.RFC3339Nano, parts[0])
	if err != nil {
		return time.Time{}, "", "", fmt.Errorf("triage: malformed cursor ts: %w", err)
	}
	return t, parts[1], parts[2], nil
}

// tsLayout is RFC3339 with always-9-digit fractional seconds. Using the
// zero-suppressing RFC3339Nano broke ORDER BY in SQLite: a time with no
// fractional part (`12:00:00Z`) lex-compares AFTER one with a fractional
// part (`12:00:00.001Z`) because `.` (0x2E) < `Z` (0x5A). The fixed-width
// format makes lexicographic order match chronological order.
const tsLayout = "2006-01-02T15:04:05.000000000Z07:00"

func formatTS(t time.Time) string { return t.UTC().Format(tsLayout) }

func parseTS(s string) time.Time {
	// time.RFC3339Nano accepts both fixed and zero-suppressed forms.
	t, err := time.Parse(time.RFC3339Nano, s)
	if err != nil {
		return time.Time{}
	}
	return t.UTC()
}
