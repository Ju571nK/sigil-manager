import { createFileRoute } from '@tanstack/react-router';

export const Route = createFileRoute('/_authed/fleet/events')({
  component: () => <div className="text-sm text-text-muted">Events tab — coming in T9.</div>,
});
