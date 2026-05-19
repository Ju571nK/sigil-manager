import { createFileRoute, redirect } from '@tanstack/react-router';
import { me } from '@/api/auth';
import { SessionExpiredError, UnauthorizedError } from '@/api/client';
import { DEFAULT_FILTER } from '@/hooks/useAlerts';

/**
 * `/` is a redirect: authed → `/alerts`, otherwise → `/login`. We probe
 * `/api/v1/auth/me` to make the decision so the bounce reflects actual
 * server state, not a stale client-side flag. The `/alerts` route has a
 * required search shape, so we seed defaults here.
 */
export const Route = createFileRoute('/')({
  beforeLoad: async () => {
    try {
      await me();
      throw redirect({
        to: '/alerts',
        search: { ...DEFAULT_FILTER, sort: 'severity_desc' as const },
      });
    } catch (err) {
      if (err instanceof UnauthorizedError || err instanceof SessionExpiredError) {
        throw redirect({ to: '/login' });
      }
      throw err;
    }
  },
});
