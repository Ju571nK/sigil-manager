// Package fleet defines the consumer-side interface to sigil-server's
// `/v1/*` read API (see docs/superpowers/specs/2026-05-16-fleet-api-contract.md
// v1.0, locked against sigil 3b.4 + 3b.6 / 3b.6.1 / 3b.6.2 additive notes
// in §14).
//
// Two implementations satisfy [Client]:
//   - [HttpClient] (internal/fleet/http.go, Plan 02 Task 3) — talks to a real
//     sigil-server over HTTP with shared bearer auth.
//   - [MockClient] (internal/fleet/mock.go, Plan 02 Task 4) — in-process
//     deterministic fixtures for local dev (MOCK_FLEET=1).
//
// Response types mirror the wire shapes exactly. Field names are the contract's
// snake_case via `json:"..."` tags; Go field names are PascalCase per gofmt.
// Where the wire allows null, we use pointer / nullable types. Where a single
// wire field can take multiple sub-shapes (Evidence variants, Source kinds,
// Subject kinds, AiGuardScope kinds), we keep the raw JSON and provide typed
// decoders for the variants Plan 02 actually renders.
package fleet

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"
)

// Client is the read-only Fleet API surface that sigil-manager consumes from
// sigil-server. Every method maps 1:1 to a contract §5 endpoint. Implementors
// MUST be safe for concurrent use.
type Client interface {
	// Healthz hits `GET /v1/healthz` (contract §5.1). No auth required.
	Healthz(ctx context.Context) (*Healthz, error)

	// Meta hits `GET /v1/meta` (contract §5.2). Returns server version,
	// schema_version, and the producer's recommended alerts definition.
	Meta(ctx context.Context) (*Meta, error)

	// PolicyMeta hits `GET /v1/policy/meta` (contract §5.9). Lightweight
	// policy envelope summary for the Settings page.
	PolicyMeta(ctx context.Context) (*PolicyMeta, error)

	// Events hits `GET /v1/events` (contract §5.7). Reverse-chronological
	// paged scan over JSONL evidence. Plan 02's Alerts queue is the primary
	// caller.
	Events(ctx context.Context, p EventsParams) (*EventsPage, error)

	// EventByID hits `GET /v1/events/{event_id}` (contract §5.8). Returns
	// the alert slide-over's body. 404 → ErrNotFound.
	EventByID(ctx context.Context, eventID string) (*Event, error)

	// FleetHosts hits `GET /v1/fleet/hosts` (contract §5.3). Plan 03 surface;
	// declared here so both implementations satisfy it now.
	FleetHosts(ctx context.Context, p HostsParams) (*HostsPage, error)

	// FleetHostByID hits `GET /v1/fleet/hosts/{host_id}` (contract §5.4).
	// Host-detail primary fetch. 404 → ErrNotFound.
	FleetHostByID(ctx context.Context, hostID string) (*HostDetail, error)

	// FleetRisk hits `GET /v1/fleet/risk` (contract §5.5). 1-row-per-host
	// risk summary sorted by max_score desc.
	FleetRisk(ctx context.Context, p RiskParams) (*RiskPage, error)

	// FleetCompliance hits `GET /v1/fleet/compliance` (contract §5.6). Raw
	// signals only — sigil-manager derives the status pill client-side per
	// F13.
	FleetCompliance(ctx context.Context, p ComplianceParams) (*CompliancePage, error)
}

// -----------------------------------------------------------------------------
// Errors
// -----------------------------------------------------------------------------

// ErrNotFound wraps a 404 response from any endpoint that exposes `{id}`
// lookups (`/v1/events/{id}`, `/v1/fleet/hosts/{id}`). Per contract §3.4, the
// read API returns 404 with no body when SIGIL_SERVER_READ_TOKEN is unset on
// the server side; callers should not distinguish that case from a genuine
// missing-id, just surface "host/event not found".
var ErrNotFound = errors.New("fleet: not found")

// ErrUnauthorized wraps a 401 — bearer missing or wrong (contract §6.1).
var ErrUnauthorized = errors.New("fleet: unauthorized")

