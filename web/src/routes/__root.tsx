import { createRootRoute, Outlet } from '@tanstack/react-router';
import { Toaster } from '@/components/ui/sonner';

/**
 * Root route: just an Outlet so each child layout (the public `/login`
 * route and the `_authed` layout group) can own its own chrome. A global
 * toaster lives here so all routes can fire toasts via `sonner`.
 *
 * Dark-only per UI/UX §9 — the body background is set in index.css; we
 * don't need an inner wrapper to repeat it.
 */
export const Route = createRootRoute({
  component: () => (
    <>
      <Outlet />
      <Toaster position="bottom-right" />
    </>
  ),
});
