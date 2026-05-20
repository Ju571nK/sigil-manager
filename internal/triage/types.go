package triage

import (
	"encoding/json"
	"errors"
	"time"
)

// Status is the lifecycle stage of a triage row. Matches the schema CHECK
// constraint; bad values are rejected at the DB layer.
type Status string

// Triage lifecycle statuses. Match the schema CHECK constraint exactly.
const (
	StatusOpen          Status = "open"
	StatusAcknowledged  Status = "acknowledged"
	StatusInvestigating Status = "investigating"
	StatusResolved      Status = "resolved"
)

// AllStatuses lists the values the schema CHECK constraint accepts.
var AllStatuses = []Status{StatusOpen, StatusAcknowledged, StatusInvestigating, StatusResolved}

// IsValid reports whether s is one of the accepted statuses.
func (s Status) IsValid() bool {
	for _, v := range AllStatuses {
		if v == s {
			return true
		}
	}
	return false
}

// Row is one record in the `triage` table. EvidenceSnapshot holds the raw
// JSON of the source event at the moment triage was opened — see
// schema.go's note about contract §13 retention mitigation.
type Row struct {
	HostID           string
	EventID          string
	Status           Status
	Assignee         *string         // null = unassigned
	EvidenceSnapshot json.RawMessage // raw event JSON; never re-shaped
	CreatedAt        time.Time
	UpdatedAt        time.Time
}

// Note is one row of `triage_notes`.
type Note struct {
	ID        int64
	HostID    string
	EventID   string
	Author    string
	Body      string
	CreatedAt time.Time
}

// LogEntry is one row of `triage_log`. FromStatus is nil for the initial
// open (no prior state).
type LogEntry struct {
	ID         int64
	HostID     string
	EventID    string
	Actor      string
	FromStatus *Status
	ToStatus   *Status
	At         time.Time
}

// UpsertInput is the payload [Repo.Upsert] accepts. Fields not present
// (zero values for pointers) mean "leave unchanged on update"; for a brand
// new row the missing fields default to (Status=open, Assignee=nil).
type UpsertInput struct {
	HostID           string
	EventID          string
	Status           Status          // empty string = "no change on update"; required on insert
	Assignee         *string         // nil = no change on update; "" cannot clear (use ClearAssignee)
	ClearAssignee    bool            // explicit nil-out for update
	EvidenceSnapshot json.RawMessage // required on insert; ignored on update
	Actor            string          // who's making the change; recorded in triage_log
}

// ListOpts filters [Repo.ListByStatus]. Limit clamps to [1, 1000] like the
// fleet API does (contract §6.2). Cursor is the trailing
// `<updated_at_rfc3339>|<host_id>|<event_id>` triple of the prior page so
// pagination is stable under concurrent updates.
type ListOpts struct {
	Statuses []Status
	Limit    int
	Cursor   string
}

// ListResult is one page of [Repo.ListByStatus]. NextCursor is empty when
// the walk is complete.
type ListResult struct {
	Rows       []Row
	NextCursor string
}

// ErrNotFound is returned by [Repo.GetByEventKey] and [Repo.ListNotes] when
// the (host_id, event_id) key isn't present.
var ErrNotFound = errors.New("triage: not found")

// ErrInvalidStatus is returned when an UpsertInput.Status is set but not
// one of [AllStatuses].
var ErrInvalidStatus = errors.New("triage: invalid status")

// ErrMissingFields is returned for an insert without the required fields
// (HostID, EventID, EvidenceSnapshot, Actor, Status).
var ErrMissingFields = errors.New("triage: missing required fields")