// ErrServiceUnavailable wraps a 503 — sigil-server's boot rebuild is still
// in progress (F15). The HTTP impl honors `Retry-After` from the response
// header; callers may also back off and retry.
var ErrServiceUnavailable = errors.New("fleet: service unavailable")

// APIError is the contract §6.1 error body. HttpClient wraps non-2xx
// responses in this type so callers can inspect `Code` programmatically.
type APIError struct {
	Status  int             `json:"-"`
	Code    string          `json:"code"`
	Message string          `json:"message"`
	Details json.RawMessage `json:"details,omitempty"`
}

func (e *APIError) Error() string {
	return fmt.Sprintf("fleet: %d %s: %s", e.Status, e.Code, e.Message)
}

// -----------------------------------------------------------------------------
// Request params
// -----------------------------------------------------------------------------

// EventsParams maps to `GET /v1/events` query params (contract §5.7).
// Zero values mean "omit"; the HTTP impl skips empty strings/slices/zeros.
type EventsParams struct {
	Cursor           string    // opaque, from prior NextCursor
	Limit            int       // silently clamped [1,1000] server-side (§6.2)
	HostIDs          []string  // repeatable `?host_id=a&host_id=b`
	Since            time.Time // inclusive, agent's event.ts
	Until            time.Time // exclusive, agent's event.ts
	EvidenceKinds    []string  // comma-list, snake_case
	Severity         []string  // comma-list: info|warn (v1)
	Source           []string  // comma-list: file_system|agent
	MinAiGuardBucket string    // low|medium|high|critical; only meaningful when EvidenceKinds includes ai_guard_risk_assessed
}

// HostsParams maps to `GET /v1/fleet/hosts` query params (contract §5.3).
type HostsParams struct {
	Cursor string
	Limit  int
	Status []string // healthy|stale|disconnected
	Bucket []string // low|medium|high|critical (max per-tool current bucket)
	Sort   string   // last_seen|risk|host_id; unknown → last_seen
}

// RiskParams maps to `GET /v1/fleet/risk` query params (contract §5.5).
type RiskParams struct {
	Cursor    string
	Limit     int
	Tool      []string // claude_code|codex|claude_desktop|continue_dev|... (§14.5)
	MinBucket string   // low|medium|high|critical
}

// ComplianceParams maps to `GET /v1/fleet/compliance` query params (§5.6).
type ComplianceParams struct {
	Cursor string
	Limit  int
}

// -----------------------------------------------------------------------------
// Response: /v1/healthz (§5.1)
// -----------------------------------------------------------------------------

// Healthz is the body of `GET /v1/healthz` (§5.1). No auth.
type Healthz struct {
	Status string    `json:"status"` // "ok" in v1; reserved for degraded states
	TS     time.Time `json:"ts"`
}

// -----------------------------------------------------------------------------
// Response: /v1/meta (§5.2)
// -----------------------------------------------------------------------------

// Meta is the body of `GET /v1/meta` (§5.2).
type Meta struct {
	ServerVersion           string                  `json:"server_version"`
	SchemaVersion           int                     `json:"schema_version"`
	TS                      time.Time               `json:"ts"`
	AlertsDefinitionDefault AlertsDefinitionDefault `json:"alerts_definition_default"`
}

// AlertsDefinitionDefault is the producer's recommended alert set. Per F10
// this is NOT a versioned schema field — consumers MUST tolerate unknown
// values in AdditionalKinds.
type AlertsDefinitionDefault struct {
	EvidenceKinds   []string `json:"evidence_kinds"`
	AiGuardBuckets  []string `json:"ai_guard_buckets"`
	AdditionalKinds []string `json:"additional_kinds"`
}

// -----------------------------------------------------------------------------
// Response: /v1/policy/meta (§5.9)
// -----------------------------------------------------------------------------

// PolicyMeta is the body of `GET /v1/policy/meta` (§5.9). Lightweight
// companion to the full `/v1/policy` envelope endpoint.
type PolicyMeta struct {
	PolicyVersion   int       `json:"policy_version"`
	SigningPubkeyID string    `json:"signing_pubkey_id"`
	SignedAt        time.Time `json:"signed_at"`
	ValidUntil      time.Time `json:"valid_until"`
}

