package fleet

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"time"
)

// MockClient is an in-process [Client] implementation backed by deterministic
// fixtures. Toggled via MOCK_FLEET=1 (config.IsMockFleet); used in local dev
// when no sigil-server is running and in unit tests. Safe for concurrent reads.
//
// Fixtures cover the §14.5 aggregate checklist: 5 hosts (healthy/stale/
// disconnected mix), one host with full HostMetaSnapshot + one with null,
// and 32 events spanning the trailing 24h with the union of `tool` values
// (claude_code, codex, claude_desktop, continue_dev, gemini, cursor) and the union of
// `scope.kind` values (user_global, project, application) all present.
type MockClient struct {
	seed         time.Time
	hosts        []HostSummary
	hostDetails  map[string]*HostDetail
	events       []Event // sorted by event_id desc (matches §5.7 reverse-chrono ordering)
	policyMeta   PolicyMeta
	meta         Meta
	versionDrift map[string]int // host_id → server_current_policy_version - last_applied
}

// NewMock returns a MockClient with timestamps anchored at seed.
// seed.Zero() picks time.Now().UTC().Truncate(time.Second).
func NewMock(seed time.Time) *MockClient {
	if seed.IsZero() {
		seed = time.Now().UTC().Truncate(time.Second)
	}
	m := &MockClient{seed: seed, hostDetails: map[string]*HostDetail{}, versionDrift: map[string]int{}}
	m.build()
	return m
}

// -----------------------------------------------------------------------------
// Client interface
// -----------------------------------------------------------------------------

// Healthz implements [Client.Healthz].
func (m *MockClient) Healthz(_ context.Context) (*Healthz, error) {
	return &Healthz{Status: "ok", TS: m.seed}, nil
}

// Meta implements [Client.Meta].
func (m *MockClient) Meta(_ context.Context) (*Meta, error) {
	out := m.meta
	return &out, nil
}

// PolicyMeta implements [Client.PolicyMeta].
func (m *MockClient) PolicyMeta(_ context.Context) (*PolicyMeta, error) {
	out := m.policyMeta
	return &out, nil
}

// Events implements [Client.Events]. Reverse-chronological walk of fixtures
// with cursor = last returned event_id; events with event_id < cursor are
// returned next (matches §5.7 / §7).
func (m *MockClient) Events(_ context.Context, p EventsParams) (*EventsPage, error) {
	limit := clampLimit(p.Limit)

	filtered := make([]Event, 0, len(m.events))
	for _, e := range m.events {
		if p.Cursor != "" && e.EventID >= p.Cursor {
			continue
		}
		if !matchesEvent(e, p) {
			continue
		}
		filtered = append(filtered, e)
	}

	var nextCursor *string
	if len(filtered) > limit {
		filtered = filtered[:limit]
		last := filtered[len(filtered)-1].EventID
		nextCursor = &last
	}
	return &EventsPage{Events: filtered, NextCursor: nextCursor}, nil
}

// EventByID implements [Client.EventByID].
func (m *MockClient) EventByID(_ context.Context, eventID string) (*Event, error) {
	for i := range m.events {
		if m.events[i].EventID == eventID {
			out := m.events[i]
			return &out, nil
		}
	}
	return nil, ErrNotFound
}

// FleetHosts implements [Client.FleetHosts].
func (m *MockClient) FleetHosts(_ context.Context, p HostsParams) (*HostsPage, error) {
	limit := clampLimit(p.Limit)

	hosts := make([]HostSummary, 0, len(m.hosts))
	for _, h := range m.hosts {
		if !matchesHost(h, p) {
			continue
		}
		hosts = append(hosts, h)
	}

	startIdx := 0
	if p.Cursor != "" {
		for i, h := range hosts {
			if h.HostID == p.Cursor {
				startIdx = i + 1
				break
			}
		}
	}
	if startIdx >= len(hosts) {
		return &HostsPage{Hosts: nil, NextCursor: nil, TotalEstimated: len(m.hosts)}, nil
	}
	hosts = hosts[startIdx:]

	var nextCursor *string
	if len(hosts) > limit {
		hosts = hosts[:limit]
		last := hosts[len(hosts)-1].HostID
		nextCursor = &last
	}
	return &HostsPage{Hosts: hosts, NextCursor: nextCursor, TotalEstimated: len(m.hosts)}, nil
}

