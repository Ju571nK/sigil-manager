package v1

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/Ju571nK/sigil-manager/internal/httputil"
	"github.com/Ju571nK/sigil-manager/internal/triage"
)

// upsertRequest is the POST body for /api/v1/triage/upsert.
//
// Status is required on initial creation; updates may omit it to mean "no
// status change". EvidenceSnapshot is REQUIRED on first upsert so the row
// can outlive producer-side JSONL retention (contract §13).
type upsertRequest struct {
	HostID           string          `json:"host_id"`
	EventID          string          `json:"event_id"`
	Status           triage.Status   `json:"status"`
	Assignee         *string         `json:"assignee"`
	ClearAssignee    bool            `json:"clear_assignee"`
	EvidenceSnapshot json.RawMessage `json:"evidence_snapshot"`
}

// rowResponse is the wire shape of a triage.Row.
type rowResponse struct {
	HostID           string          `json:"host_id"`
	EventID          string          `json:"event_id"`
	Status           triage.Status   `json:"status"`
	Assignee         *string         `json:"assignee"`
	EvidenceSnapshot json.RawMessage `json:"evidence_snapshot"`
	CreatedAt        string          `json:"created_at"`
	UpdatedAt        string          `json:"updated_at"`
}

type noteRequest struct {
	HostID  string `json:"host_id"`
	EventID string `json:"event_id"`
	Body    string `json:"body"`
}

type noteResponse struct {
	ID        int64  `json:"id"`
	HostID    string `json:"host_id"`
	EventID   string `json:"event_id"`
	Author    string `json:"author"`
	Body      string `json:"body"`
	CreatedAt string `json:"created_at"`
}

type getResponse struct {
	Row   rowResponse    `json:"row"`
	Notes []noteResponse `json:"notes"`
	Log   []logResponse  `json:"log"`
}

type logResponse struct {
	ID         int64   `json:"id"`
	Actor      string  `json:"actor"`
	FromStatus *string `json:"from_status"`
	ToStatus   *string `json:"to_status"`
	At         string  `json:"at"`
}

// handleTriageUpsert creates or updates the triage row for (host_id,
// event_id). The actor is the authenticated admin username from context.
func (s *Server) handleTriageUpsert(w http.ResponseWriter, r *http.Request) {
	var req upsertRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_query", "malformed JSON body")
		return
	}
	if req.HostID == "" || req.EventID == "" {
		writeError(w, http.StatusBadRequest, "invalid_query", "host_id and event_id are required")
		return
	}
	if req.Status != "" && !req.Status.IsValid() {
		writeError(w, http.StatusBadRequest, "invalid_query", "status must be one of: open, acknowledged, investigating, resolved")
		return
	}

	subject := Subject(r.Context())
	row, err := s.Triage.Upsert(r.Context(), triage.UpsertInput{
		HostID:           req.HostID,
		EventID:          req.EventID,
		Status:           req.Status,
		Assignee:         req.Assignee,
		ClearAssignee:    req.ClearAssignee,
		EvidenceSnapshot: req.EvidenceSnapshot,
		Actor:            subject,
	})
	if err != nil {
		mapTriageErr(w, err)
		return
	}
	httputil.WriteJSON(w, http.StatusOK, rowToResponse(row))
}

// handleTriageNote appends a note to an existing triage row.
func (s *Server) handleTriageNote(w http.ResponseWriter, r *http.Request) {
	var req noteRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_query", "malformed JSON body")
		return
	}
	if req.HostID == "" || req.EventID == "" || req.Body == "" {
		writeError(w, http.StatusBadRequest, "invalid_query", "host_id, event_id, body are required")
		return
	}

	subject := Subject(r.Context())
	note, err := s.Triage.AppendNote(r.Context(), req.HostID, req.EventID, subject, req.Body)
	if err != nil {
		mapTriageErr(w, err)
		return
	}
	httputil.WriteJSON(w, http.StatusOK, noteResponse{
		ID:        note.ID,
		HostID:    note.HostID,
		EventID:   note.EventID,
		Author:    note.Author,
		Body:      note.Body,
		CreatedAt: note.CreatedAt.Format(rfc3339Nano),
	})
}

// handleTriageGet returns the full triage row + notes + state-transition
// log for one (host_id, event_id).
func (s *Server) handleTriageGet(w http.ResponseWriter, r *http.Request) {
	hostID := chi.URLParam(r, "host_id")
	eventID := chi.URLParam(r, "event_id")
	if hostID == "" || eventID == "" {
		writeError(w, http.StatusBadRequest, "invalid_query", "host_id and event_id required")
		return
	}
	row, err := s.Triage.GetByEventKey(r.Context(), hostID, eventID)
	if err != nil {
		mapTriageErr(w, err)
		return
	}
	notes, err := s.Triage.ListNotes(r.Context(), hostID, eventID)
	if err != nil {
		mapTriageErr(w, err)
		return
	}
	logEntries, err := s.Triage.ListLog(r.Context(), hostID, eventID)
	if err != nil {
		mapTriageErr(w, err)
		return
	}

	resp := getResponse{
		Row:   rowToResponse(row),
		Notes: make([]noteResponse, 0, len(notes)),
		Log:   make([]logResponse, 0, len(logEntries)),
	}
	for _, n := range notes {
		resp.Notes = append(resp.Notes, noteResponse{
			ID: n.ID, HostID: n.HostID, EventID: n.EventID,
			Author: n.Author, Body: n.Body,
			CreatedAt: n.CreatedAt.Format(rfc3339Nano),
		})
	}
	for _, e := range logEntries {
		var from, to *string
		if e.FromStatus != nil {
			v := string(*e.FromStatus)
			from = &v
		}
		if e.ToStatus != nil {
			v := string(*e.ToStatus)
			to = &v
		}
		resp.Log = append(resp.Log, logResponse{
			ID: e.ID, Actor: e.Actor, FromStatus: from, ToStatus: to,
			At: e.At.Format(rfc3339Nano),
		})
	}
	httputil.WriteJSON(w, http.StatusOK, resp)
}

const rfc3339Nano = "2006-01-02T15:04:05.000000000Z07:00"

func rowToResponse(row triage.Row) rowResponse {
	return rowResponse{
		HostID:           row.HostID,
		EventID:          row.EventID,
		Status:           row.Status,
		Assignee:         row.Assignee,
		EvidenceSnapshot: row.EvidenceSnapshot,
		CreatedAt:        row.CreatedAt.Format(rfc3339Nano),
		UpdatedAt:        row.UpdatedAt.Format(rfc3339Nano),
	}
}

func mapTriageErr(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, triage.ErrNotFound):
		writeError(w, http.StatusNotFound, "not_found", "triage row not found")
	case errors.Is(err, triage.ErrInvalidStatus):
		writeError(w, http.StatusBadRequest, "invalid_query", err.Error())
	case errors.Is(err, triage.ErrMissingFields):
		writeError(w, http.StatusBadRequest, "invalid_query", err.Error())
	default:
		writeError(w, http.StatusInternalServerError, "internal", err.Error())
	}
}