// -----------------------------------------------------------------------------
// Response: /v1/fleet/hosts (§5.3) + shared HostSummary
// -----------------------------------------------------------------------------

// HostSummary is one row of `/v1/fleet/hosts.hosts[*]` (§5.3) and the
// top-level block of `/v1/fleet/hosts/{host_id}` (§5.4).
type HostSummary struct {
	HostID             string         `json:"host_id"`
	Hostname           *string        `json:"hostname"` // null when no HostMetaSnapshot yet
	AgentVersion       string         `json:"agent_version"`
	LastSeenTS         time.Time      `json:"last_seen_ts"`
	Status             string         `json:"status"` // healthy|stale|disconnected
	CurrentRisk        *CurrentRisk   `json:"current_risk"`
	OpenEventCounts24h map[string]int `json:"open_event_counts_24h"` // keyed by severity
}

// CurrentRisk is the per-host current risk rollup. Null when the host has
// emitted no AiGuardRiskAssessed yet (§5.3 note).
type CurrentRisk struct {
	MaxScore  float64             `json:"max_score"`
	MaxBucket string              `json:"max_bucket"`
	ByTool    map[string]ToolRisk `json:"by_tool"` // only assessed tools
}

// ToolRisk is one entry of CurrentRisk.ByTool. Note: per §14.3 / §14.4 the
// `(tool, scope)` overwrite issue means a per-project assessment may stomp
// the user-global one; the host-detail page (Plan 03+) compensates by
// fetching event history.
type ToolRisk struct {
	Score      float64   `json:"score"`
	Bucket     string    `json:"bucket"`
	AssessedTS time.Time `json:"assessed_ts"`
}

// HostsPage is the body of `GET /v1/fleet/hosts` (§5.3).
type HostsPage struct {
	Hosts          []HostSummary `json:"hosts"`
	NextCursor     *string       `json:"next_cursor"`
	TotalEstimated int           `json:"total_estimated"` // exact in v1 (HashMap.len())
}

// -----------------------------------------------------------------------------
// Response: /v1/fleet/hosts/{host_id} (§5.4)
// -----------------------------------------------------------------------------

// HostDetail is the body of `GET /v1/fleet/hosts/{host_id}` (§5.4). It
// embeds HostSummary plus four nullable detail blocks. A null block means
// the host hasn't emitted that evidence variant yet (NOT "data unavailable").
type HostDetail struct {
	HostSummary
	HostMeta    *HostMeta    `json:"host_meta"`
	PolicyState *PolicyState `json:"policy_state"`
	AgentHealth *AgentHealth `json:"agent_health"`
	AiGuard     *AiGuard     `json:"ai_guard"`
}

// HostMeta is the verbatim wire payload of the latest HostMetaSnapshot
// evidence (3b.4-pre, wire-stable per sigil-core).
type HostMeta struct {
	OSName           string         `json:"os_name"`
	OSVersion        string         `json:"os_version"`
	KernelVersion    string         `json:"kernel_version"`
	Architecture     string         `json:"architecture"`
	Interfaces       []NetInterface `json:"interfaces"`
	DefaultGatewayV4 *string        `json:"default_gateway_v4"`
	DefaultGatewayV6 *string        `json:"default_gateway_v6"`
	DNSServers       []string       `json:"dns_servers"`
}

// NetInterface is one entry of HostMeta.Interfaces.
type NetInterface struct {
	Name string   `json:"name"`
	MAC  string   `json:"mac"`
	IPv4 []string `json:"ipv4"`
	IPv6 []string `json:"ipv6"`
}

// PolicyState is the host-detail policy block (§5.4). The 24h
// signature_failures field is intentionally NOT here — it lives on
// `/v1/fleet/compliance` per F13.
type PolicyState struct {
	LastAppliedPolicyVersion int        `json:"last_applied_policy_version"`
	PolicyExpiredActive      bool       `json:"policy_expired_active"`
	LastPolicyReloadTS       *time.Time `json:"last_policy_reload_ts"`
}

