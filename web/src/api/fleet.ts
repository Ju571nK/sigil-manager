/**
 * Wraps the `/api/v1/fleet/*` handlers. Types mirror the Go server's wire
 * shape exactly (`internal/api/v1/fleet.go` + `internal/fleet/client.go`).
 * Plan 02 needs healthz, meta, events listing, and single-event lookup.
 */

import { api } from './client';

// -----------------------------------------------------------------------------
// Healthz + Meta
// -----------------------------------------------------------------------------

export interface HealthzResponse {
  status: string;
  ts: string;
}

/**
 * Optional license block on `/v1/meta` (contract §14.9.3). Absent on
 * open-source / older servers, so `MetaResponse.license` is optional.
 */
export interface LicenseStatus {
  state: 'ok' | 'over_limit' | string;
  licensed: boolean;
  expired: boolean;
  effective_max_hosts: number;
  current_host_count: number;
  active_window_days: number;
  customer_id: string | null;
  license_id: string | null;
  not_after: string | null;
}

/** Optional signed audit-log head on `/v1/meta` (§14.9.3); opaque to the console. */
export interface AuditHead {
  seq: number;
  hash: string;
  sig: string;
  pubkey_id: string;
  pubkey: string;
}

export interface MetaResponse {
  server_version: string;
  schema_version: number;
  ts: string;
  alerts_definition_default: {
    evidence_kinds: string[];
    ai_guard_buckets: string[];
    additional_kinds: string[];
  };
  license?: LicenseStatus;
  audit_head?: AuditHead | null;
}

export function fleetHealthz(): Promise<HealthzResponse> {
  return api<HealthzResponse>('/fleet/healthz');
}

export function fleetMeta(): Promise<MetaResponse> {
  return api<MetaResponse>('/fleet/meta');
}

// -----------------------------------------------------------------------------
// Events
// -----------------------------------------------------------------------------

/**
 * One row of the on-the-wire Event JSON. Mirrors `fleet.Event` in
 * internal/fleet/client.go — most fields are surfaced raw; the SPA
 * decodes `evidence` further via [`extractAiGuard`].
 */
export interface Event {
  schema_version: number;
  event_id: string;
  ts: string;
  host_id: string;
  agent_version: string;
  severity: 'info' | 'warn' | string; // future variants treated as warn
  source: unknown; // { kind: "file_system" | "agent" }
  subject: unknown;
  evidence: Evidence;
  target_id: string | null;
}

/** Discriminated evidence object. `kind` selects the variant shape. */
export interface Evidence {
  kind: string;
  [key: string]: unknown;
}

/** Decoded `ai_guard_risk_assessed` payload — Plan 02's main render path. */
export interface AiGuardEvidence extends Evidence {
  kind: 'ai_guard_risk_assessed';
  tool:
    | 'claude_code'
    | 'codex'
    | 'claude_desktop'
    | 'continue_dev'
    | 'antigravity'
    | 'grok'
    | 'other'
    | string;
  /**
   * Operator-supplied label for `tool === "other"` rule-pack matches
   * (contract §14.9.1). Absent for built-in tools. Pass to [`humanTool`] as
   * its second arg so an "Other" match shows the operator's name.
   */
  tool_label?: string;
  scope: ScopeUserGlobal | ScopeProject | ScopeApplication;
  score: number;
  bucket: 'low' | 'medium' | 'high' | 'critical';
  reasons: ReasonLike[];
  is_reattestation: boolean;
}

export interface ScopeUserGlobal {
  kind: 'user_global';
}
export interface ScopeProject {
  kind: 'project';
  path: string;
}
export interface ScopeApplication {
  kind: 'application';
  app: string;
}
export type Scope = ScopeUserGlobal | ScopeProject | ScopeApplication;

/** Reasons are open-shape; we surface kind + a few common fields. */
export interface ReasonLike {
  kind: string;
  pattern?: string;
  hook_event?: string;
  executor?: string;
  snippet?: string;
  [key: string]: unknown;
}

// -----------------------------------------------------------------------------
// sigil-hook evidence (contract §14.9.2)
// -----------------------------------------------------------------------------
// Four kinds emitted at the agent tool boundary. All carry `agent` (an AiTool
// wire string) and `peer_uid`; only hook_invocation carries `other_label` (the
// operator label for an `agent === "other"` match). Optional (`?`) fields are
// `Option` on the wire and arrive absent/null.

/** A runtime tool invocation observed by sigil-hook. */
export interface HookInvocationEvidence extends Evidence {
  kind: 'hook_invocation';
  agent: string;
  peer_uid: number;
  agent_session_id?: string;
  tool_use_id?: string;
  action_kind: string;
  other_label?: string;
  action_hash: string;
  action_preview?: string;
  capture_level: string;
  capture_status: string;
}

/** An allow/deny decision sigil-hook made at the tool boundary. */
export interface HookDecisionEvidence extends Evidence {
  kind: 'hook_decision';
  agent: string;
  peer_uid: number;
  agent_session_id?: string;
  tool_use_id?: string;
  action_kind: string;
  action_hash: string;
  action_preview?: string;
  decision: string;
  rule_id?: string;
  deny_reason?: string;
  enforcement_mode: string;
  capture_level: string;
}

