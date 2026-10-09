import type { FleetSummary } from '@/api/fleet';

/** "N active hosts · last D days" for the fleet header, or null when not reported. */
export function activeHostSummary(fleet: FleetSummary | null | undefined): string | null {
  if (!fleet) return null;
  const hosts = fleet.active_host_count === 1 ? 'host' : 'hosts';
  const days = fleet.active_window_days === 1 ? 'day' : 'days';
  return `${fleet.active_host_count} active ${hosts} · last ${fleet.active_window_days} ${days}`;
}
