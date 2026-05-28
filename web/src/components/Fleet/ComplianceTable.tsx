import type { ComplianceRow } from '@/api/fleet';
import { CompliancePill } from '@/components/CompliancePill';
import { SkeletonRows } from '@/components/Fleet/SkeletonRows';
import { deriveComplianceStatus } from '@/lib/compliance';
import { relativeAge } from '@/lib/time';

interface Props {
  rows: ComplianceRow[];
  isPending: boolean;
}

export function ComplianceTable({ rows, isPending }: Props) {
  if (isPending) {
    return <SkeletonRows />;
  }
  if (rows.length === 0) {
    return (
      <div className="px-4 py-8 text-center text-sm text-text-muted">
        No hosts reporting policy state yet.
      </div>
    );
  }
  return (
    <table className="w-full text-xs">
      <thead className="text-text-subtle">
        <tr className="border-b border-border-subtle text-left">
          <th className="px-3 py-2 font-medium">Host</th>
          <th className="px-3 py-2 font-medium">Status</th>
          <th className="px-3 py-2 font-medium">Policy version</th>
          <th className="px-3 py-2 font-medium">Sig failures 24h</th>
          <th className="px-3 py-2 font-medium">Last reload</th>
        </tr>
      </thead>
      <tbody>
        {rows.map((row) => {
          const status = deriveComplianceStatus(row);
          return (
            <tr key={row.host_id} className="border-b border-border-subtle">
              <td className="px-3 py-2 font-mono text-text-primary" title={row.host_id}>
                {row.hostname ?? row.host_id.split('-')[0]}
              </td>
              <td className="px-3 py-2">
                <CompliancePill status={status} />
              </td>
              <td className="px-3 py-2 font-mono text-text-muted">
                {row.last_applied_policy_version}
                {row.version_drift > 0 && (
                  <span className="text-status-degraded">
                    {' '}
                    → {row.server_current_policy_version}
                  </span>
                )}
              </td>
              <td className="px-3 py-2 font-mono text-text-muted">{row.signature_failures_24h}</td>
              <td className="px-3 py-2 text-text-muted">{reloadCell(row.last_policy_reload_ts)}</td>
            </tr>
          );
        })}
      </tbody>
    </table>
  );
}

function reloadCell(ts: string | null): string {
  if (!ts) return '—';
  const age = relativeAge(ts);
  return age === '—' ? '—' : `${age} ago`;
}