// FleetHostByID implements [Client.FleetHostByID].
func (m *MockClient) FleetHostByID(_ context.Context, hostID string) (*HostDetail, error) {
	d, ok := m.hostDetails[hostID]
	if !ok {
		return nil, ErrNotFound
	}
	out := *d
	return &out, nil
}

// FleetRisk implements [Client.FleetRisk]. Derives rows from hosts with
// non-nil current_risk, sorted by max_score desc per F14.
func (m *MockClient) FleetRisk(_ context.Context, p RiskParams) (*RiskPage, error) {
	limit := clampLimit(p.Limit)

	rows := make([]RiskRow, 0, len(m.hosts))
	for _, h := range m.hosts {
		if h.CurrentRisk == nil {
			continue
		}
		topTool := pickTopTool(h.CurrentRisk)
		if len(p.Tool) > 0 && !containsString(p.Tool, topTool) {
			continue
		}
		if p.MinBucket != "" && !bucketAtLeast(h.CurrentRisk.MaxBucket, p.MinBucket) {
			continue
		}
		rows = append(rows, RiskRow{
			HostID:            h.HostID,
			Hostname:          h.Hostname,
			Score:             h.CurrentRisk.MaxScore,
			Bucket:            h.CurrentRisk.MaxBucket,
			TopTool:           topTool,
			ReasonsCount:      0,
			AssessedTS:        latestAssessed(h.CurrentRisk),
			OpenAlertCount24h: h.OpenEventCounts24h["warn"], // mirrors D2 sum_warn behavior
		})
	}
	sort.SliceStable(rows, func(i, j int) bool { return rows[i].Score > rows[j].Score })

	startIdx := 0
	if p.Cursor != "" {
		for i, r := range rows {
			if r.HostID == p.Cursor {
				startIdx = i + 1
				break
			}
		}
	}
	if startIdx >= len(rows) {
		return &RiskPage{Rows: nil, NextCursor: nil}, nil
	}
	rows = rows[startIdx:]
	var nextCursor *string
	if len(rows) > limit {
		rows = rows[:limit]
		last := rows[len(rows)-1].HostID
		nextCursor = &last
	}
	return &RiskPage{Rows: rows, NextCursor: nextCursor}, nil
}

// FleetCompliance implements [Client.FleetCompliance].
func (m *MockClient) FleetCompliance(_ context.Context, p ComplianceParams) (*CompliancePage, error) {
	limit := clampLimit(p.Limit)

	rows := make([]ComplianceRow, 0, len(m.hosts))
	for _, h := range m.hosts {
		detail := m.hostDetails[h.HostID]
		var lastApplied int
		var reloadTS *time.Time
		expired := false
		if detail != nil && detail.PolicyState != nil {
			lastApplied = detail.PolicyState.LastAppliedPolicyVersion
			reloadTS = detail.PolicyState.LastPolicyReloadTS
			expired = detail.PolicyState.PolicyExpiredActive
		}
		serverVer := m.policyMeta.PolicyVersion
		rows = append(rows, ComplianceRow{
			HostID:                     h.HostID,
			Hostname:                   h.Hostname,
			LastAppliedPolicyVersion:   lastApplied,
			ServerCurrentPolicyVersion: serverVer,
			VersionDrift:               m.versionDrift[h.HostID],
			PolicyExpiredActive:        expired,
			LastPolicyReloadTS:         reloadTS,
			SignatureFailures24h:       0,
		})
	}

	startIdx := 0
	if p.Cursor != "" {
		for i, r := range rows {
			if r.HostID == p.Cursor {
				startIdx = i + 1
				break
			}
		}
	}
	if startIdx >= len(rows) {
		return &CompliancePage{Rows: nil, NextCursor: nil}, nil
	}
	rows = rows[startIdx:]
	var nextCursor *string
	if len(rows) > limit {
		rows = rows[:limit]
		last := rows[len(rows)-1].HostID
		nextCursor = &last
	}
	return &CompliancePage{Rows: rows, NextCursor: nextCursor}, nil
}

