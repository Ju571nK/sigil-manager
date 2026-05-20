import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { Link, useRouter } from '@tanstack/react-router';
import { LogOut } from 'lucide-react';
import { logout } from '@/api/auth';
import { ServiceUnavailableError, UnauthorizedError } from '@/api/client';
import { fleetHealthz } from '@/api/fleet';
import { Button } from '@/components/ui/button';
import { cn } from '@/lib/utils';

/**
 * Top navigation strip. Per UI/UX §4:
 *   - Brand mark on the left (◆ sigil).
 *   - Primary nav items: Alerts (active in Plan 02), Fleet + Settings
 *     stubbed with "Coming in Plan 03/04" badges.
 *   - Connection-state pill on the right showing sigil-server liveness;
 *     polled every 10 s via /api/v1/fleet/healthz so the operator knows
 *     immediately when the upstream goes away.
 *   - Logout button at the far right.
 */
export function TopNav() {
  const router = useRouter();
  const queryClient = useQueryClient();

  const healthz = useQuery({
    queryKey: ['fleet', 'healthz'],
    queryFn: fleetHealthz,
    refetchInterval: 10_000,
    refetchIntervalInBackground: false,
    retry: false,
  });

  const logoutMutation = useMutation({
    mutationFn: logout,
    onSettled: async () => {
      queryClient.clear();
      await router.navigate({ to: '/login' });
    },
  });

  const state = connectionState(healthz);

  return (
    <header className="sticky top-0 z-30 border-b border-border-subtle bg-bg-surface/80 backdrop-blur">
      <div className="mx-auto flex h-12 max-w-[1280px] items-center gap-6 px-6">
        <Link to="/alerts" className="flex items-center gap-2 text-text-primary font-semibold">
          <span className="text-accent">◆</span>
          <span>sigil</span>
        </Link>

        <nav className="flex items-center gap-1 text-sm">
          <Link
            to="/alerts"
            activeProps={{ className: 'text-text-primary bg-bg-elevated' }}
            inactiveProps={{ className: 'text-text-muted hover:text-text-primary' }}
            className="rounded px-2.5 py-1 transition-colors"
          >
            Alerts
          </Link>
          <NavStub label="Fleet" hint="Plan 03" />
          <NavStub label="Settings" hint="Plan 05" />
        </nav>

        <div className="ml-auto flex items-center gap-3">
          <ConnectionPill state={state} />
          <Button
            variant="ghost"
            size="sm"
            onClick={() => logoutMutation.mutate()}
            disabled={logoutMutation.isPending}
            aria-label="Log out"
          >
            <LogOut className="h-3.5 w-3.5" />
          </Button>
        </div>
      </div>
    </header>
  );
}

type ConnectionState = 'connected' | 'stale' | 'disconnected' | 'unknown';

function connectionState(q: ReturnType<typeof useQuery>): ConnectionState {
  if (q.isPending && !q.data) return 'unknown';
  if (q.isError) {
    if (q.error instanceof ServiceUnavailableError) return 'stale';
    if (q.error instanceof UnauthorizedError) return 'unknown';
    return 'disconnected';
  }
  // dataUpdatedAt > 30s old without a refresh = treat as stale
  const age = Date.now() - (q.dataUpdatedAt ?? 0);
  if (age > 30_000) return 'stale';
  return 'connected';
}

function ConnectionPill({ state }: { state: ConnectionState }) {
  const labels: Record<ConnectionState, string> = {
    connected: 'Connected',
    stale: 'Stale',
    disconnected: 'Disconnected',
    unknown: '—',
  };
  const dotColor: Record<ConnectionState, string> = {
    connected: 'bg-status-healthy',
    stale: 'bg-status-degraded',
    disconnected: 'bg-status-down',
    unknown: 'bg-text-subtle',
  };
  return (
    <div
      className="flex items-center gap-1.5 text-xs text-text-muted"
      title={`sigil-server: ${labels[state]}`}
    >
      <span className={cn('inline-block h-1.5 w-1.5 rounded-full', dotColor[state])} />
      <span>{labels[state]}</span>
    </div>
  );
}

function NavStub({ label, hint }: { label: string; hint: string }) {
  return (
    <span
      className="rounded px-2.5 py-1 text-text-subtle cursor-not-allowed"
      title={`Coming in ${hint}`}
    >
      {label}
    </span>
  );
}
