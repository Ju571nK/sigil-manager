import { createFileRoute, Link } from '@tanstack/react-router';
import { useEffect } from 'react';
import { NotFoundError } from '@/api/client';
import { EventDetails } from '@/components/EventDetails';
import { EventsTable } from '@/components/Fleet/EventsTable';
import { Pagination } from '@/components/Fleet/Pagination';
import { AiGuardByTool } from '@/components/Host/AiGuardByTool';
import { HostHeader } from '@/components/Host/HostHeader';
import { HostMetaCard } from '@/components/Host/HostMetaCard';
import { PolicyHealthCard } from '@/components/Host/PolicyHealthCard';
import { useFleetCompliance } from '@/hooks/useFleetCompliance';
import { useFleetEvents } from '@/hooks/useFleetEvents';
import { useFleetHost } from '@/hooks/useFleetHost';
import { deriveComplianceStatus } from '@/lib/compliance';

export const Route = createFileRoute('/_authed/hosts/$hostId')({
  validateSearch: (raw: Record<string, unknown>): { event?: string } => ({
    event: typeof raw.event === 'string' ? raw.event : undefined,
  }),
  component: HostDetailPage,
});

function HostDetailPage() {
  const { hostId } = Route.useParams();
  const search = Route.useSearch();
  const navigate = Route.useNavigate();
  const { host, isPending, error } = useFleetHost(hostId);

  // Continue the shared paginated feed until this host is found.
  const compliance = useFleetCompliance();
  const row = compliance.rows.find((r) => r.host_id === hostId);
  const status = row ? deriveComplianceStatus(row) : undefined;
  useEffect(() => {
    if (!row && compliance.hasMore && !compliance.isFetching && !compliance.error)
      void compliance.loadMore();
  }, [row, compliance.hasMore, compliance.isFetching, compliance.error, compliance.loadMore]);

  const events = useFleetEvents({ evidenceKinds: [], hostIDs: [hostId], since: null });

  if (error instanceof NotFoundError) {
    return <NotFoundPanel hostId={hostId} />;
  }
  if (error) {
    return (
      <div className="px-4 py-6 text-sm text-sev-critical">
        Failed to load host: {error.message}
      </div>
    );
  }
  if (isPending || !host) {
    return (
      <div className="space-y-3 px-1 py-2">
        <div className="h-5 w-48 animate-pulse rounded bg-bg-elevated" />
        <div className="h-24 w-full animate-pulse rounded bg-bg-elevated" />
        <div className="h-24 w-full animate-pulse rounded bg-bg-elevated" />
      </div>
    );
  }

  return (
    <div>
      <HostHeader
        hostname={host.hostname}
        hostId={host.host_id}
        status={host.status}
        lastSeenTs={host.last_seen_ts}
        agentVersion={host.agent_version}
        compliance={status}
      />
      {!row && (
        <p className="mb-3 text-xs text-text-muted">
          {compliance.error || compliance.cursorRepeated
            ? 'Policy status could not be loaded.'
            : compliance.isPending || compliance.hasMore || compliance.isFetching
              ? 'Looking up policy status…'
              : 'No policy status reported for this host.'}{' '}
          {(compliance.error || compliance.cursorRepeated) && (
            <button type="button" className="text-accent" onClick={() => compliance.refetch()}>
              Retry policy status
            </button>
          )}
        </p>
      )}
      <AiGuardByTool byTool={host.ai_guard?.by_tool ?? {}} />
      <div className="mb-4 grid gap-3 md:grid-cols-2">
        <HostMetaCard meta={host.host_meta} />
        <PolicyHealthCard
          policy={host.policy_state}
          health={host.agent_health}
          signatureFailures24h={row?.signature_failures_24h}
        />
      </div>
      <section>
        <div className="mb-2 flex items-center justify-between">
          <h2 className="text-xs font-medium uppercase tracking-wide text-text-subtle">
            Recent events
          </h2>
          <Link
            to="/fleet/events"
            search={{ host: [hostId] }}
            className="text-xs text-text-muted hover:text-text-primary"
          >
            see all in Fleet Events ▸
          </Link>
        </div>
        <div className="overflow-hidden rounded-md border border-border bg-bg-surface">
          <EventsTable
            rows={events.rows}
            isPending={events.isPending}
            onSelect={(event) => navigate({ search: { event: event.event_id }, replace: true })}
          />
          <Pagination {...events} count={events.rawCount} />
        </div>
      </section>
      <EventDetails
        eventID={search.event ?? null}
        hostID={hostId}
        rows={events.rows}
        onClose={() => navigate({ search: { event: undefined }, replace: true })}
      />
    </div>
  );
}

function NotFoundPanel({ hostId }: { hostId: string }) {
  return (
    <div className="rounded-md border border-border bg-bg-surface px-6 py-10 text-center">
      <p className="text-sm text-text-primary">Host not found</p>
      <p className="mt-1 font-mono text-xs text-text-muted">{hostId}</p>
      <p className="mt-3 text-xs text-text-muted">
        It may have been evicted from the server's index. ◂{' '}
        <Link className="text-accent hover:underline" to="/fleet/risk">
          Back to Fleet
        </Link>
      </p>
    </div>
  );
}
