import type { ReasonLike } from '@/api/fleet';
import { humanKind, shortPath } from '@/lib/labels';

/**
 * Renders AI Guard reasons (contract §14.11 variant census: no_sandbox,
 * broad_matcher, mcp_server_suspicious_launcher, instruction_file_directive,
 * the source_chain breadcrumb, …). Shared by the alerts SlideOver and the
 * host-detail AI Guard block. Open-shape: unknown kinds still render via
 * humanKind plus whichever known fields they carry.
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
 *  (pattern/executor/server_name/url/command/shape/evidence/mechanism/…,
 *  contract §14.11) surfaced inline when present. */
function ReasonItem({ reason: r }: { reason: ReasonLike }) {
  const serverName = asString(r.server_name);
  const url = asString(r.url);
  const command = asString(r.command);
  const chain = asStringArray(r.source_chain);
  // §14.11 additions.
  const shape = asString(r.shape); // mcp_server_suspicious_launcher
  const evidence = asString(r.evidence);
  const mechanism = asString(r.mechanism); // project_mcp_auto_enabled
  const mode = asString(r.mode); // auto_approval_enabled
  const rule = asString(r.rule); // permissions_allow_broad
  const matcher = asString(r.matcher); // broad_matcher
  const name = asString(r.name); // unattended_scheduled_task
  const path = asString(r.path); // instruction_file_directive / *_script
  const directiveKind = asString(r.directive_kind);
  const snippet = asString(r.snippet);

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
      {shape && <span className="ml-1 text-text-muted">· shape {humanKind(shape)}</span>}
      {evidence && (
        <span className="ml-1 text-text-muted">
          · match <code className="font-mono break-all">{evidence}</code>
        </span>
      )}
      {mechanism && <span className="ml-1 text-text-muted">· via {mechanism}</span>}
      {mode && <span className="ml-1 text-text-muted">· mode {mode}</span>}
      {rule && (
        <span className="ml-1 text-text-muted">
          · rule <code className="font-mono">{rule}</code>
        </span>
      )}
      {matcher && (
        <span className="ml-1 text-text-muted">
          · matcher <code className="font-mono">{matcher}</code>
        </span>
      )}
      {name && (
        <span className="ml-1 text-text-muted">
          · task <code className="font-mono">{name}</code>
        </span>
      )}
      {path && (
        <span className="ml-1 text-text-muted">
          · <code className="font-mono">{shortPath(path)}</code>
        </span>
      )}
      {directiveKind && (
        <span className="ml-1 text-text-muted">· directive {humanKind(directiveKind)}</span>
      )}
      {snippet && (
        <span className="ml-1 block text-text-muted">
          <code className="font-mono break-all">{snippet}</code>
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
  // §14.11 kinds are distinguished by path/name/mechanism/rule/matcher/mode —
  // without them two instruction_file_directive reasons for different files
  // would collide on the same React key.
  const extra = [r.path, r.name, r.mechanism, r.rule, r.matcher, r.mode]
    .map((v) => asString(v) ?? '')
    .join(':');
  return `${r.kind}:${r.pattern ?? ''}:${r.hook_event ?? ''}:${r.executor ?? ''}:${asString(r.server_name) ?? ''}:${extra}:${chain}`;
}

function asString(v: unknown): string | undefined {
  return typeof v === 'string' && v.length > 0 ? v : undefined;
}

function asStringArray(v: unknown): string[] {
  return Array.isArray(v) ? v.filter((x): x is string => typeof x === 'string') : [];
}
