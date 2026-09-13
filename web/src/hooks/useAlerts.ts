import { useCallback, useMemo, useRef, useState } from 'react';
import type { EventsPage } from '@/api/fleet';
import { type EventsParams, type EventWithTriage, fleetEvents } from '@/api/fleet';
import { useFleetMeta } from './useFleetMeta';
import { usePagedFleet } from './usePagedFleet';

const POLL_INTERVAL_MS = 5_000; // UI/UX §7.2: 5s queue refresh

/** Filter state owned by the Alerts page; lives in the URL (route.search). */
export interface AlertFilter {
  /** Server-side filter; matches contract §5.7 `min_ai_guard_bucket`. */
  minBucket: 'low' | 'medium' | 'high' | 'critical';
  /** Client-side filter on the JOINED triage status. Empty = all statuses. */
  triageStatuses: Array<'open' | 'acknowledged' | 'investigating' | 'resolved'>;
  /** Server-side `?since=` cutoff. `null` = no lower bound. */
  since: string | null;
  /** Client-side fuzzy match against event_id / host / evidence kind. */
  query: string;
}

export const DEFAULT_FILTER: AlertFilter = {
  minBucket: 'high',
  triageStatuses: [],
  since: null,
  query: '',
};

/**
 * Polls `/api/v1/fleet/events` every 5s with the alerts definition from
 * `/api/v1/fleet/meta`. Pauses refetch while the user hovers a row (caller
 * wires `onRowHoverEnter` / `onRowHoverLeave` to the table) so the queue
 * doesn't reorder under the mouse.
 *
 * Server-side filters: evidence_kinds (from meta + additional_kinds),
 * min_ai_guard_bucket, since. Status + query are applied client-side after
 * the server returns rows.
 */
export function useAlerts(filter: AlertFilter) {
  const meta = useFleetMeta();

  // Hover pause: a ref + setState so onRowHover* call sites stay stable.
  const [paused, setPaused] = useState(false);
  const hoverDepth = useRef(0);
  const onRowHoverEnter = useCallback(() => {
    hoverDepth.current += 1;
    setPaused(true);
  }, []);
  const onRowHoverLeave = useCallback(() => {
    hoverDepth.current = Math.max(0, hoverDepth.current - 1);
    if (hoverDepth.current === 0) setPaused(false);
  }, []);

  const params = useMemo<EventsParams>(() => {
    const m = meta.data?.alerts_definition_default;
    const evidenceKinds = m
      ? Array.from(new Set([...(m.evidence_kinds ?? []), ...(m.additional_kinds ?? [])]))
      : ['ai_guard_risk_assessed'];
    return {
      limit: 100,
      evidence_kind: evidenceKinds,
      min_ai_guard_bucket: filter.minBucket,
      since: filter.since ?? undefined,
    };
  }, [meta.data, filter.minBucket, filter.since]);

  const events = usePagedFleet(
    ['fleet', 'events', params, filter.triageStatuses, filter.query],
    (cursor, signal) => fleetEvents({ ...params, cursor }, signal),
    selectEvents,
    eventID,
    { enabled: !!meta.data, interval: paused ? false : POLL_INTERVAL_MS },
  );

  // Client-side filters: triage status + free-text query.
  const visible = useMemo(() => {
    const all = events.rows;
    return applyClientFilter(all, filter);
  }, [events.rows, filter]);

  return {
    ...events,
    rows: visible,
    rawCount: events.rows.length,
    meta: meta.data,
    isPending: meta.isPending || (!!meta.data && events.isPending),
    error: meta.error ?? events.error,
    lastUpdatedAt: events.lastUpdatedAt,
    isFetching: events.isFetching,
    isPaused: paused,
    onRowHoverEnter,
    onRowHoverLeave,
    refetch: async () => {
      await meta.refetch();
      if (meta.data) await events.refetch();
    },
  };
}

function applyClientFilter(rows: EventWithTriage[], f: AlertFilter): EventWithTriage[] {
  let out = rows;
  if (f.triageStatuses.length > 0) {
    out = out.filter((ev) => {
      const s = ev.triage?.status ?? 'open';
      return f.triageStatuses.includes(s);
    });
  }
  const q = f.query.trim().toLowerCase();
  if (q.length > 0) {
    out = out.filter((ev) => {
      if (ev.event_id.toLowerCase().includes(q)) return true;
      if (ev.host_id.toLowerCase().includes(q)) return true;
      if (ev.evidence?.kind?.toLowerCase().includes(q)) return true;
      return false;
    });
  }
  return out;
}

const selectEvents = (page: EventsPage) => page.events;
const eventID = (row: EventWithTriage) => `${row.host_id}:${row.event_id}`;
