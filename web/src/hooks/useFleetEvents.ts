import type { EventsPage, EventWithTriage } from '@/api/fleet';
import { type EventsParams, fleetEvents } from '@/api/fleet';
import { usePagedFleet } from './usePagedFleet';

export interface FleetEventsFilter {
  evidenceKinds: string[]; // empty = all kinds
  hostIDs: string[]; // empty = all hosts
  since: string | null;
  until?: string;
  tool?: string;
}

export const DEFAULT_FLEET_EVENTS_FILTER: FleetEventsFilter = {
  evidenceKinds: [],
  hostIDs: [],
  since: null,
};

export function useFleetEvents(filter: FleetEventsFilter) {
  const params: EventsParams = {
    limit: 100,
    evidence_kind: filter.evidenceKinds.length ? filter.evidenceKinds : undefined,
    host_id: filter.hostIDs.length ? filter.hostIDs : undefined,
    since: filter.since ?? undefined,
    until: filter.until,
  };
  const pages = usePagedFleet(
    ['fleet', 'events-timeline', params, filter.tool],
    (cursor, signal) => fleetEvents({ ...params, cursor }, signal),
    selectRows,
    rowID,
  );
  const tool = filter.tool?.toLowerCase();
  return {
    ...pages,
    rawCount: pages.rows.length,
    rows: tool
      ? pages.rows.filter(
          (row) => String(row.evidence.tool ?? row.evidence.agent ?? '').toLowerCase() === tool,
        )
      : pages.rows,
  };
}
const selectRows = (page: EventsPage) => page.events;
const rowID = (row: EventWithTriage) => `${row.host_id}:${row.event_id}`;
