import { createFileRoute, Link } from '@tanstack/react-router';
import { useState } from 'react';
import { fleetHosts, type HostSummary, type HostsPage } from '@/api/fleet';
import { Pagination } from '@/components/Fleet/Pagination';
import { SkeletonRows } from '@/components/Fleet/SkeletonRows';
import { usePagedFleet } from '@/hooks/usePagedFleet';
import { relativeAge } from '@/lib/time';

const statuses = ['healthy', 'stale', 'disconnected'] as const;
type Status = (typeof statuses)[number];
interface DeviceSearch {
  status?: Status;
}
export const Route = createFileRoute('/_authed/fleet/devices')({
  validateSearch: (raw: Record<string, unknown>): DeviceSearch => ({
    status: statuses.includes(raw.status as Status) ? (raw.status as Status) : undefined,
  }),
  component: DevicesTab,
});
const selectHosts = (page: HostsPage) => page.hosts;
const hostID = (host: HostSummary) => host.host_id;
const normalizeVersion = (version: string) => version.trim().replace(/^v/, '');

function DevicesTab() {
  const { status } = Route.useSearch();
  const navigate = Route.useNavigate();
  const [search, setSearch] = useState('');
  const [target, setTarget] = useState('');
  const params = { status, limit: 100, sort: 'host_id' as const };
  const devices = usePagedFleet(
    ['fleet', 'hosts', params],
    (cursor, signal) => fleetHosts({ ...params, cursor }, signal),
    selectHosts,
    hostID,
  );
  const needle = search.trim().toLowerCase();
  const rows = devices.rows.filter((host) =>
    [host.host_id, host.hostname ?? '', host.agent_version].some((value) =>
      value.toLowerCase().includes(needle),
    ),
  );
  const targetVersion = normalizeVersion(target);
  const inputClass = 'rounded border border-border bg-bg-surface px-2 py-1 text-sm';
  return (
    <div className="space-y-3">
      <p className="text-sm text-text-muted">
        Hosts observed by Sigil, including devices without a risk assessment. This is not a complete
        corporate asset inventory.
      </p>
      <div className="flex flex-wrap items-end gap-3 text-xs text-text-muted">
        <label className="grid gap-1">
          Connection status
          <select
            className={inputClass}
            value={status ?? ''}
            onChange={(event) =>
              navigate({
                search: { status: (event.target.value || undefined) as Status | undefined },
              })
            }
          >
            <option value="">All statuses</option>
            {statuses.map((value) => (
              <option key={value} value={value}>
                {value}
              </option>
            ))}
          </select>
        </label>
        <label className="grid gap-1">
          Search loaded devices
          <input
            className={inputClass}
            value={search}
            onChange={(event) => setSearch(event.target.value)}
            placeholder="Hostname, host ID or version"
          />
        </label>
        <label className="grid gap-1">
          Target agent version
          <input
            className={inputClass}
            value={target}
            onChange={(event) => setTarget(event.target.value)}
            placeholder="e.g. 0.8.3"
          />
        </label>
      </div>
      <p className="text-xs text-text-muted">
        Status applies across the server results; search applies to loaded devices only. Versions
        are last reported, not verified installations. Target comparison is an exact match (ignoring
        a leading v), not an upgrade recommendation; the target is temporary for this view.
      </p>
      <div className="overflow-x-auto rounded-md border border-border bg-bg-surface">
        {devices.isPending ? (
          <SkeletonRows />
        ) : rows.length === 0 ? (
          <p className="p-6 text-sm text-text-muted">
            {devices.error
              ? 'Device data is unavailable.'
              : devices.rows.length || devices.hasMore
                ? 'No matching devices in loaded results.'
                : 'No observed devices for this status.'}
          </p>
        ) : (
          <table className="w-full text-left text-xs">
            <thead className="text-text-muted">
              <tr>
                {[
                  'Device / Host ID',
                  'Connection',
                  'Last seen',
                  'Reported agent',
                  ...(targetVersion ? ['Target comparison'] : []),
                  'Risk',
                ].map((label) => (
                  <th key={label} className="px-3 py-2 font-medium">
                    {label}
                  </th>
                ))}
              </tr>
            </thead>
            <tbody>
              {rows.map((host) => (
                <tr key={host.host_id} className="border-t border-border-subtle">
                  <td className="px-3 py-2">
                    <Link
                      to="/hosts/$hostId"
                      params={{ hostId: host.host_id }}
                      className="text-accent hover:underline"
                    >
                      {host.hostname || 'Unnamed device'}
                    </Link>
                    <div className="mt-1 font-mono text-text-muted">{host.host_id}</div>
                  </td>
                  <td className="px-3 py-2">{host.status}</td>
                  <td className="px-3 py-2" title={host.last_seen_ts}>
                    {relativeAge(host.last_seen_ts)} ago
                  </td>
                  <td className="px-3 py-2 font-mono">{host.agent_version || 'Not reported'}</td>
                  {targetVersion && (
                    <td className="px-3 py-2">
                      {!host.agent_version
                        ? 'Unknown'
                        : normalizeVersion(host.agent_version) === targetVersion
                          ? 'Matches target'
                          : 'Differs from target'}
                    </td>
                  )}
                  <td className="px-3 py-2">
                    {host.current_risk
                      ? `${host.current_risk.max_bucket} · ${host.current_risk.max_score.toFixed(1)}`
                      : 'Not assessed'}
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        )}
        <Pagination {...devices} count={devices.rows.length} />
      </div>
    </div>
  );
}
