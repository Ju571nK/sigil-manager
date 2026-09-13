import { createFileRoute } from '@tanstack/react-router';
import { ComplianceTable } from '@/components/Fleet/ComplianceTable';
import { Pagination } from '@/components/Fleet/Pagination';
import { useFleetCompliance } from '@/hooks/useFleetCompliance';

export const Route = createFileRoute('/_authed/fleet/compliance')({
  component: ComplianceTab,
});

function ComplianceTab() {
  const compliance = useFleetCompliance();
  const { rows, isPending } = compliance;
  return (
    <div className="overflow-hidden rounded-md border border-border bg-bg-surface">
      <ComplianceTable rows={rows} isPending={isPending} />
      <Pagination {...compliance} count={rows.length} />
    </div>
  );
}
