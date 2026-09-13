import { useQuery } from '@tanstack/react-query';
import { createFileRoute } from '@tanstack/react-router';
import { fleetHealthz } from '@/api/fleet';
import { useFleetMeta } from '@/hooks/useFleetMeta';
import { auditSigningSummary } from '@/lib/audit';
import { licenseHostSummary } from '@/lib/license';
import { readAPIError } from '@/lib/server-status';

export const Route = createFileRoute('/_authed/settings')({ component: SettingsPage });

function SettingsPage() {
  const meta = useFleetMeta({ refetchInterval: 10_000, refetchOnMount: 'always' });
  const health = useQuery({
    queryKey: ['fleet', 'healthz'],
    queryFn: fleetHealthz,
    refetchInterval: 10_000,
    refetchIntervalInBackground: false,
    retry: false,
  });
  const license = meta.data?.license;
  const busy = meta.isFetching || health.isFetching;
  return (
    <div className="flex max-w-[720px] flex-col py-4">
      <h1 className="mb-4 text-lg font-semibold text-text-primary">Settings</h1>
      <Section title="sigil-server">
        <Row label="Liveness">
          {health.isError
            ? 'Unreachable'
            : health.isPending
              ? 'Checking…'
              : health.data?.status === 'ok'
                ? 'Reachable'
                : 'Unexpected health status'}
        </Row>
        <Row label="Read API">
          {meta.isError ? (
            <span role="alert" className="text-sev-critical">
              {readAPIError(meta.error)}
            </span>
          ) : meta.isPending ? (
            'Checking authenticated access…'
          ) : (
            'Connected (authenticated)'
          )}
        </Row>
      </Section>
      <button
        type="button"
        className="mb-4 self-start rounded border border-border px-3 py-1 text-xs text-accent disabled:opacity-50"
        disabled={busy}
        onClick={() => {
          void meta.refetch();
          void health.refetch();
        }}
      >
        {busy ? 'Checking connection…' : 'Retry connection'}
      </button>
      {!meta.data ? (
        <p role="status" className="text-sm text-text-muted">
          {meta.isPending
            ? 'Loading server metadata…'
            : 'Metadata unavailable. License and audit status are unknown.'}
        </p>
      ) : (
        <>
          {meta.isError && (
            <p role="status" className="mb-3 text-sm text-status-degraded">
              Cached metadata — last successful fetch {new Date(meta.dataUpdatedAt).toISOString()}.
              These values may be stale.
            </p>
          )}
          <Section title={meta.isError ? 'Server metadata (cached)' : 'Server metadata'}>
            <Row label="Server version">
              <code>{meta.data.server_version}</code>
            </Row>
            <Row label="Schema version">v{meta.data.schema_version}</Row>
          </Section>
          <Section title={meta.isError ? 'License (cached)' : 'License'}>
            {license ? (
              <>
                <Row label="Hosts">{licenseHostSummary(license)}</Row>
                <Row label="Licensed">{license.licensed ? 'yes' : 'no (reported by server)'}</Row>
                <Row label="State">{license.expired ? 'expired' : license.state}</Row>
                {license.not_after && (
                  <Row label="Valid until">
                    <time dateTime={license.not_after}>{license.not_after}</time>
                  </Row>
                )}
              </>
            ) : (
              <Row label="License">Not reported by this server</Row>
            )}
          </Section>
          <Section title={meta.isError ? 'Audit (cached)' : 'Audit'}>
            <Row label="Log signing">{auditSigningSummary(meta.data.audit_head)}</Row>
          </Section>
        </>
      )}
      <Section title="Auth">
        <Row label="Users">single admin user, configured via environment (v1)</Row>
      </Section>
    </div>
  );
}

function Section({ title, children }: { title: string; children: React.ReactNode }) {
  return (
    <section className="mb-5">
      <h2 className="mb-2 text-xs uppercase tracking-wide text-text-subtle">{title}</h2>
      <dl className="grid grid-cols-[140px_minmax(0,1fr)] gap-y-1.5 rounded border border-border-subtle bg-bg-surface px-4 py-3 text-sm">
        {children}
      </dl>
    </section>
  );
}
function Row({ label, children }: { label: string; children: React.ReactNode }) {
  return (
    <>
      <dt className="text-text-subtle">{label}</dt>
      <dd className="min-w-0 break-words text-text-body">{children}</dd>
    </>
  );
}
