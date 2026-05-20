import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { NotFoundError } from '@/api/client';
import {
  appendNote,
  getTriage,
  type NotePayload,
  type TriageDetail,
  type TriageStatus,
  type UpsertPayload,
  upsertTriage,
} from '@/api/triage';

/**
 * Triage hooks for the slide-over. Reads + writes go through TanStack
 * Query so the rest of the SPA (Alerts queue rows in particular) get
 * automatic re-render after every mutation.
 *
 * The queue's `useAlerts` lives under the ['fleet','events'] key tree;
 * mutations here invalidate ['fleet','events'] so the joined `triage`
 * block on each row updates immediately after Ack/Resolve/Assign.
 */

export function useTriageDetail(hostID: string | null, eventID: string | null) {
  return useQuery({
    queryKey: ['triage', hostID, eventID],
    queryFn: () => {
      if (!hostID || !eventID) throw new Error('hook called without ids');
      return getTriage(hostID, eventID);
    },
    enabled: !!hostID && !!eventID,
    // Triage row may not exist yet — surface NotFoundError as data:null
    // rather than spamming the boundary every time the user opens an
    // un-actioned alert.
    retry: (failureCount, error) => {
      if (error instanceof NotFoundError) return false;
      return failureCount < 2;
    },
  });
}

export function useUpsertTriage() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (payload: UpsertPayload) => upsertTriage(payload),
    onSuccess: (row) => {
      qc.invalidateQueries({ queryKey: ['triage', row.host_id, row.event_id] });
      qc.invalidateQueries({ queryKey: ['fleet', 'events'] });
    },
  });
}

export function useAppendNote() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (payload: NotePayload) => appendNote(payload),
    onSuccess: (note) => {
      qc.invalidateQueries({ queryKey: ['triage', note.host_id, note.event_id] });
    },
  });
}

/**
 * Convenience: full triage detail with NotFoundError flattened into
 * data===null + error===null. Slide-over callers can then `if (!detail)`
 * branch into the "no triage yet" state without juggling error types.
 */
export function useTriageDetailOrNull(
  hostID: string | null,
  eventID: string | null,
): { data: TriageDetail | null; isPending: boolean; status: TriageStatus | null } {
  const q = useTriageDetail(hostID, eventID);
  if (q.error instanceof NotFoundError) {
    return { data: null, isPending: false, status: 'open' };
  }
  return {
    data: q.data ?? null,
    isPending: q.isPending,
    status: q.data?.row.status ?? null,
  };
}
