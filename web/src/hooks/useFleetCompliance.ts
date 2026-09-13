import type { CompliancePage, ComplianceRow } from '@/api/fleet';
import { fleetCompliance } from '@/api/fleet';
import { usePagedFleet } from './usePagedFleet';

export function useFleetCompliance() {
  // Keep the params in the cache key so they can't collide if a cursor/limit/
  // host_id filter is ever added (contract §10 host-side filter). Constant today.
  const params = { limit: 100 };
  return usePagedFleet(
    ['fleet', 'compliance', params],
    (cursor, signal) => fleetCompliance({ ...params, cursor }, signal),
    selectRows,
    rowID,
  );
}
const selectRows = (page: CompliancePage) => page.rows;
const rowID = (row: ComplianceRow) => row.host_id;