// AgentHealth is the host-detail agent-health block (§5.4). Fields derive
// from the most recent Heartbeat / WatcherDegraded / SenderLagCritical
// evidence per Q7 resolution.
type AgentHealth struct {
	RecentChannelStalls24h     int        `json:"recent_channel_stalls_24h"`
	RecentWatcherDegraded24h   int        `json:"recent_watcher_degraded_24h"`
	RecentSenderLagCritical24h int        `json:"recent_sender_lag_critical_24h"`
	LastHeartbeatTS            *time.Time `json:"last_heartbeat_ts"`
	HashP99MsLatest            *int       `json:"hash_p99_ms_latest"`
	JSONLAboveSoftFloorLatest  *bool      `json:"jsonl_above_soft_floor_latest"`
}

// AiGuard is the host-detail AI guard block (§5.4).
type AiGuard struct {
	ByTool map[string]ToolAiGuard `json:"by_tool"`
}

// ToolAiGuard is one entry of AiGuard.ByTool. Scope is kept raw because it
// has three variant shapes per §14.5 (user_global / project / application);
// callers parse with DecodeScope.
type ToolAiGuard struct {
	Score           float64           `json:"score"`
	Bucket          string            `json:"bucket"`
	AssessedTS      time.Time         `json:"assessed_ts"`
	IsReattestation bool              `json:"is_reattestation"`
	Scope           json.RawMessage   `json:"scope"`
	Reasons         []json.RawMessage `json:"reasons"`
}

// -----------------------------------------------------------------------------
// Response: /v1/fleet/risk (§5.5)
// -----------------------------------------------------------------------------

// RiskPage is the body of `GET /v1/fleet/risk` (§5.5).
type RiskPage struct {
	Rows       []RiskRow `json:"rows"`
	NextCursor *string   `json:"next_cursor"`
}

// RiskRow is one row of `/v1/fleet/risk.rows` (§5.5).
//
// OpenAlertCount24h note: per contract §10 (D2 / Ju571nK/sigil#21) the
// current producer impl returns `sum_warn()`, not the alerts-definition
// match count. Treat as a coarse "warn events in 24h" indicator until the
// follow-up ships.
type RiskRow struct {
	HostID            string    `json:"host_id"`
	Hostname          *string   `json:"hostname"`
	Score             float64   `json:"score"`
	Bucket            string    `json:"bucket"`
	TopTool           string    `json:"top_tool"`
	ReasonsCount      int       `json:"reasons_count"`
	AssessedTS        time.Time `json:"assessed_ts"`
	OpenAlertCount24h int       `json:"open_alert_count_24h"`
}

// -----------------------------------------------------------------------------
// Response: /v1/fleet/compliance (§5.6)
// -----------------------------------------------------------------------------

// CompliancePage is the body of `GET /v1/fleet/compliance` (§5.6).
type CompliancePage struct {
	Rows       []ComplianceRow `json:"rows"`
	NextCursor *string         `json:"next_cursor"`
}

// ComplianceRow is one row of `/v1/fleet/compliance.rows` (§5.6). Per F13
// this is raw signals only — sigil-manager derives the status pill from
// (VersionDrift, PolicyExpiredActive, SignatureFailures24h).
type ComplianceRow struct {
	HostID                     string     `json:"host_id"`
	Hostname                   *string    `json:"hostname"`
	LastAppliedPolicyVersion   int        `json:"last_applied_policy_version"`
	ServerCurrentPolicyVersion int        `json:"server_current_policy_version"`
	VersionDrift               int        `json:"version_drift"`
	PolicyExpiredActive        bool       `json:"policy_expired_active"`
	LastPolicyReloadTS         *time.Time `json:"last_policy_reload_ts"`
	SignatureFailures24h       int        `json:"signature_failures_24h"`
}

// -----------------------------------------------------------------------------
// Response: /v1/events (§5.7) + /v1/events/{id} (§5.8)
// -----------------------------------------------------------------------------

// EventsPage is the body of `GET /v1/events` (§5.7).
type EventsPage struct {
	Events     []Event `json:"events"`
	NextCursor *string `json:"next_cursor"`
}

