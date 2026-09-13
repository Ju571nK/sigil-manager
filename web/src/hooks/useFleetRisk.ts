import type { RiskPage, RiskRow } from '@/api/fleet';
import { fleetRisk, type RiskParams } from '@/api/fleet';
import { usePagedFleet } from './usePagedFleet';

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
  return usePagedFleet(
    ['fleet', 'risk', params],
    (cursor, signal) => fleetRisk({ ...params, cursor }, signal),
    selectRows,
    rowID,
  );
}
const selectRows = (page: RiskPage) => page.rows;
const rowID = (row: RiskRow) => row.host_id;
