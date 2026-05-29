import { fleetCompliance } from '@/api/fleet';
import { useFleetQuery } from './useFleetQuery';

export function useFleetCompliance() {
  // Keep the params in the cache key so they can't collide if a cursor/limit/
  // host_id filter is ever added (contract §10 host-side filter). Constant today.
  const params = { limit: 100 };
  const q = useFleetQuery(['fleet', 'compliance', params], () => fleetCompliance(params));
  return {
    rows: q.data?.rows ?? [],
    isPending: q.isPending && !q.data,
    error: q.error,
    isFetching: q.isFetching,
    lastUpdatedAt: q.dataUpdatedAt,
    refetch: q.refetch,
  };
}
