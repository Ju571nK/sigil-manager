package v1

import (
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/Ju571nK/sigil-manager/internal/fleet"
	"github.com/Ju571nK/sigil-manager/internal/httputil"
	"github.com/Ju571nK/sigil-manager/internal/triage"
)

// handleFleetMeta is a thin pass-through to FleetClient.Meta. Errors map
// to consumer-side codes; the SPA never sees `sigil-server` URLs.
func (s *Server) handleFleetMeta(w http.ResponseWriter, r *http.Request) {
	out, err := s.Fleet.Meta(r.Context())
	if err != nil {
		mapFleetErr(w, err)
		return
	}
	httputil.WriteJSON(w, http.StatusOK, out)
}

// handleFleetHealthz pass-through.
func (s *Server) handleFleetHealthz(w http.ResponseWriter, r *http.Request) {
	out, err := s.Fleet.Healthz(r.Context())
	if err != nil {
		mapFleetErr(w, err)
		return
	}
	httputil.WriteJSON(w, http.StatusOK, out)
}

// eventWithTriage is the per-event row the queue renders. The triage field
// is null when no triage row exists yet (most events are unactioned).
type eventWithTriage struct {
	fleet.Event
	Triage *triageView `json:"triage"`
}

type triageView struct {
	Status    triage.Status `json:"status"`
	Assignee  *string       `json:"assignee"`
	UpdatedAt time.Time     `json:"updated_at"`
}

type eventsResponse struct {
	Events     []eventWithTriage `json:"events"`
	NextCursor *string           `json:"next_cursor"`
}

// handleFleetEvents calls FleetClient.Events with passthrough query params
// then joins the triage table so the SPA can paint status pills without a
// second round-trip per row.
func (s *Server) handleFleetEvents(w http.ResponseWriter, r *http.Request) {
	params, err := parseEventsParams(r)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid_query", err.Error())
		return
	}
	page, err := s.Fleet.Events(r.Context(), params)
	if err != nil {
		mapFleetErr(w, err)
		return
	}
	enriched := make([]eventWithTriage, 0, len(page.Events))
	for _, ev := range page.Events {
		enriched = append(enriched, eventWithTriage{Event: ev, Triage: s.lookupTriageView(r, ev.HostID, ev.EventID)})
	}
	httputil.WriteJSON(w, http.StatusOK, eventsResponse{Events: enriched, NextCursor: page.NextCursor})
}

// handleFleetEventByID returns the single event plus its triage view.
func (s *Server) handleFleetEventByID(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "event_id")
	if id == "" {
		writeError(w, http.StatusBadRequest, "invalid_query", "event_id required")
		return
	}
	ev, err := s.Fleet.EventByID(r.Context(), id)
	if err != nil {
		mapFleetErr(w, err)
		return
	}
	httputil.WriteJSON(w, http.StatusOK, eventWithTriage{
		Event:  *ev,
		Triage: s.lookupTriageView(r, ev.HostID, ev.EventID),
	})
}

// handleFleetRisk is a pass-through to FleetClient.FleetRisk (§5.5). The
// RiskPage already carries JSON tags so we relay it verbatim. Note the
// open_alert_count_24h caveat (contract §13.1 / issue #21) is a rendering
// concern, handled SPA-side — the server relays the producer's number.
func (s *Server) handleFleetRisk(w http.ResponseWriter, r *http.Request) {
	params, err := parseRiskParams(r)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid_query", err.Error())
		return
	}
	page, err := s.Fleet.FleetRisk(r.Context(), params)
	if err != nil {
		mapFleetErr(w, err)
		return
	}
	httputil.WriteJSON(w, http.StatusOK, page)
}

// handleFleetCompliance is a pass-through to FleetClient.FleetCompliance
// (§5.6). Per F13 the server returns raw signals only — the status pill is
// derived SPA-side (web/src/lib/compliance.ts).
func (s *Server) handleFleetCompliance(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	params := fleet.ComplianceParams{Cursor: q.Get("cursor")}
	if v := q.Get("limit"); v != "" {
		n, err := strconv.Atoi(v)
		if err != nil {
			writeError(w, http.StatusBadRequest, "invalid_query", "limit must be an integer")
			return
		}
		params.Limit = n
	}
	page, err := s.Fleet.FleetCompliance(r.Context(), params)
	if err != nil {
		mapFleetErr(w, err)
		return
	}
	httputil.WriteJSON(w, http.StatusOK, page)
}

