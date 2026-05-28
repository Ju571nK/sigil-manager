import { COMPLIANCE_META, type ComplianceStatus } from '@/lib/compliance';
import { cn } from '@/lib/utils';

const TONE_CLASS: Record<'healthy' | 'degraded' | 'down', string> = {
  healthy: 'text-status-healthy border-status-healthy/40 bg-status-healthy/10',
  degraded: 'text-status-degraded border-status-degraded/40 bg-status-degraded/10',
  down: 'text-status-down border-status-down/40 bg-status-down/10',
};

/**
 * Derived compliance status as a tone-colored pill (label + tone from
 * COMPLIANCE_META). Shared by the Compliance tab and the host-detail header so
 * the COMPLIANCE_META-driven color map lives in exactly one place.
 */
export function CompliancePill({ status }: { status: ComplianceStatus }) {
  const meta = COMPLIANCE_META[status];
  return (
    <span
      className={cn(
        'inline-block rounded border px-1.5 py-px text-[10px] uppercase tracking-wide',
        TONE_CLASS[meta.tone],
      )}
    >
      {meta.label}
    </span>
  );
}
