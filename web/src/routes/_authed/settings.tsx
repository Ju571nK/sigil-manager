import { useQuery } from '@tanstack/react-query';
import { createFileRoute } from '@tanstack/react-router';
import { ServiceUnavailableError } from '@/api/client';
import { fleetHealthz } from '@/api/fleet';
import { useFleetMeta } from '@/hooks/useFleetMeta';
import { auditSigningSummary } from '@/lib/audit';
import { licenseHostSummary } from '@/lib/license';
import { cn } from '@/lib/utils';

export const Route = createFileRoute('/_authed/settings')({
  component: SettingsPage,
});

/**
 * Settings, per UI/UX §5.4 — deliberately minimal in v1:
 *   - sigil-server connection status + version (URL intentionally NOT shown:
 *     the API layer strips upstream URLs from every error for the same
 *     reason — the browser should never learn the upstream address).
 *   - License / host count from /v1/meta.license (§14.9.3).
 *   - Audit signing presence from /v1/meta.audit_head (§14.9.3) — reported,
 *     never verified here.
 *   - Auth: single admin user via env; no user management in v1.
 * Reuses the ['fleet','healthz'] and ['fleet','meta'] queries the TopNav and
 * alerts queue already poll — no extra request load.
 */
function SettingsPage() {
  const meta = useFleetMeta();
  const healthz = useQuery({
    queryKey: ['fleet', 'healthz'],
    queryFn: fleetHealthz,
    refetchInterval: 10_000,
    refetchIntervalInBackground: false,
    retry: false,
  });

  const license = meta.data?.license;
  const hosts = licenseHostSummary(license);

  return (
    <div className="flex max-w-[720px] flex-col py-4">
      <h1 className="mb-4 text-lg font-semibold text-text-primary">Settings</h1>

      <Section title="sigil-server">
        <Row label="Connection">
          <StatusDot state={serverState(healthz)} />
          <span className="ml-1.5">{serverStateLabel(serverState(healthz))}</span>
        </Row>
        <Row label="Server version">
          {meta.data ? <code className="font-mono">{meta.data.server_version}</code> : '—'}
        </Row>
        <Row label="Schema version">{meta.data ? `v${meta.data.schema_version}` : '—'}</Row>
      </Section>

      <Section title="License">
        {license ? (
          <>
            <Row label="Hosts">{hosts}</Row>
            <Row label="State">
              <span
                className={cn(
                  license.expired || license.state === 'over_limit'
                    ? 'text-sev-critical'
                    : 'text-status-healthy',
                )}
              >
                {license.expired ? 'expired' : license.state}
              </span>
            </Row>
            {license.not_after && (
              <Row label="Valid until">
                <span title={license.not_after}>{license.not_after.slice(0, 10)}</span>
              </Row>
            )}
          </>
        ) : (
          <Row label="License">none (open-source server)</Row>
        )}
      </Section>

      <Section title="Audit">
        <Row label="Log signing">{auditSigningSummary(meta.data?.audit_head)}</Row>
      </Section>

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
      <dd className="text-text-body min-w-0">{children}</dd>
    </>
  );
}

type ServerState = 'connected' | 'rebuilding' | 'disconnected' | 'unknown';

function serverState(q: { isPending: boolean; isError: boolean; error: unknown }): ServerState {
  if (q.isPending) return 'unknown';
  if (q.isError) {
    return q.error instanceof ServiceUnavailableError ? 'rebuilding' : 'disconnected';
  }
  return 'connected';
}

function serverStateLabel(s: ServerState): string {
  switch (s) {
    case 'connected':
      return 'connected';
    case 'rebuilding':
      return 'rebuilding index (503)';
    case 'disconnected':
      return 'disconnected';
    case 'unknown':
      return 'checking…';
  }
}

function StatusDot({ state }: { state: ServerState }) {
  const color =
    state === 'connected'
      ? 'bg-status-healthy'
      : state === 'rebuilding'
        ? 'bg-status-degraded'
        : state === 'disconnected'
          ? 'bg-sev-critical'
          : 'bg-text-subtle';
  return <span className={cn('inline-block h-2 w-2 rounded-full align-middle', color)} />;
}
