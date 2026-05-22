import { createFileRoute } from '@tanstack/react-router';

export const Route = createFileRoute('/_authed/fleet/risk')({
  component: () => <div className="text-sm text-text-muted">Risk tab — coming in T8.</div>,
});