// -----------------------------------------------------------------------------
// Filter helpers
// -----------------------------------------------------------------------------

func matchesEvent(e Event, p EventsParams) bool {
	if !p.Since.IsZero() && e.TS.Before(p.Since) {
		return false
	}
	if !p.Until.IsZero() && !e.TS.Before(p.Until) {
		return false
	}
	if len(p.HostIDs) > 0 && !containsString(p.HostIDs, e.HostID) {
		return false
	}
	if len(p.Severity) > 0 && !containsString(p.Severity, e.Severity) {
		return false
	}
	if len(p.EvidenceKinds) > 0 && !containsString(p.EvidenceKinds, e.Evidence.Kind) {
		return false
	}
	if p.MinAiGuardBucket != "" && e.Evidence.Kind == "ai_guard_risk_assessed" {
		ag, err := e.Evidence.AsAiGuard()
		if err != nil || ag == nil || !bucketAtLeast(ag.Bucket, p.MinAiGuardBucket) {
			return false
		}
	}
	return true
}

func matchesHost(h HostSummary, p HostsParams) bool {
	if len(p.Status) > 0 && !containsString(p.Status, h.Status) {
		return false
	}
	if len(p.Bucket) > 0 {
		if h.CurrentRisk == nil || !containsString(p.Bucket, h.CurrentRisk.MaxBucket) {
			return false
		}
	}
	return true
}

func clampLimit(n int) int {
	if n <= 0 {
		return 100
	}
	if n > 1000 {
		return 1000
	}
	return n
}

func containsString(haystack []string, needle string) bool {
	for _, s := range haystack {
		if s == needle {
			return true
		}
	}
	return false
}

var bucketOrder = map[string]int{"low": 1, "medium": 2, "high": 3, "critical": 4}

func bucketAtLeast(have, min string) bool {
	return bucketOrder[have] >= bucketOrder[min]
}

func pickTopTool(cr *CurrentRisk) string {
	var top string
	var topScore float64 = -1
	for tool, tr := range cr.ByTool {
		if tr.Score > topScore {
			top = tool
			topScore = tr.Score
		}
	}
	return top
}

func latestAssessed(cr *CurrentRisk) time.Time {
	var t time.Time
	for _, tr := range cr.ByTool {
		if tr.AssessedTS.After(t) {
			t = tr.AssessedTS
		}
	}
	return t
}

// -----------------------------------------------------------------------------
// Fixture construction
// -----------------------------------------------------------------------------

// build populates the Mock's fixtures. Layout:
//   - 5 hosts (alice/bob/carol/dave/eve) with the status/risk/host_meta
//     coverage described on MockClient.
//   - 32 events spanning seed-24h .. seed; tool + scope unions cover §14.5.
func (m *MockClient) build() {
	m.meta = Meta{
		ServerVersion: "0.5.0-mock",
		SchemaVersion: 1,
		TS:            m.seed,
		AlertsDefinitionDefault: AlertsDefinitionDefault{
			EvidenceKinds:   []string{"ai_guard_risk_assessed"},
			AiGuardBuckets:  []string{"high", "critical"},
			AdditionalKinds: []string{"policy_signature_invalid", "tls_failure", "host_id_fingerprint_drift", "agent_dying", "sender_lag_critical"},
		},
	}
	signedAt := m.seed.Add(-48 * time.Hour)
	validUntil := m.seed.Add(7 * 24 * time.Hour)
	m.policyMeta = PolicyMeta{
		PolicyVersion:   18,
		SigningPubkeyID: "k1",
		SignedAt:        signedAt,
		ValidUntil:      validUntil,
	}

	m.buildHosts()
	m.buildEvents()
}

