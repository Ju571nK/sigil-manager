import { createFileRoute, Link } from '@tanstack/react-router';
import { NotFoundError } from '@/api/client';
import { EventsTable } from '@/components/Fleet/EventsTable';
import { AiGuardByTool } from '@/components/Host/AiGuardByTool';
import { HostHeader } from '@/components/Host/HostHeader';
import { HostMetaCard } from '@/components/Host/HostMetaCard';
import { PolicyHealthCard } from '@/components/Host/PolicyHealthCard';
import { useFleetCompliance } from '@/hooks/useFleetCompliance';
import { useFleetEvents } from '@/hooks/useFleetEvents';
import { useFleetHost } from '@/hooks/useFleetHost';
import { deriveComplianceStatus } from '@/lib/compliance';

export const Route = createFileRoute('/_authed/hosts/$hostId')({
  component: HostDetailPage,
});

function HostDetailPage() {
  const { hostId } = Route.useParams();
  const { host, isPending, error } = useFleetHost(hostId);

  // Compliance status is derived from the fleet-wide compliance feed, not from
  // host.policy_state: signature_failures_24h and version_drift live only on
  // /fleet/compliance (F13), so HostDetail alone can't produce the status pill.
  // We find this host's row (absent until the feed loads → pill appears then).
  // Small-fleet assumption: the feed is fetched at limit 100; a host beyond the
  // first page won't get a pill until a server-side ?host_id= filter exists
  // (spec §10). Fine for current fleet sizes.
  const compliance = useFleetCompliance();
  const row = compliance.rows.find((r) => r.host_id === hostId);
  const status = row ? deriveComplianceStatus(row) : undefined;

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
          {events.error ? (
            <div className="px-4 py-6 text-sm text-sev-critical">
              Failed to load events: {events.error.message}
            </div>
          ) : (
            <EventsTable rows={events.rows} isPending={events.isPending} />
          )}
        </div>
      </section>
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