// Event is the on-the-wire Event JSON from sigil-core (§5.7). The fleet API
// does NOT re-shape, redact, or recompose event fields, so this struct
// mirrors sigil-core's serde-derived shape directly.
//
// Subject and Source kinds vary by source — kept raw; callers parse if they
// care (Plan 02's Alerts queue only needs Evidence).
type Event struct {
	SchemaVersion int             `json:"schema_version"`
	EventID       string          `json:"event_id"` // UUIDv7
	TS            time.Time       `json:"ts"`       // agent clock
	HostID        string          `json:"host_id"`
	AgentVersion  string          `json:"agent_version"`
	Severity      string          `json:"severity"` // info|warn (v1); future variants treated as warn
	Source        json.RawMessage `json:"source"`   // { kind: "file_system" | "agent" }
	Subject       json.RawMessage `json:"subject"`
	Evidence      Evidence        `json:"evidence"`
	TargetID      *string         `json:"target_id"`
}

// Evidence carries the discriminator (`Kind`) plus the entire wire object
// so callers can decode into a variant-specific struct without us
// enumerating all 25+ Evidence::* variants from sigil-core. Plan 02 only
// renders AiGuardRiskAssessed — use AsAiGuard for that.
//
// On the wire: `"evidence": {"kind": "ai_guard_risk_assessed", ...}`.
type Evidence struct {
	Kind string          // populated from the "kind" field
	Raw  json.RawMessage // entire evidence object, including "kind"
}

// UnmarshalJSON captures the entire evidence object and extracts Kind.
func (e *Evidence) UnmarshalJSON(data []byte) error {
	e.Raw = append(e.Raw[:0], data...)
	var probe struct {
		Kind string `json:"kind"`
	}
	if err := json.Unmarshal(data, &probe); err != nil {
		return fmt.Errorf("fleet: evidence: %w", err)
	}
	e.Kind = probe.Kind
	return nil
}

// MarshalJSON emits the raw object verbatim (round-trip fidelity for
// Mock fixtures and tests).
func (e Evidence) MarshalJSON() ([]byte, error) {
	if len(e.Raw) == 0 {
		return []byte(`null`), nil
	}
	return e.Raw, nil
}

// EvidenceAiGuard is the decoded payload of `evidence.kind ==
// "ai_guard_risk_assessed"` events (§5.7). Plan 02's Alerts queue renders
// from this struct. Scope is kept raw because of the three variant shapes
// in §14.5; callers use DecodeScope.
type EvidenceAiGuard struct {
	Kind            string            `json:"kind"` // always "ai_guard_risk_assessed"
	Tool            string            `json:"tool"` // §14.5: claude_code|codex|claude_desktop|continue_dev|...
	Scope           json.RawMessage   `json:"scope"`
	Score           float64           `json:"score"`
	Bucket          string            `json:"bucket"`
	Reasons         []json.RawMessage `json:"reasons"`
	IsReattestation bool              `json:"is_reattestation"`
}

// AsAiGuard decodes an Evidence as the AiGuardRiskAssessed variant. Returns
// nil with no error when Kind is a different variant — callers should check.
func (e Evidence) AsAiGuard() (*EvidenceAiGuard, error) {
	if e.Kind != "ai_guard_risk_assessed" {
		return nil, nil
	}
	var out EvidenceAiGuard
	if err := json.Unmarshal(e.Raw, &out); err != nil {
		return nil, fmt.Errorf("fleet: evidence ai_guard: %w", err)
	}
	return &out, nil
}

// Scope is the three-shape AiGuardScope per §14.5 in decoded form. Callers
// use DecodeScope to turn a raw scope json.RawMessage into this struct.
//
// Shapes:
//   - {"kind": "user_global"}                            → Kind="user_global"
//   - {"kind": "project", "path": "/abs/..."}            → Kind="project", Path set
//   - {"kind": "application", "app": "claude_desktop"}   → Kind="application", App set
type Scope struct {
	Kind string `json:"kind"`
	Path string `json:"path,omitempty"`
	App  string `json:"app,omitempty"`
}

// DecodeScope parses a raw scope value. Unknown kinds round-trip via Kind
// alone — Path and App will be empty.
func DecodeScope(raw json.RawMessage) (*Scope, error) {
	if len(raw) == 0 || string(raw) == "null" {
		return nil, nil
	}
	var s Scope
	if err := json.Unmarshal(raw, &s); err != nil {
		return nil, fmt.Errorf("fleet: scope: %w", err)
	}
	return &s, nil
}
