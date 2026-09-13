import { useState } from 'react';
import type { ReasonLike } from '@/api/fleet';
import { humanKind, shortPath } from '@/lib/labels';

const FIELD_LABELS: Record<string, string> = {
  pattern: 'pattern',
  executor: 'executor',
  server_name: 'server',
  server: 'server',
  tool: 'tool',
  servers: 'servers',
  url: 'url',
  command: 'command',
  shape: 'shape',
  evidence: 'match',
  mechanism: 'via',
  mode: 'mode',
  rule: 'rule',
  matcher: 'matcher',
  name: 'task',
  path: 'path',
  script_path: 'script path',
  hook_event: 'hook event',
  directive_kind: 'directive',
  snippet: 'snippet',
  text_kind: 'hidden text',
  baseline_hash: 'baseline hash',
  current_hash: 'current hash',
  list: 'replaced defaults',
  destination: 'destination',
  source: 'source',
};

const NOTES: Record<string, string> = {
  mcp_tool_surface_drift:
    'Compared with the first observed baseline, not an operator approval or signature.',
  mcp_unapproved_new_tool:
    'Not in the first observed baseline; this does not establish an approval violation.',
  mcp_schema_privilege_expansion:
    'Schema change relative to the first observed baseline; not proof of runtime execution.',
  mcp_read_only_hint_contradiction:
    'Metadata heuristic; not proof of a write or an approval bypass.',
};

/** All reason fields remain inspectable, including unknown additive fields. */
export function ReasonList({ reasons }: { reasons: ReasonLike[] }) {
  const occurrences = new Map<string, number>();
  return (
    <ul className="space-y-2">
      {reasons.map((reason) => {
        const identity = JSON.stringify(
          Object.entries(reason).sort(([a], [b]) => a.localeCompare(b)),
        );
        const occurrence = occurrences.get(identity) ?? 0;
        occurrences.set(identity, occurrence + 1);
        return <ReasonItem key={`${identity}:${occurrence}`} reason={reason} />;
      })}
    </ul>
  );
}

function ReasonItem({ reason }: { reason: ReasonLike }) {
  const chain = Array.isArray(reason.source_chain)
    ? reason.source_chain.filter((p): p is string => typeof p === 'string')
    : [];
  return (
    <li className="min-w-0">
      <span className="text-text-primary">{humanKind(reason.kind)}</span>
      {Object.entries(reason)
        .filter(([key, value]) => key !== 'kind' && key !== 'source_chain' && value != null)
        .map(([key, value]) => (
          <div key={key} className="text-text-muted">
            <span>{FIELD_LABELS[key] ?? humanKind(key)}: </span>
            <EvidenceValue value={value} />
          </div>
        ))}
      {chain.length > 0 && (
        <div className="text-text-muted">
          <span>Source chain: </span>
          {chain.map((path, index) => (
            <span key={path}>
              {index > 0 && <span className="mx-1">→</span>}
              <code title={path} className="break-all">
                {shortPath(path)}
              </code>
            </span>
          ))}
          <details>
            <summary className="cursor-pointer">Full source paths</summary>
            <EvidenceValue value={chain} />
          </details>
        </div>
      )}
      {NOTES[reason.kind] && <p className="mt-1 text-text-muted">{NOTES[reason.kind]}</p>}
    </li>
  );
}

function EvidenceValue({ value }: { value: unknown }) {
  const text = typeof value === 'string' ? value : JSON.stringify(value);
  const [copyState, setCopyState] = useState('Copy');
  if (!text || text.length <= 96)
    return <code className="whitespace-pre-wrap break-all font-mono">{text}</code>;
  return (
    <details className="inline">
      <summary className="cursor-pointer break-all font-mono">
        {text.slice(0, 64)}… (full value)
      </summary>
      <code className="block whitespace-pre-wrap break-all font-mono">{text}</code>
      <button
        type="button"
        className="text-accent"
        onClick={async () => {
          try {
            await navigator.clipboard.writeText(text);
            setCopyState('Copied');
          } catch {
            setCopyState('Copy unavailable — select text above');
          }
        }}
      >
        {copyState}
      </button>
    </details>
  );
}