func (m *MockClient) buildHosts() {
	type seed struct {
		id, name      string
		status        string
		lastSeenDelta time.Duration
		risk          *CurrentRisk
		hostMeta      *HostMeta
		policyVer     int
		expired       bool
		warn, info    int
	}

	seedReload := m.seed.Add(-4 * time.Hour)
	seedHB := m.seed.Add(-1 * time.Minute)

	hostMetaAlice := &HostMeta{
		OSName:        "macOS",
		OSVersion:     "14.5",
		KernelVersion: "23.5.0",
		Architecture:  "arm64",
		Interfaces: []NetInterface{{
			Name: "en0", MAC: "00:1b:44:11:3a:b7",
			IPv4: []string{"192.168.1.42/24"}, IPv6: []string{"fe80::1/64"},
		}},
		DefaultGatewayV4: strPtr("192.168.1.1"),
		DefaultGatewayV6: nil,
		DNSServers:       []string{"1.1.1.1", "8.8.8.8"},
	}
	hostMetaCarol := &HostMeta{
		OSName: "Linux", OSVersion: "Ubuntu 24.04", KernelVersion: "6.8.0-31-generic",
		Architecture: "x86_64",
		Interfaces: []NetInterface{{
			Name: "eth0", MAC: "00:1b:44:22:bb:c1",
			IPv4: []string{"10.0.5.21/24"}, IPv6: []string{},
		}},
		DefaultGatewayV4: strPtr("10.0.5.1"),
		DNSServers:       []string{"10.0.5.1"},
	}
	hostMetaEve := &HostMeta{
		OSName: "Windows", OSVersion: "11 23H2", KernelVersion: "10.0.22631",
		Architecture: "x86_64",
		Interfaces: []NetInterface{{
			Name: "Ethernet0", MAC: "00:1b:44:33:ee:42",
			IPv4: []string{"172.16.4.18/16"}, IPv6: []string{},
		}},
		DefaultGatewayV4: strPtr("172.16.0.1"),
		DNSServers:       []string{"8.8.8.8", "8.8.4.4"},
	}

	seeds := []seed{
		{
			id: "5a7c3e91-aaaa-bbbb-cccc-111111111111", name: "alice-mbp",
			status: "healthy", lastSeenDelta: -1 * time.Minute,
			risk: &CurrentRisk{
				MaxScore: 7.2, MaxBucket: "critical",
				ByTool: map[string]ToolRisk{
					"claude_code": {Score: 7.2, Bucket: "critical", AssessedTS: m.seed.Add(-2 * time.Minute)},
				},
			},
			hostMeta: hostMetaAlice, policyVer: 18, warn: 14, info: 1402,
		},
		{
			id: "5a7c3e91-aaaa-bbbb-cccc-222222222222", name: "bob-laptop",
			status: "healthy", lastSeenDelta: -3 * time.Minute,
			risk: &CurrentRisk{
				MaxScore: 4.5, MaxBucket: "medium",
				ByTool: map[string]ToolRisk{
					"codex": {Score: 4.5, Bucket: "medium", AssessedTS: m.seed.Add(-15 * time.Minute)},
				},
			},
			hostMeta: nil, policyVer: 17, warn: 3, info: 250,
		},
		{
			id: "5a7c3e91-aaaa-bbbb-cccc-333333333333", name: "carol-dev",
			status: "stale", lastSeenDelta: -30 * time.Minute,
			risk: &CurrentRisk{
				MaxScore: 6.8, MaxBucket: "high",
				ByTool: map[string]ToolRisk{
					"claude_desktop": {Score: 6.8, Bucket: "high", AssessedTS: m.seed.Add(-1 * time.Hour)},
				},
			},
			hostMeta: hostMetaCarol, policyVer: 16, warn: 8, info: 410,
		},
		{
			// dave-vm: disconnected, never emitted HostMetaSnapshot, no AI guard yet.
			// Hostname is null per §5.3 (no HostMetaSnapshot ⇒ hostname=null).
			id: "5a7c3e91-aaaa-bbbb-cccc-444444444444", name: "",
			status: "disconnected", lastSeenDelta: -3 * time.Hour,
			risk: nil, hostMeta: nil, policyVer: 12, expired: true, warn: 0, info: 0,
		},
		{
			id: "5a7c3e91-aaaa-bbbb-cccc-555555555555", name: "eve-workstation",
			status: "healthy", lastSeenDelta: -2 * time.Minute,
			risk: &CurrentRisk{
				MaxScore: 8.4, MaxBucket: "critical",
				ByTool: map[string]ToolRisk{
					"continue_dev": {Score: 8.4, Bucket: "critical", AssessedTS: m.seed.Add(-5 * time.Minute)},
				},
			},
			hostMeta: hostMetaEve, policyVer: 18, warn: 21, info: 980,
		},
	}

	for _, s := range seeds {
		var hostname *string
		if s.name != "" {
			n := s.name
			hostname = &n
		}
		summary := HostSummary{
			HostID:       s.id,
			Hostname:     hostname,
			AgentVersion: "0.4.0",
			LastSeenTS:   m.seed.Add(s.lastSeenDelta),
			Status:       s.status,
			CurrentRisk:  s.risk,
			OpenEventCounts24h: map[string]int{
				"warn": s.warn,
				"info": s.info,
			},
		}
		m.hosts = append(m.hosts, summary)

		var policyState *PolicyState
		if s.policyVer > 0 {
			reload := seedReload
			policyState = &PolicyState{
				LastAppliedPolicyVersion: s.policyVer,
				PolicyExpiredActive:      s.expired,
				LastPolicyReloadTS:       &reload,
			}
		}

		var agentHealth *AgentHealth
		if s.status != "disconnected" {
			hb := seedHB
			p99 := 4
			above := false
			agentHealth = &AgentHealth{
				LastHeartbeatTS:           &hb,
				HashP99MsLatest:           &p99,
				JSONLAboveSoftFloorLatest: &above,
			}
		}

		var aiGuard *AiGuard
		if s.risk != nil {
			byTool := map[string]ToolAiGuard{}
			for tool, tr := range s.risk.ByTool {
				scope := json.RawMessage(`{"kind":"user_global"}`)
				byTool[tool] = ToolAiGuard{
					Score: tr.Score, Bucket: tr.Bucket, AssessedTS: tr.AssessedTS,
					IsReattestation: false, Scope: scope, Reasons: nil,
				}
			}
			aiGuard = &AiGuard{ByTool: byTool}
		}

		m.hostDetails[s.id] = &HostDetail{
			HostSummary: summary,
			HostMeta:    s.hostMeta,
			PolicyState: policyState,
			AgentHealth: agentHealth,
			AiGuard:     aiGuard,
		}

		m.versionDrift[s.id] = m.policyMeta.PolicyVersion - s.policyVer
	}
}

