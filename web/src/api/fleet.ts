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

export interface MetaResponse {
  server_version: string;
  schema_version: number;
  ts: string;
  alerts_definition_default: {
    evidence_kinds: string[];
    ai_guard_buckets: string[];
    additional_kinds: string[];
  };
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
  tool: 'claude_code' | 'codex' | 'claude_desktop' | 'continue_dev' | string;
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
   * Trailing-24h warn-event count. Per contract §13.1 / issue #21 this is
   * NOT alert-definition filtered — render it as "Warn 24h", not "alerts".
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

export interface NetInterface {
  name: string;
  mac: string | null;
  ipv4: string[];
  ipv6: string[];
}

export interface HostMeta {
  os_name: string;
  os_version: string;
  kernel_version: string;
  architecture: string;
  interfaces: NetInterface[];
  default_gateway_v4: string | null;
  default_gateway_v6: string | null;
  dns_servers: string[];
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
