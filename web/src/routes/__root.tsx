import type { QueryClient } from '@tanstack/react-query';
import { createRootRouteWithContext, Outlet } from '@tanstack/react-router';
import { Toaster } from '@/components/ui/sonner';

/** Router context shared with every route's beforeLoad (see main.tsx). */
export interface RouterContext {
  queryClient: QueryClient;
}

/**
 * Root route: just an Outlet so each child layout (the public `/login`
 * route and the `_authed` layout group) can own its own chrome. A global
 * toaster lives here so all routes can fire toasts via `sonner`.
 *
 * Dark-only per UI/UX §9 — the body background is set in index.css; we
 * don't need an inner wrapper to repeat it.
 */
export const Route = createRootRouteWithContext<RouterContext>()({
  component: () => (
    <>
      <Outlet />
      <Toaster position="bottom-right" />
    </>
  ),
});
