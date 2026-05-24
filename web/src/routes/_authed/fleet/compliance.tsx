import { createFileRoute } from '@tanstack/react-router';
import { ComplianceTable } from '@/components/Fleet/ComplianceTable';
import { useFleetCompliance } from '@/hooks/useFleetCompliance';

export const Route = createFileRoute('/_authed/fleet/compliance')({
  component: ComplianceTab,
});

function ComplianceTab() {
  const { rows, isPending, error } = useFleetCompliance();
  return (
    <div className="overflow-hidden rounded-md border border-border bg-bg-surface">
      {error ? (
        <div className="px-4 py-6 text-sm text-sev-critical">
          Failed to load compliance: {error.message}
        </div>
      ) : (
        <ComplianceTable rows={rows} isPending={isPending} />
      )}
    </div>
  );
}
