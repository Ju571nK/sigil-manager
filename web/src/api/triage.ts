/** Wraps the `/api/v1/triage/*` handlers (consumer-local triage state). */

import { api } from './client';

export type TriageStatus = 'open' | 'acknowledged' | 'investigating' | 'resolved';

export interface TriageRow {
  host_id: string;
  event_id: string;
  status: TriageStatus;
  assignee: string | null;
  evidence_snapshot: unknown;
  created_at: string;
  updated_at: string;
}

export interface TriageNote {
  id: number;
  host_id: string;
  event_id: string;
  author: string;
  body: string;
  created_at: string;
}

export interface TriageLogEntry {
  id: number;
  actor: string;
  from_status: TriageStatus | null;
  to_status: TriageStatus | null;
  at: string;
}

export interface TriageDetail {
  row: TriageRow;
  notes: TriageNote[];
  log: TriageLogEntry[];
}

export interface UpsertPayload {
  host_id: string;
  event_id: string;
  status?: TriageStatus;
  assignee?: string;
  clear_assignee?: boolean;
  /** Required on first upsert; ignored on subsequent updates server-side. */
  evidence_snapshot?: unknown;
}

export interface NotePayload {
  host_id: string;
  event_id: string;
  body: string;
}

/** GET /api/v1/triage/:host_id/:event_id — row + notes + log. */
export function getTriage(hostID: string, eventID: string): Promise<TriageDetail> {
  return api<TriageDetail>(`/triage/${encodeURIComponent(hostID)}/${encodeURIComponent(eventID)}`);
}

/** POST /api/v1/triage/upsert — create or update the triage row. */
export function upsertTriage(payload: UpsertPayload): Promise<TriageRow> {
  return api<TriageRow>('/triage/upsert', {
    method: 'POST',
    body: JSON.stringify(payload),
  });
}

/** POST /api/v1/triage/note — append a note. Triage row must exist. */
export function appendNote(payload: NotePayload): Promise<TriageNote> {
  return api<TriageNote>('/triage/note', {
    method: 'POST',
    body: JSON.stringify(payload),
  });
}
