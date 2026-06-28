import { type EventWithTriage, extractAiGuard, extractHook, extractToggleDrift } from '@/api/fleet';
import { hookTitle, humanKind, humanTool } from '@/lib/labels';
import { bucketTextColor } from '@/lib/severity';
import { relativeAge } from '@/lib/time';
import { cn } from '@/lib/utils';

interface Props {
  event: EventWithTriage;
  selected: boolean;
  onSelect: (event: EventWithTriage) => void;
  onHoverEnter: () => void;
  onHoverLeave: () => void;
  /** True for ~1s after this row first appears so we can flash a highlight. */
  isFresh?: boolean;
}

/**
 * One row of the Alerts queue at 28px height per UI/UX §6.4.
 * Layout (left → right):
 *   severity dot · age · title · host · severity label · triage status pill · assignee.
 */
export function QueueRow({
  event,
  selected,
  onSelect,
  onHoverEnter,
  onHoverLeave,
  isFresh,
}: Props) {
  const ag = extractAiGuard(event);
  const bucket = ag?.bucket ?? severityToBucket(event.severity);

  const title = computeTitle(event, ag);
  const host = displayHost(event.host_id);
  const age = relativeAge(event.ts);
  const triage = event.triage;

  return (
    <button
      type="button"
      onClick={() => onSelect(event)}
      onMouseEnter={onHoverEnter}
      onMouseLeave={onHoverLeave}
      data-fresh={isFresh ? 'true' : undefined}
      className={cn(
        'group grid h-[28px] w-full grid-cols-[16px_64px_minmax(0,1fr)_140px_72px_92px_120px] items-center gap-3 px-3 text-left text-xs transition-colors',
        'border-b border-border-subtle',
        selected ? 'bg-bg-elevated text-text-primary' : 'text-text-body hover:bg-bg-elevated/60',
        // Critical glow: subtle box-shadow per UI/UX §6.2. The data-fresh
        // attribute drives a 1s fade-in animation defined in index.css.
        bucket === 'critical' && 'shadow-[inset_2px_0_0] shadow-sev-critical',
        isFresh && 'animate-[alertFlash_1s_ease-out]',
      )}
      aria-pressed={selected}
    >
      <SeverityDot bucket={bucket} />
      <span className="font-mono text-text-muted">{age}</span>
      <span className="truncate text-text-primary">{title}</span>
      <span className="truncate font-mono text-text-muted" title={event.host_id}>
        {host}
      </span>
      <span className={cn('uppercase tracking-wide font-medium', bucketTextColor(bucket))}>
        {bucket}
      </span>
      <StatusPill status={triage?.status ?? 'open'} hasTriage={!!triage} />
      <span className="truncate text-text-muted">{triage?.assignee ?? '—'}</span>
    </button>
  );
}

function SeverityDot({ bucket }: { bucket: string }) {
  const color = bucketTextColor(bucket).replace('text-', 'bg-');
  return (
    <span
      className={cn('inline-block h-2 w-2 rounded-full', color, {
        'shadow-[0_0_6px] shadow-sev-critical': bucket === 'critical',
      })}
    />
  );
}

function StatusPill({
  status,
  hasTriage,
}: {
  status: 'open' | 'acknowledged' | 'investigating' | 'resolved';
  hasTriage: boolean;
}) {
  const label = status === 'acknowledged' ? 'ack' : status;
  const className = hasTriage ? statusColor(status) : 'text-text-subtle border-border-subtle';
  return (
    <span
      className={cn(
        'inline-block rounded border px-1.5 py-px text-[10px] uppercase tracking-wide w-fit',
        className,
      )}
    >
      {label}
    </span>
  );
}

function statusColor(status: 'open' | 'acknowledged' | 'investigating' | 'resolved'): string {
  switch (status) {
    case 'open':
      return 'text-sev-medium border-sev-medium/40 bg-sev-medium/10';
    case 'acknowledged':
      return 'text-status-degraded border-status-degraded/40 bg-status-degraded/10';
    case 'investigating':
      return 'text-accent border-accent/40 bg-accent/10';
    case 'resolved':
      return 'text-status-healthy border-status-healthy/40 bg-status-healthy/10';
  }
}

function severityToBucket(severity: string): 'low' | 'medium' | 'high' | 'critical' | 'info' {
  if (severity === 'warn') return 'medium';
  if (severity === 'info') return 'info';
  return 'info';
}

function displayHost(hostID: string): string {
  // Show first segment of the UUID; full id is in the title= tooltip.
  return hostID.split('-')[0] ?? hostID;
}

const KIND_TITLES: Record<string, string> = {
  ai_guard_risk_assessed: 'AI Guard risk',
  host_meta_snapshot: 'Host metadata',
  heartbeat: 'Heartbeat',
  policy_reloaded: 'Policy reloaded',
  policy_signature_invalid: 'Policy signature invalid',
  tls_failure: 'TLS failure',
  host_id_fingerprint_drift: 'Host ID fingerprint drift',
  agent_dying: 'Agent dying',
  sender_lag_critical: 'Sender lag critical',
  file_change: 'File change',
};

function computeTitle(ev: EventWithTriage, ag: ReturnType<typeof extractAiGuard>): string {
  if (ag) {
    const tool = humanTool(ag.tool, ag.tool_label);
    const reason = ag.reasons?.[0]?.kind;
    return reason ? `AI Guard risk: ${humanKind(reason)} · ${tool}` : `AI Guard risk · ${tool}`;
  }
  const hook = extractHook(ev);
  if (hook) return hookTitle(hook);
  const drift = extractToggleDrift(ev);
  if (drift) return `AI Guard toggle drift · ${humanTool(drift.tool, drift.tool_label)}`;
  return KIND_TITLES[ev.evidence?.kind] ?? humanKind(ev.evidence?.kind ?? 'unknown');
}
