import type { ReasonLike } from '@/api/fleet';
import { humanKind, shortPath } from '@/lib/labels';

/**
 * Renders AI Guard reasons (contract §14.5 variant shapes: no_sandbox,
 * broad_matcher, permissions_deny_empty, mcp_server_local_command, the
 * source_chain breadcrumb, …). Shared by the alerts SlideOver and the
 * host-detail AI Guard block.
 */
export function ReasonList({ reasons }: { reasons: ReasonLike[] }) {
  return (
    <ul className="space-y-0.5">
      {reasons.map((r) => (
        <ReasonItem key={reasonKey(r)} reason={r} />
      ))}
    </ul>
  );
}

/** One reason row. Each AI Guard reason has a `kind` plus a few open fields
 *  (pattern/executor/server_name/url/command/source_chain) surfaced inline. */
function ReasonItem({ reason: r }: { reason: ReasonLike }) {
  const serverName = asString(r.server_name);
  const url = asString(r.url);
  const command = asString(r.command);
  const chain = asStringArray(r.source_chain);

  return (
    <li>
      <span className="text-text-primary">{humanKind(r.kind)}</span>
      {r.pattern && (
        <span className="ml-1 text-text-muted">
          · pattern <code className="font-mono">{r.pattern}</code>
        </span>
      )}
      {r.executor && <span className="ml-1 text-text-muted">· executor {r.executor}</span>}
      {serverName && (
        <span className="ml-1 text-text-muted">
          · server <code className="font-mono">{serverName}</code>
        </span>
      )}
      {url && (
        <span className="ml-1 text-text-muted">
          · url <code className="font-mono break-all">{url}</code>
        </span>
      )}
      {command && (
        <span className="ml-1 text-text-muted">
          · command <code className="font-mono break-all">{command}</code>
        </span>
      )}
      {/* 3b.3.1 source-follow breadcrumb (contract §14.8). */}
      {chain.length > 0 && (
        <span className="ml-1 block text-text-muted">
          {chain.map((p, i) => (
            <span key={p}>
              {i > 0 && <span className="mx-1 text-text-subtle">→</span>}
              <code className="font-mono">{shortPath(p)}</code>
            </span>
          ))}
        </span>
      )}
    </li>
  );
}

function reasonKey(r: ReasonLike): string {
  const chain = asStringArray(r.source_chain).join('>');
  return `${r.kind}:${r.pattern ?? ''}:${r.hook_event ?? ''}:${r.executor ?? ''}:${asString(r.server_name) ?? ''}:${chain}`;
}

function asString(v: unknown): string | undefined {
  return typeof v === 'string' && v.length > 0 ? v : undefined;
}

function asStringArray(v: unknown): string[] {
  return Array.isArray(v) ? v.filter((x): x is string => typeof x === 'string') : [];
}
