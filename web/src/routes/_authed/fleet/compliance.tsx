import { createFileRoute } from '@tanstack/react-router';

export const Route = createFileRoute('/_authed/fleet/compliance')({
  component: () => <div className="text-sm text-text-muted">Compliance tab — coming in T10.</div>,
});
