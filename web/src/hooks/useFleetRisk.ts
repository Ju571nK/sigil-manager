import { fleetRisk, type RiskParams } from '@/api/fleet';
import { useFleetQuery } from './useFleetQuery';

export interface RiskFilter {
  minBucket: 'low' | 'medium' | 'high' | 'critical';
  tool: string[]; // empty = all tools
}

export const DEFAULT_RISK_FILTER: RiskFilter = { minBucket: 'low', tool: [] };

export function useFleetRisk(filter: RiskFilter) {
  const params: RiskParams = {
    limit: 100,
    min_bucket: filter.minBucket,
    tool: filter.tool.length ? filter.tool : undefined,
  };
  const q = useFleetQuery(['fleet', 'risk', params], () => fleetRisk(params));
  return {
    rows: q.data?.rows ?? [],
    isPending: q.isPending && !q.data,
    error: q.error,
    isFetching: q.isFetching,
    lastUpdatedAt: q.dataUpdatedAt,
    refetch: q.refetch,
  };
}
