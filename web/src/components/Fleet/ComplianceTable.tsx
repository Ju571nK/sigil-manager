import { formatDistanceToNowStrict } from 'date-fns';
import type { ComplianceRow } from '@/api/fleet';
import { COMPLIANCE_META, deriveComplianceStatus } from '@/lib/compliance';
import { cn } from '@/lib/utils';

interface Props {
  rows: ComplianceRow[];
  isPending: boolean;
}

export function ComplianceTable({ rows, isPending }: Props) {
  if (isPending) {
    return (
      <div aria-hidden className="space-y-2 px-3 py-3">
        {Array.from({ length: 5 }).map((_, i) => (
          // biome-ignore lint/suspicious/noArrayIndexKey: static skeleton placeholders
          <div key={i} className="h-3 w-full animate-pulse rounded bg-bg-elevated" />
        ))}
      </div>
    );
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
          const meta = COMPLIANCE_META[status];
          return (
            <tr key={row.host_id} className="border-b border-border-subtle">
              <td className="px-3 py-2 font-mono text-text-primary" title={row.host_id}>
                {row.hostname ?? row.host_id.split('-')[0]}
              </td>
              <td className="px-3 py-2">
                <StatusPill tone={meta.tone} label={meta.label} />
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
              <td className="px-3 py-2 text-text-muted">
                {row.last_policy_reload_ts
                  ? `${formatDistanceToNowStrict(new Date(row.last_policy_reload_ts))} ago`
                  : '—'}
              </td>
            </tr>
          );
        })}
      </tbody>
    </table>
  );
}

function StatusPill({ tone, label }: { tone: 'healthy' | 'degraded' | 'down'; label: string }) {
  const toneClass =
    tone === 'healthy'
      ? 'text-status-healthy border-status-healthy/40 bg-status-healthy/10'
      : tone === 'degraded'
        ? 'text-status-degraded border-status-degraded/40 bg-status-degraded/10'
        : 'text-status-down border-status-down/40 bg-status-down/10';
  return (
    <span
      className={cn(
        'inline-block rounded border px-1.5 py-px text-[10px] uppercase tracking-wide',
        toneClass,
      )}
    >
      {label}
    </span>
  );
}
