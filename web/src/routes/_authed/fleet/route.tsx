import { createFileRoute, Outlet } from '@tanstack/react-router';
import { FleetTabs } from '@/components/Fleet/FleetTabs';
import { useFleetMeta } from '@/hooks/useFleetMeta';
import { activeHostSummary } from '@/lib/fleet-size';

export const Route = createFileRoute('/_authed/fleet')({
  component: FleetLayout,
});

function FleetLayout() {
  const meta = useFleetMeta();
  const hosts = activeHostSummary(meta.data?.fleet);
  return (
    <div className="flex flex-col py-4">
      <div className="mb-3 flex items-baseline gap-3">
        <h1 className="text-lg font-semibold text-text-primary">Fleet</h1>
        {hosts && <span className="text-xs text-text-muted">{hosts}</span>}
      </div>
      <FleetTabs />
      <div className="pt-4">
        <Outlet />
      </div>
    </div>
  );
}