// buildEvents emits 32 deterministic events spanning seed-24h..seed. The
// recipe in `eventRecipes` is hand-crafted so the union of `tool` values
// used by ai_guard_risk_assessed events == {claude_code, codex,
// claude_desktop, continue_dev, gemini, cursor} (the latter two added
// per contract §14.7 after producer Phase 3b.7 shipped) and the union
// of `scope.kind` shapes == {user_global, project, application}, with
// at least 5 events in {high, critical} bucket.
func (m *MockClient) buildEvents() {
	recipes := eventRecipes()

	for i, r := range recipes {
		// Stagger event timestamps backwards from seed across 24h, ~44 min apart
		// (32 × 44 = 1408 min < 24h). Earlier index → newer timestamp (so
		// events[0] is the most recent).
		eventTS := m.seed.Add(-time.Duration(i) * 44 * time.Minute)
		eventID := uuidv7At(eventTS.UnixMilli(), uint64(len(recipes)-i))

		ev := Event{
			SchemaVersion: 1,
			EventID:       eventID,
			TS:            eventTS,
			HostID:        r.hostID,
			AgentVersion:  "0.4.0",
			Severity:      r.severity,
			Source:        json.RawMessage(r.source),
			Subject:       json.RawMessage(r.subject),
			Evidence:      Evidence{Raw: json.RawMessage(r.evidence)},
		}
		// Populate Evidence.Kind from the raw JSON.
		var probe struct {
			Kind string `json:"kind"`
		}
		if err := json.Unmarshal(ev.Evidence.Raw, &probe); err == nil {
			ev.Evidence.Kind = probe.Kind
		}
		m.events = append(m.events, ev)
	}

	// Sort by event_id desc (matches §5.7 reverse-chrono walk).
	sort.SliceStable(m.events, func(i, j int) bool {
		return m.events[i].EventID > m.events[j].EventID
	})
}

