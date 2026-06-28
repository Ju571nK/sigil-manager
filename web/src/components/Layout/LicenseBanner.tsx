import { AlertTriangle } from 'lucide-react';
import { useFleetMeta } from '@/hooks/useFleetMeta';
import { licenseAlert } from '@/lib/license';
import { cn } from '@/lib/utils';

/**
 * Banner under the TopNav that surfaces a license problem from `/v1/meta`
 * (contract §14.9.3 / issue #7): expired (red) or over the host limit
 * (yellow). Renders nothing in the happy path or when the server omits the
 * license block (open-source / older server). Read-only — enforcement lives
 * in the producer; the console only reports.
 */
export function LicenseBanner() {
  const q = useFleetMeta();
  const alert = licenseAlert(q.data?.license);
  if (!alert) return null;

  const tone =
    alert.severity === 'error'
      ? 'border-sev-critical/40 bg-sev-critical/10 text-sev-critical'
      : 'border-status-degraded/40 bg-status-degraded/10 text-status-degraded';

  return (
    <div className={cn('border-b px-6 py-2 text-xs', tone)} role="status">
      <div className="mx-auto flex max-w-[1280px] items-center gap-2">
        <AlertTriangle className="h-3.5 w-3.5 shrink-0" />
        <span className="flex-1">{alert.message}</span>
      </div>
    </div>
  );
}
