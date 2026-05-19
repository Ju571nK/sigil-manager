import { createFileRoute } from '@tanstack/react-router';

/**
 * Placeholder for the Alerts queue. T11 fills this in with the actual
 * filter row + 28px-density table + 5s polling per UI/UX D2.
 *
 * Keeping it here in T10 so the `_authed` guard has a route to land
 * `/` redirects on, and so we can manually verify the login flow
 * end-to-end before T11.
 */
export const Route = createFileRoute('/_authed/alerts')({
  component: AlertsPlaceholder,
});

function AlertsPlaceholder() {
  return (
    <div className="py-8">
      <h1 className="text-xl font-semibold text-text-primary">Alerts</h1>
      <p className="mt-2 text-sm text-text-muted">
        Queue UI ships in Plan 02 T11. The route guard + chrome are live now.
      </p>
    </div>
  );
}