/** The agent's on-disk hook config drifted from what sigil installed. */
export interface HookConfigDriftEvidence extends Evidence {
  kind: 'hook_config_drift';
  agent: string;
  peer_uid: number;
  drift_kind: string;
  settings_path: string;
  expected_command_hash: string;
  observed_command_hash?: string;
  expected_matcher?: string;
  observed_matcher?: string;
}

/** A session looked active but no hook fired in the window — possible bypass. */
export interface PossibleHookActivitySilentEvidence extends Evidence {
  kind: 'possible_hook_activity_silent';
  agent: string;
  uid?: number;
  last_hook_seen_at: string;
  last_session_activity_at?: string;
  window_secs: number;
  probe_kind: string;
  path_hash?: string;
  probe_error?: string;
  scan_truncated: boolean;
  confidence: 'low' | 'medium' | 'high' | string;
}

/** Union of the four sigil-hook evidence variants. */
export type HookEvidence =
  | HookInvocationEvidence
  | HookDecisionEvidence
  | HookConfigDriftEvidence
  | PossibleHookActivitySilentEvidence;

const HOOK_KINDS = new Set<string>([
  'hook_invocation',
  'hook_decision',
  'hook_config_drift',
  'possible_hook_activity_silent',
]);

/** Returns the typed [`HookEvidence`] view if the event is a hook kind, else null. */
export function extractHook(ev: Event): HookEvidence | null {
  return HOOK_KINDS.has(ev.evidence?.kind) ? (ev.evidence as HookEvidence) : null;
}

/** Server-side join: triage view embedded per event, or null if untriaged. */
export interface TriageView {
  status: 'open' | 'acknowledged' | 'investigating' | 'resolved';
  assignee: string | null;
  updated_at: string;
}

/** One row of `/api/v1/fleet/events.events[*]`. */
export interface EventWithTriage extends Event {
  triage: TriageView | null;
}

/** Page body of `/api/v1/fleet/events`. */
export interface EventsPage {
  events: EventWithTriage[];
  next_cursor: string | null;
}

/** Query params accepted by `/api/v1/fleet/events`. */
export interface EventsParams {
  cursor?: string;
  limit?: number;
  host_id?: string[]; // serialized as repeated ?host_id=a&host_id=b
  since?: string; // RFC3339
  until?: string; // RFC3339
  evidence_kind?: string[]; // comma-joined
  severity?: string[]; // comma-joined
  source?: string[]; // comma-joined
  min_ai_guard_bucket?: 'low' | 'medium' | 'high' | 'critical';
}

export function fleetEvents(params: EventsParams = {}): Promise<EventsPage> {
  const search = buildQuery(params);
  return api<EventsPage>(`/fleet/events${search ? `?${search}` : ''}`);
}

export function fleetEventByID(eventID: string): Promise<EventWithTriage> {
  return api<EventWithTriage>(`/fleet/events/${encodeURIComponent(eventID)}`);
}

// -----------------------------------------------------------------------------
// Fleet Risk (/api/v1/fleet/risk — contract §5.5)
// -----------------------------------------------------------------------------

/** One row of `/api/v1/fleet/risk.rows`. Mirrors fleet.RiskRow. */
export interface RiskRow {
  host_id: string;
  hostname: string | null;
  score: number; // 0.0–10.0 (F7)
  bucket: 'low' | 'medium' | 'high' | 'critical' | string;
  top_tool: string;
  reasons_count: number;
  assessed_ts: string;
  /**
   * Trailing-24h alert count. Per contract §14.9.4 (issue #21, sigil 8ebfd49)
   * the producer now filters this to the alerts-definition evidence kinds
   * (1:1 with /v1/meta) — it genuinely means "alerts in 24h". Recompute
   * client-side only if the operator overrides alerts_definition_default.
   */
  open_alert_count_24h: number;
}

export interface RiskPage {
  rows: RiskRow[];
  next_cursor: string | null;
}

export interface RiskParams {
  cursor?: string;
  limit?: number;
  tool?: string[]; // comma-joined
  min_bucket?: 'low' | 'medium' | 'high' | 'critical';
}

export function fleetRisk(params: RiskParams = {}): Promise<RiskPage> {
  const q = new URLSearchParams();
  if (params.cursor) q.set('cursor', params.cursor);
  if (typeof params.limit === 'number') q.set('limit', String(params.limit));
  if (params.tool?.length) q.set('tool', params.tool.join(','));
  if (params.min_bucket) q.set('min_bucket', params.min_bucket);
  const s = q.toString();
  return api<RiskPage>(`/fleet/risk${s ? `?${s}` : ''}`);
}

// -----------------------------------------------------------------------------
// Fleet Compliance (/api/v1/fleet/compliance — contract §5.6)
// -----------------------------------------------------------------------------

