import { Link } from '@tanstack/react-router';
import { CompliancePill } from '@/components/CompliancePill';
import type { ComplianceStatus } from '@/lib/compliance';
import { relativeAge } from '@/lib/time';
import { cn } from '@/lib/utils';

interface Props {
  hostname: string | null;
  hostId: string;
  status: string;
  lastSeenTs: string;
  agentVersion: string;
  /** Derived from the compliance row, when this host appears in /fleet/compliance. */
  compliance?: ComplianceStatus;
}

/** Host detail header (UI/UX §5.3): identity + status + compliance + back link. */
export function HostHeader({
  hostname,
  hostId,
  status,
  lastSeenTs,
  agentVersion,
  compliance,
}: Props) {
  const tone = statusTone(status);
  return (
    <div className="mb-4">
      <Link to="/fleet/risk" className="text-xs text-text-muted hover:text-text-primary">
        ◂ Fleet
      </Link>
      <div className="mt-1 flex flex-wrap items-center gap-3">
        <h1 className="text-lg font-semibold text-text-primary">
          {hostname ?? hostId.split('-')[0]}
        </h1>
        <Dot tone={tone} label={status} />
        {compliance && <CompliancePill status={compliance} />}
        <span className="text-xs text-text-muted">
          last seen <span title={lastSeenTs}>{relativeAge(lastSeenTs)} ago</span>
        </span>
        <span className="text-xs text-text-muted">agent v{agentVersion}</span>
        <span className="ml-auto font-mono text-[10px] text-text-subtle" title={hostId}>
          {hostId}
        </span>
      </div>
    </div>
  );
}

function statusTone(status: string): 'healthy' | 'degraded' | 'down' | 'subtle' {
  if (status === 'healthy') return 'healthy';
  if (status === 'stale') return 'degraded';
  if (status === 'disconnected') return 'down';
  return 'subtle';
}

function Dot({ tone, label }: { tone: 'healthy' | 'degraded' | 'down' | 'subtle'; label: string }) {
  const bg = {
    healthy: 'bg-status-healthy',
    degraded: 'bg-status-degraded',
    down: 'bg-status-down',
    subtle: 'bg-text-subtle',
  }[tone];
  return (
    <span className="flex items-center gap-1.5 text-xs text-text-muted">
      <span className={cn('inline-block h-1.5 w-1.5 rounded-full', bg)} />
      {label}
    </span>
  );
}