// uuidv7At returns a UUIDv7-shaped string with the millisecond timestamp
// embedded in the high 48 bits, so events sort lexicographically by time.
// Not cryptographically random; counter fills the lower bits for uniqueness.
func uuidv7At(unixMs int64, counter uint64) string {
	hi32 := uint32(unixMs >> 16)
	mid16 := uint16(unixMs & 0xFFFF)
	rand1 := uint16(counter & 0xFFF)
	rand2 := uint16((counter >> 12) & 0xFFF)
	rand3 := counter >> 24 & 0xFFFFFFFFFFFF
	return fmt.Sprintf("%08x-%04x-7%03x-8%03x-%012x", hi32, mid16, rand1, rand2, rand3)
}

func strPtr(s string) *string { return &s }

// eventRecipe is a compact spec for a fixture event. Fields are raw JSON
// strings to keep the recipe table readable.
type eventRecipe struct {
	hostID, severity, source, subject, evidence string
}

// eventRecipes is the canonical list of 32 fixture events. The first ones
// in the list become the newest (most recent ts) after build sorting.
// Coverage targets (see buildEvents): union of tools, union of scope kinds,
// ≥5 events with kind=ai_guard_risk_assessed AND bucket ∈ {high, critical}.
//
//nolint:lll // wire-shape JSON literals are easier to read on one line each
func eventRecipes() []eventRecipe {
	const (
		hAlice = "5a7c3e91-aaaa-bbbb-cccc-111111111111"
		hBob   = "5a7c3e91-aaaa-bbbb-cccc-222222222222"
		hCarol = "5a7c3e91-aaaa-bbbb-cccc-333333333333"
		hDave  = "5a7c3e91-aaaa-bbbb-cccc-444444444444"
		hEve   = "5a7c3e91-aaaa-bbbb-cccc-555555555555"
	)
	src := `{"kind":"file_system"}`
	srcAgent := `{"kind":"agent"}`
	subjPath := func(p string) string {
		return `{"kind":"path","value":"` + p + `"}`
	}
	subjAgent := `{"kind":"agent"}`

	return []eventRecipe{
		// --- Alice: claude_code critical, user_global ---
		{hAlice, "warn", src, subjPath("/Users/alice/.claude/settings.json"), `{"kind":"ai_guard_risk_assessed","tool":"claude_code","scope":{"kind":"user_global"},"score":7.2,"bucket":"critical","reasons":[{"kind":"destructive_in_inline_command","pattern":"rm -rf","hook_event":"PreToolUse","snippet":"rm -rf /"}],"is_reattestation":false}`},
		// --- Eve: continue_dev critical, user_global ---
		{hEve, "warn", src, subjPath("/Users/eve/.continue/config.json"), `{"kind":"ai_guard_risk_assessed","tool":"continue_dev","scope":{"kind":"user_global"},"score":8.4,"bucket":"critical","reasons":[{"kind":"mcp_server_remote","hook_event":"mcp_command"}],"is_reattestation":false}`},
		// --- Carol: claude_desktop high, application scope ---
		{hCarol, "warn", src, subjPath("/home/carol/.config/Claude/claude_desktop_config.json"), `{"kind":"ai_guard_risk_assessed","tool":"claude_desktop","scope":{"kind":"application","app":"claude_desktop"},"score":6.8,"bucket":"high","reasons":[{"kind":"no_sandbox","executor":"mcp_command"}],"is_reattestation":false}`},
		// --- Alice: per-project claude_code, high (project scope) ---
		{hAlice, "warn", src, subjPath("/Users/alice/code/work-repo/.claude/settings.local.json"), `{"kind":"ai_guard_risk_assessed","tool":"claude_code","scope":{"kind":"project","path":"/Users/alice/code/work-repo"},"score":6.1,"bucket":"high","reasons":[{"kind":"external_script_unscanned","pattern":"hooks/preflight.sh"}],"is_reattestation":false}`},
		// --- Bob: codex medium, project scope ---
		{hBob, "warn", src, subjPath("/Users/bob/code/api/.codex/config.toml"), `{"kind":"ai_guard_risk_assessed","tool":"codex","scope":{"kind":"project","path":"/Users/bob/code/api"},"score":4.5,"bucket":"medium","reasons":[{"kind":"destructive_in_hook_script","pattern":"sudo rm"}],"is_reattestation":false}`},
		// --- Eve: continue_dev high, project scope ---
		{hEve, "warn", src, subjPath("/Users/eve/code/billing/.continue/config.json"), `{"kind":"ai_guard_risk_assessed","tool":"continue_dev","scope":{"kind":"project","path":"/Users/eve/code/billing"},"score":7.4,"bucket":"high","reasons":[{"kind":"mcp_server_remote","hook_event":"mcp_command"}],"is_reattestation":false}`},
		// --- Bob: gemini medium, user_global (3b.7) ---
		{hBob, "warn", src, subjPath("/Users/bob/.config/gemini-cli/config.json"), `{"kind":"ai_guard_risk_assessed","tool":"gemini","scope":{"kind":"user_global"},"score":3.8,"bucket":"medium","reasons":[{"kind":"mcp_server_remote","hook_event":"mcp_command"}],"is_reattestation":false}`},
		// --- Carol: cursor high, application scope (3b.7) ---
		{hCarol, "warn", src, subjPath("/home/carol/.cursor/settings.json"), `{"kind":"ai_guard_risk_assessed","tool":"cursor","scope":{"kind":"application","app":"cursor"},"score":6.2,"bucket":"high","reasons":[{"kind":"no_sandbox","executor":"mcp_command"}],"is_reattestation":false}`},

		// --- Non-AI-guard, host-meta + heartbeats + policy ---
		{hAlice, "info", srcAgent, subjAgent, `{"kind":"host_meta_snapshot","os_name":"macOS","os_version":"14.5","kernel_version":"23.5.0","architecture":"arm64","interfaces":[{"name":"en0","mac":"00:1b:44:11:3a:b7","ipv4":["192.168.1.42/24"],"ipv6":["fe80::1/64"]}],"default_gateway_v4":"192.168.1.1","default_gateway_v6":null,"dns_servers":["1.1.1.1","8.8.8.8"]}`},
		{hCarol, "info", srcAgent, subjAgent, `{"kind":"host_meta_snapshot","os_name":"Linux","os_version":"Ubuntu 24.04","kernel_version":"6.8.0-31-generic","architecture":"x86_64","interfaces":[{"name":"eth0","mac":"00:1b:44:22:bb:c1","ipv4":["10.0.5.21/24"],"ipv6":[]}],"default_gateway_v4":"10.0.5.1","default_gateway_v6":null,"dns_servers":["10.0.5.1"]}`},
		{hEve, "info", srcAgent, subjAgent, `{"kind":"host_meta_snapshot","os_name":"Windows","os_version":"11 23H2","kernel_version":"10.0.22631","architecture":"x86_64","interfaces":[{"name":"Ethernet0","mac":"00:1b:44:33:ee:42","ipv4":["172.16.4.18/16"],"ipv6":[]}],"default_gateway_v4":"172.16.0.1","default_gateway_v6":null,"dns_servers":["8.8.8.8","8.8.4.4"]}`},
		{hAlice, "info", srcAgent, subjAgent, `{"kind":"heartbeat","hash_p99_ms":4,"jsonl_above_soft_floor":false}`},
		{hBob, "info", srcAgent, subjAgent, `{"kind":"heartbeat","hash_p99_ms":3,"jsonl_above_soft_floor":false}`},
		{hCarol, "info", srcAgent, subjAgent, `{"kind":"heartbeat","hash_p99_ms":7,"jsonl_above_soft_floor":false}`},
		{hEve, "info", srcAgent, subjAgent, `{"kind":"heartbeat","hash_p99_ms":5,"jsonl_above_soft_floor":false}`},
		{hAlice, "info", srcAgent, subjAgent, `{"kind":"policy_reloaded","policy_version":18}`},
		{hBob, "info", srcAgent, subjAgent, `{"kind":"policy_reloaded","policy_version":17}`},

		// --- A few warn events that aren't ai_guard_risk_assessed (sender_lag etc.) ---
		{hAlice, "warn", srcAgent, subjAgent, `{"kind":"sender_lag_critical","queue_depth":1532}`},
		{hCarol, "warn", srcAgent, subjAgent, `{"kind":"agent_dying","reason":"oom"}`},
		{hEve, "warn", srcAgent, subjAgent, `{"kind":"tls_failure","peer":"sigil-server"}`},
		{hAlice, "warn", srcAgent, subjAgent, `{"kind":"host_id_fingerprint_drift","old":"abc","new":"def"}`},

		// --- Reattestations + file changes (info-level) ---
		{hAlice, "info", src, subjPath("/Users/alice/.claude/settings.json"), `{"kind":"ai_guard_risk_assessed","tool":"claude_code","scope":{"kind":"user_global"},"score":7.2,"bucket":"critical","reasons":[],"is_reattestation":true}`},
		{hEve, "info", src, subjPath("/Users/eve/.continue/config.json"), `{"kind":"ai_guard_risk_assessed","tool":"continue_dev","scope":{"kind":"user_global"},"score":8.4,"bucket":"critical","reasons":[],"is_reattestation":true}`},
		{hAlice, "info", src, subjPath("/Users/alice/code/work-repo/AGENTS.md"), `{"kind":"file_change","op":"modify","sha256":"deadbeef"}`},
		{hCarol, "info", src, subjPath("/home/carol/.local/share/config.yaml"), `{"kind":"file_change","op":"create","sha256":"cafef00d"}`},

		// --- Older AI guard events for time-window coverage ---
		{hBob, "warn", src, subjPath("/Users/bob/.codex/config.toml"), `{"kind":"ai_guard_risk_assessed","tool":"codex","scope":{"kind":"user_global"},"score":3.1,"bucket":"medium","reasons":[],"is_reattestation":false}`},
		{hCarol, "warn", src, subjPath("/home/carol/.config/Claude/claude_desktop_config.json"), `{"kind":"ai_guard_risk_assessed","tool":"claude_desktop","scope":{"kind":"application","app":"claude_desktop"},"score":5.6,"bucket":"high","reasons":[{"kind":"no_sandbox","executor":"mcp_command"}],"is_reattestation":false}`},
		{hAlice, "warn", src, subjPath("/Users/alice/.claude/hooks/preflight.sh"), `{"kind":"ai_guard_risk_assessed","tool":"claude_code","scope":{"kind":"user_global"},"score":6.5,"bucket":"high","reasons":[{"kind":"external_script_unscanned"}],"is_reattestation":false}`},
		{hEve, "warn", src, subjPath("/Users/eve/.continue/config.json"), `{"kind":"ai_guard_risk_assessed","tool":"continue_dev","scope":{"kind":"user_global"},"score":7.9,"bucket":"critical","reasons":[],"is_reattestation":false}`},

		// --- More info-level activity to round to 32 ---
		{hBob, "info", srcAgent, subjAgent, `{"kind":"heartbeat","hash_p99_ms":3,"jsonl_above_soft_floor":false}`},
		{hCarol, "info", src, subjPath("/etc/hosts"), `{"kind":"file_change","op":"modify","sha256":"01020304"}`},
		{hEve, "info", srcAgent, subjAgent, `{"kind":"policy_reloaded","policy_version":18}`},
	}
}

// Compile-time guard that MockClient and HTTPClient both satisfy Client.
var (
	_ Client = (*MockClient)(nil)
	_ Client = (*HTTPClient)(nil)
)
