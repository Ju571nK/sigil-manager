import { createFileRoute, Outlet } from '@tanstack/react-router';
import { FleetTabs } from '@/components/Fleet/FleetTabs';

export const Route = createFileRoute('/_authed/fleet')({
  component: FleetLayout,
});

function FleetLayout() {
  return (
    <div className="flex flex-col py-4">
      <h1 className="mb-3 text-lg font-semibold text-text-primary">Fleet</h1>
      <FleetTabs />
      <div className="pt-4">
        <Outlet />
      </div>
    </div>
  );
}
