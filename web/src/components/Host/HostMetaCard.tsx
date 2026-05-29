import type { HostMeta } from '@/api/fleet';

/** Host metadata block (§5.4 host_meta). Null = host hasn't sent a
 *  HostMetaSnapshot yet, NOT an error. */
export function HostMetaCard({ meta }: { meta: HostMeta | null }) {
  return (
    <Card title="Host metadata">
      {!meta ? (
        <Empty>No host metadata reported yet.</Empty>
      ) : (
        <dl className="space-y-1 text-xs">
          <Row label="OS">
            {meta.os_name} {meta.os_version}
          </Row>
          <Row label="Kernel">{meta.kernel_version}</Row>
          <Row label="Arch">{meta.architecture}</Row>
          <Row label="Gateway">{meta.default_gateway_v4 ?? '—'}</Row>
          <Row label="DNS">{meta.dns_servers?.length ? meta.dns_servers.join(', ') : '—'}</Row>
          <Row label="Interfaces">
            <ul className="space-y-0.5">
              {(meta.interfaces ?? []).map((i) => (
                <li key={i.name} className="font-mono">
                  {i.name}
                  {i.ipv4 && i.ipv4.length > 0 && (
                    <span className="text-text-subtle"> · {i.ipv4.join(', ')}</span>
                  )}
                  {i.mac && <span className="text-text-subtle"> · {i.mac}</span>}
                </li>
              ))}
            </ul>
          </Row>
        </dl>
      )}
    </Card>
  );
}

export function Card({ title, children }: { title: string; children: React.ReactNode }) {
  return (
    <section className="rounded-md border border-border bg-bg-surface p-3">
      <h2 className="mb-2 text-xs font-medium uppercase tracking-wide text-text-subtle">{title}</h2>
      {children}
    </section>
  );
}

export function Empty({ children }: { children: React.ReactNode }) {
  return <p className="text-xs text-text-muted">{children}</p>;
}

function Row({ label, children }: { label: string; children: React.ReactNode }) {
  return (
    <div className="flex gap-2">
      <dt className="w-20 shrink-0 text-text-subtle">{label}</dt>
      <dd className="text-text-body">{children}</dd>
    </div>
  );
}
