import { NotFoundError } from '@/api/client';
import { fleetHost } from '@/api/fleet';
import { useFleetQuery } from './useFleetQuery';

/**
 * Single host detail (§5.4) at the fleet poll interval. A 404 (host evicted
 * from the server index) surfaces immediately — we don't retry it — so the
 * route can paint its "host not found" panel without a multi-second stall.
 */
export function useFleetHost(hostId: string) {
  const q = useFleetQuery(['fleet', 'host', hostId], () => fleetHost(hostId), {
    retry: (failureCount, error) => !(error instanceof NotFoundError) && failureCount < 2,
  });
  return {
    host: q.data,
    isPending: q.isPending && !q.data,
    error: q.error,
    isFetching: q.isFetching,
    lastUpdatedAt: q.dataUpdatedAt,
    refetch: q.refetch,
  };
}
