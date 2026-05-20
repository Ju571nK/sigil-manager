import { useQuery } from '@tanstack/react-query';
import { AlertTriangle, RefreshCw } from 'lucide-react';
import { ServiceUnavailableError, UnauthorizedError } from '@/api/client';
import { fleetHealthz } from '@/api/fleet';
import { cn } from '@/lib/utils';

/**
 * Sticky banner under the TopNav that surfaces when sigil-server is
 * unreachable (UI/UX §8.3). The TopNav already shows a small connection
 * pill; this banner is the louder escalation:
 *   - Disconnected (network / 5xx / unknown): red banner with manual retry.
 *   - 503 service_unavailable (boot rebuild): yellow banner, retry hint.
 *   - 401 unauthorized: handled elsewhere (the _authed guard kicks to /login).
 *
 * Uses the same `['fleet','healthz']` query that the TopNav polls, so we
 * don't fire a second request just for the banner.
 */
export function ConnectionBanner() {
  const q = useQuery({
    queryKey: ['fleet', 'healthz'],
    queryFn: fleetHealthz,
    refetchInterval: 10_000,
    refetchIntervalInBackground: false,
    retry: false,
  });

  if (!q.isError) return null;
  if (q.error instanceof UnauthorizedError) return null;

  const isServiceUnavailable = q.error instanceof ServiceUnavailableError;
  const tone = isServiceUnavailable
    ? 'border-status-degraded/40 bg-status-degraded/10 text-status-degraded'
    : 'border-sev-critical/40 bg-sev-critical/10 text-sev-critical';
  const message = isServiceUnavailable
    ? 'sigil-server is rebuilding its index. Retrying every 10s…'
    : 'Lost connection to sigil-server. Retrying every 10s…';

  return (
    <div className={cn('border-b px-6 py-2 text-xs', tone)}>
      <div className="mx-auto flex max-w-[1280px] items-center gap-2">
        <AlertTriangle className="h-3.5 w-3.5 shrink-0" />
        <span className="flex-1">{message}</span>
        <button
          type="button"
          onClick={() => q.refetch()}
          disabled={q.isFetching}
          className="inline-flex items-center gap-1 rounded border border-current/40 px-2 py-0.5 text-[11px] hover:bg-bg-elevated"
        >
          <RefreshCw className={cn('h-3 w-3', q.isFetching && 'animate-spin')} />
          {q.isFetching ? 'Retrying' : 'Retry now'}
        </button>
      </div>
    </div>
  );
}
