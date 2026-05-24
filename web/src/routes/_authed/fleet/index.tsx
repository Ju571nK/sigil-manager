import { createFileRoute, redirect } from '@tanstack/react-router';

export const Route = createFileRoute('/_authed/fleet/')({
  beforeLoad: () => {
    throw redirect({ to: '/fleet/risk' });
  },
});
