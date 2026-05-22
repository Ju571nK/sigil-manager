import { type EventsParams, fleetEvents } from '@/api/fleet';
import { useFleetQuery } from './useFleetQuery';

export interface FleetEventsFilter {
  evidenceKinds: string[]; // empty = all kinds
  hostIDs: string[]; // empty = all hosts
  since: string | null;
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
  };
  const q = useFleetQuery(['fleet', 'events-timeline', params], () => fleetEvents(params));
  return {
    rows: q.data?.events ?? [],
    isPending: q.isPending && !q.data,
    error: q.error,
    isFetching: q.isFetching,
    lastUpdatedAt: q.dataUpdatedAt,
    refetch: q.refetch,
  };
}