/** One row of `/api/v1/fleet/compliance.rows`. Mirrors fleet.ComplianceRow. */
export interface ComplianceRow {
  host_id: string;
  hostname: string | null;
  last_applied_policy_version: number;
  server_current_policy_version: number;
  version_drift: number;
  policy_expired_active: boolean;
  last_policy_reload_ts: string | null;
  signature_failures_24h: number;
}

export interface CompliancePage {
  rows: ComplianceRow[];
  next_cursor: string | null;
}

export interface ComplianceParams {
  cursor?: string;
  limit?: number;
}

export function fleetCompliance(params: ComplianceParams = {}): Promise<CompliancePage> {
  const q = new URLSearchParams();
  if (params.cursor) q.set('cursor', params.cursor);
  if (typeof params.limit === 'number') q.set('limit', String(params.limit));
  const s = q.toString();
  return api<CompliancePage>(`/fleet/compliance${s ? `?${s}` : ''}`);
}

// -----------------------------------------------------------------------------
// Host detail (/fleet/hosts/{host_id} — contract §5.4)
// -----------------------------------------------------------------------------

// Array fields are Go slices on the wire — a nil slice marshals to JSON null,
// not [], so every array here is nullable and consumers must guard.
export interface NetInterface {
  name: string;
  mac: string | null;
  ipv4: string[] | null;
  ipv6: string[] | null;
}

export interface HostMeta {
  os_name: string;
  os_version: string;
  kernel_version: string;
  architecture: string;
  interfaces: NetInterface[] | null;
  default_gateway_v4: string | null;
  default_gateway_v6: string | null;
  dns_servers: string[] | null;
}

export interface PolicyState {
  last_applied_policy_version: number;
  policy_expired_active: boolean;
  last_policy_reload_ts: string | null;
}

export interface AgentHealth {
  recent_channel_stalls_24h: number;
  recent_watcher_degraded_24h: number;
  recent_sender_lag_critical_24h: number;
  last_heartbeat_ts: string | null;
  hash_p99_ms_latest: number | null;
  jsonl_above_soft_floor_latest: boolean | null;
}

/** One per-tool AI Guard rollup (§5.4 ai_guard.by_tool). Same reason/scope
 * shapes as AiGuardEvidence, so it reuses ReasonLike/Scope. */
export interface ToolAiGuard {
  score: number;
  bucket: 'low' | 'medium' | 'high' | 'critical' | string;
  assessed_ts: string;
  is_reattestation: boolean;
  scope: Scope;
  // Nullable on the wire: a Go nil reasons slice serializes to `null`, so a
  // tool with no reasons arrives as null, not []. Consumers must guard.
  reasons: ReasonLike[] | null;
}

/** Per-tool current risk rollup embedded in current_risk (§5.4). */
export interface ToolRisk {
  score: number;
  bucket: 'low' | 'medium' | 'high' | 'critical' | string;
  assessed_ts: string;
}

export interface CurrentRisk {
  max_score: number;
  max_bucket: 'low' | 'medium' | 'high' | 'critical' | string;
  by_tool: Record<string, ToolRisk>;
}

/** Body of GET /fleet/hosts/{host_id} (§5.4): HostSummary + 4 nullable blocks. */
export interface HostDetail {
  host_id: string;
  hostname: string | null;
  agent_version: string;
  last_seen_ts: string;
  status: 'healthy' | 'stale' | 'disconnected' | string;
  current_risk: CurrentRisk | null;
  open_event_counts_24h: Record<string, number>;
  host_meta: HostMeta | null;
  policy_state: PolicyState | null;
  agent_health: AgentHealth | null;
  ai_guard: { by_tool: Record<string, ToolAiGuard> } | null;
}

export function fleetHost(hostId: string): Promise<HostDetail> {
  return api<HostDetail>(`/fleet/hosts/${encodeURIComponent(hostId)}`);
}

// -----------------------------------------------------------------------------
// Helpers
// -----------------------------------------------------------------------------

function buildQuery(p: EventsParams): string {
  const params = new URLSearchParams();
  if (p.cursor) params.set('cursor', p.cursor);
  if (typeof p.limit === 'number') params.set('limit', String(p.limit));
  if (p.since) params.set('since', p.since);
  if (p.until) params.set('until', p.until);
  if (p.evidence_kind?.length) params.set('evidence_kind', p.evidence_kind.join(','));
  if (p.severity?.length) params.set('severity', p.severity.join(','));
  if (p.source?.length) params.set('source', p.source.join(','));
  if (p.min_ai_guard_bucket) params.set('min_ai_guard_bucket', p.min_ai_guard_bucket);
  if (p.host_id?.length) {
    // Server accepts both comma-list and repeated `host_id=`; we use repeated.
    for (const h of p.host_id) params.append('host_id', h);
  }
  return params.toString();
}

/** Returns the [`AiGuardEvidence`] view if the event matches, else null. */
export function extractAiGuard(ev: Event): AiGuardEvidence | null {
  if (ev.evidence?.kind !== 'ai_guard_risk_assessed') return null;
  return ev.evidence as AiGuardEvidence;
}
