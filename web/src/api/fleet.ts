/**
 * Wraps the `/api/v1/fleet/*` handlers — the Plan 02 surface only needs
 * /healthz (for the TopNav connection pill) and /meta (boot probe).
 * Events lives in [`./events`] (Plan 02 T11).
 */

import { api } from './client';

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

/** GET /api/v1/fleet/healthz — sigil-server liveness, proxied. */
export function fleetHealthz(): Promise<HealthzResponse> {
  return api<HealthzResponse>('/fleet/healthz');
}

/** GET /api/v1/fleet/meta — version + alerts definition. */
export function fleetMeta(): Promise<MetaResponse> {
  return api<MetaResponse>('/fleet/meta');
}
