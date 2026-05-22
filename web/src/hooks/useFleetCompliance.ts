import { fleetCompliance } from '@/api/fleet';
import { useFleetQuery } from './useFleetQuery';

export function useFleetCompliance() {
  const q = useFleetQuery(['fleet', 'compliance'], () => fleetCompliance({ limit: 100 }));
  return {
    rows: q.data?.rows ?? [],
    isPending: q.isPending && !q.data,
    error: q.error,
    isFetching: q.isFetching,
    lastUpdatedAt: q.dataUpdatedAt,
    refetch: q.refetch,
  };
}
