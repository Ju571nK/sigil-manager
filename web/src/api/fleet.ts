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
