import { type QueryKey, useQuery } from '@tanstack/react-query';

/** Fleet tabs poll slower than the alert queue (UI/UX §7.2 sets 5s for alerts). */
export const FLEET_POLL_INTERVAL_MS = 20_000;

/**
 * Thin wrapper over useQuery with the fleet-page polling defaults applied.
 * Keeps the three fleet hooks consistent and makes the interval one edit.
 */
export function useFleetQuery<T>(key: QueryKey, queryFn: () => Promise<T>) {
  return useQuery({
    queryKey: key,
    queryFn,
    refetchInterval: FLEET_POLL_INTERVAL_MS,
    refetchIntervalInBackground: false,
  });
}
