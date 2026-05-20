import { createFileRoute, Outlet, redirect } from '@tanstack/react-router';
import { me } from '@/api/auth';
import { SessionExpiredError, UnauthorizedError } from '@/api/client';
import { ConnectionBanner } from '@/components/Layout/ConnectionBanner';
import { PageShell } from '@/components/Layout/PageShell';
import { TopNav } from '@/components/Layout/TopNav';

/**
 * `_authed` is a pathless layout group: every route nested under
 * `src/routes/_authed/*` inherits this layout AND the `beforeLoad` guard.
 *
 * The guard calls `/api/v1/auth/me`; a 401 (either `unauthorized` or
 * `session_expired`) throws a `redirect` to `/login` carrying the original
 * URL in `?redirect=` so post-login we can land back where we started.
 */
export const Route = createFileRoute('/_authed')({
  beforeLoad: async ({ location }) => {
    try {
      await me();
    } catch (err) {
      if (err instanceof UnauthorizedError || err instanceof SessionExpiredError) {
        throw redirect({
          to: '/login',
          search: { redirect: location.href },
        });
      }
      throw err;
    }
  },
  component: AuthedLayout,
});

function AuthedLayout() {
  return (
    <div className="min-h-screen bg-bg-page text-text-body">
      <TopNav />
      <ConnectionBanner />
      <main>
        <PageShell>
          <Outlet />
        </PageShell>
      </main>
    </div>
  );
}
