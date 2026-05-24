import { type EventWithTriage, extractAiGuard } from '@/api/fleet';
import { SkeletonRows } from '@/components/Fleet/SkeletonRows';
import { humanKind, humanTool } from '@/lib/labels';
import { relativeAge } from '@/lib/time';
import { cn } from '@/lib/utils';

interface Props {
  rows: EventWithTriage[];
  isPending: boolean;
}

/** Fleet-wide event timeline (UI/UX §5.2 Events tab). No triage columns. */
export function EventsTable({ rows, isPending }: Props) {
  if (isPending) {
    return <SkeletonRows count={6} />;
  }
  if (rows.length === 0) {
    return (
      <div className="px-4 py-8 text-center text-sm text-text-muted">
        No events in the selected range.
      </div>
    );
  }
  return (
    <table className="w-full text-xs">
      <thead className="text-text-subtle">
        <tr className="border-b border-border-subtle text-left">
          <th className="px-3 py-2 font-medium">Sev</th>
          <th className="px-3 py-2 font-medium">Age</th>
          <th className="px-3 py-2 font-medium">Kind</th>
          <th className="px-3 py-2 font-medium">Host</th>
          <th className="px-3 py-2 font-medium">Tool</th>
        </tr>
      </thead>
      <tbody>
        {rows.map((ev) => {
          const ag = extractAiGuard(ev);
          return (
            <tr key={ev.event_id} className="border-b border-border-subtle">
              <td className="px-3 py-2">
                {/* Contract treats unknown/future severities as warn, not info
                    (api/fleet.ts) — so only an explicit "info" is the low dot. */}
                <span
                  role="img"
                  title={ev.severity === 'info' ? 'Info' : 'Warning'}
                  aria-label={ev.severity === 'info' ? 'Info' : 'Warning'}
                  className={cn(
                    'inline-block h-2 w-2 rounded-full',
                    ev.severity === 'info' ? 'bg-sev-info' : 'bg-sev-medium',
                  )}
                />
              </td>
              <td className="px-3 py-2 font-mono text-text-muted">{relativeAge(ev.ts)}</td>
              <td className="px-3 py-2 text-text-primary">{humanKind(ev.evidence?.kind ?? '')}</td>
              <td className="px-3 py-2 font-mono text-text-muted" title={ev.host_id}>
                {ev.host_id.split('-')[0]}
              </td>
              <td className="px-3 py-2 text-text-muted">{ag ? humanTool(ag.tool) : '—'}</td>
            </tr>
          );
        })}
      </tbody>
    </table>
  );
}