// parseRiskParams translates the public query string into a
// [fleet.RiskParams] (§5.5). `tool` is a comma list; `min_bucket` is
// relayed as-is (the client clamps unknown values).
func parseRiskParams(r *http.Request) (fleet.RiskParams, error) {
	q := r.URL.Query()
	out := fleet.RiskParams{
		Cursor:    q.Get("cursor"),
		Tool:      splitComma(q.Get("tool")),
		MinBucket: q.Get("min_bucket"),
	}
	if v := q.Get("limit"); v != "" {
		n, err := strconv.Atoi(v)
		if err != nil {
			return out, errors.New("limit must be an integer")
		}
		out.Limit = n
	}
	return out, nil
}

// lookupTriageView returns the per-row triage projection, or nil when the
// row doesn't exist (every event is born untriaged). Errors are silently
// dropped because a triage DB blip shouldn't tank the alerts queue —
// log+continue would be ideal once we wire a logger here.
func (s *Server) lookupTriageView(r *http.Request, hostID, eventID string) *triageView {
	if s.Triage == nil {
		return nil
	}
	row, err := s.Triage.GetByEventKey(r.Context(), hostID, eventID)
	if err != nil {
		return nil
	}
	return &triageView{Status: row.Status, Assignee: row.Assignee, UpdatedAt: row.UpdatedAt}
}

// parseEventsParams translates the public query string into an
// [fleet.EventsParams]. Unknown params are silently ignored. Bad timestamps
// or non-integer limits return errors so the SPA can show a useful message.
func parseEventsParams(r *http.Request) (fleet.EventsParams, error) {
	q := r.URL.Query()
	out := fleet.EventsParams{
		Cursor:           q.Get("cursor"),
		HostIDs:          splitComma(q.Get("host_id")),
		EvidenceKinds:    splitComma(q.Get("evidence_kind")),
		Severity:         splitComma(q.Get("severity")),
		Source:           splitComma(q.Get("source")),
		MinAiGuardBucket: q.Get("min_ai_guard_bucket"),
	}
	if v := q.Get("limit"); v != "" {
		n, err := strconv.Atoi(v)
		if err != nil {
			return out, errors.New("limit must be an integer")
		}
		out.Limit = n
	}
	if v := q.Get("since"); v != "" {
		t, err := time.Parse(time.RFC3339, v)
		if err != nil {
			return out, errors.New("since must be RFC3339")
		}
		out.Since = t
	}
	if v := q.Get("until"); v != "" {
		t, err := time.Parse(time.RFC3339, v)
		if err != nil {
			return out, errors.New("until must be RFC3339")
		}
		out.Until = t
	}
	// `host_id` is the contract's only repeatable param; merge ?host_id=a&host_id=b
	// alongside the comma-list form so both work.
	if multi, ok := r.URL.Query()["host_id"]; ok && len(multi) > 1 {
		out.HostIDs = nil
		for _, v := range multi {
			out.HostIDs = append(out.HostIDs, splitComma(v)...)
		}
	}
	return out, nil
}

func splitComma(v string) []string {
	if v == "" {
		return nil
	}
	parts := strings.Split(v, ",")
	out := parts[:0]
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			out = append(out, p)
		}
	}
	return out
}

// mapFleetErr translates a fleet.* error into the public HTTP shape. The
// codes mirror the contract §6.1 vocabulary so SPA error-handling can be
// shared between consumer-side and producer-side errors.
func mapFleetErr(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, fleet.ErrNotFound):
		writeError(w, http.StatusNotFound, "not_found", "event or host not found")
	case errors.Is(err, fleet.ErrReadAPIDisabled):
		writeError(w, http.StatusBadGateway, "read_api_disabled",
			"sigil-server has the read API disabled — set SIGIL_SERVER_READ_TOKEN there")
	case errors.Is(err, fleet.ErrUnauthorized):
		writeError(w, http.StatusBadGateway, "upstream_unauthorized",
			"sigil-server rejected our token — check SIGIL_SERVER_READ_TOKEN")
	case errors.Is(err, fleet.ErrServiceUnavailable):
		writeError(w, http.StatusServiceUnavailable, "service_unavailable",
			"sigil-server is rebuilding its index, retry shortly")
	default:
		var apiErr *fleet.APIError
		if errors.As(err, &apiErr) {
			writeError(w, http.StatusBadGateway, "upstream_error", apiErr.Error())
			return
		}
		writeError(w, http.StatusBadGateway, "upstream_error", err.Error())
	}
}
