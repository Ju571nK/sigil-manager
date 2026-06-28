import { useQuery } from '@tanstack/react-query';
import { fleetMeta } from '@/api/fleet';

// /v1/meta changes rarely (alerts definition, license counts) — cache 5 min.
const META_STALE_MS = 5 * 60_000;

/**
 * Shared `/v1/meta` query. Uses the same `['fleet','meta']` key as
 * [`useAlerts`], so the license banner / fleet header reuse the cached
 * response instead of firing a second request.
 */
export function useFleetMeta() {
  return useQuery({
    queryKey: ['fleet', 'meta'],
    queryFn: fleetMeta,
    staleTime: META_STALE_MS,
  });
}
