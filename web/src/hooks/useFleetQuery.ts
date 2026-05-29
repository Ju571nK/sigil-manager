import { keepPreviousData, type QueryKey, useQuery } from '@tanstack/react-query';

/** Fleet tabs poll slower than the alert queue (UI/UX §7.2 sets 5s for alerts). */
export const FLEET_POLL_INTERVAL_MS = 20_000;

type FleetQueryOptions = {
  /** Override react-query's default retry (e.g. don't retry a 404). */
  retry?: boolean | number | ((failureCount: number, error: Error) => boolean);
};

/**
 * Thin wrapper over useQuery with the fleet-page polling defaults applied.
 * Keeps the fleet hooks consistent and makes the interval one edit. `options`
 * is spread last so callers can override (existing callers pass none).
 *
 * `placeholderData: keepPreviousData` keeps the last result visible while a
 * key change (filter chip, host navigation) refetches, so the table refreshes
 * in place instead of flashing the skeleton/empty state for a fetch cycle.
 */
export function useFleetQuery<T>(
  key: QueryKey,
  queryFn: () => Promise<T>,
  options?: FleetQueryOptions,
) {
  return useQuery({
    queryKey: key,
    queryFn,
    refetchInterval: FLEET_POLL_INTERVAL_MS,
    refetchIntervalInBackground: false,
    placeholderData: keepPreviousData,
    ...options,
  });
}
