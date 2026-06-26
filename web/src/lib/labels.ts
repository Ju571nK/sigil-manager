import type { HookEvidence, Scope } from '@/api/fleet';

/**
 * Maps an AI-tool wire string (contract §14.5/§14.7/§14.9.1) to a display name.
 *
 * `toolLabel` is the operator-supplied `tool_label` sibling field (§14.9.1):
 * when `tool === "other"` and a non-empty label is present, that label wins
 * over the generic "Other". It is ignored for every built-in tool, so the
 * mapping stays stable for known wire values.
 */
export function humanTool(tool: string, toolLabel?: string | null): string {
  if (tool === 'other' && toolLabel) return toolLabel;
  switch (tool) {
    case 'claude_code':
      return 'Claude Code';
    case 'claude_desktop':
      return 'Claude Desktop';
    case 'continue_dev':
      return 'Continue.dev';
    case 'codex':
      return 'Codex';
    case 'gemini':
      return 'Gemini';
    case 'cursor':
      return 'Cursor';
    case 'antigravity':
      return 'Antigravity';
    case 'grok':
      return 'Grok';
    case 'other':
      return 'Other';
    default:
      return tool;
  }
}

/** Title-cases a snake_case wire kind/reason string for display, e.g. "policy_reloaded" → "Policy Reloaded". */
export function humanKind(kind: string): string {
  return kind
    .split('_')
    .map((s) => (s.length ? s[0].toUpperCase() + s.slice(1) : s))
    .join(' ');
}

/** Maps a hook_decision `decision` wire string to a past-tense verb. Unknown
 *  decisions pass through unchanged (so a future variant still reads sensibly). */
const HOOK_DECISION_VERB: Record<string, string> = {
  deny: 'denied',
  block: 'denied',
  allow: 'allowed',
  warn: 'warned',
};

/**
 * Builds a one-line queue/slide-over title for a sigil-hook evidence
 * (contract §14.9.2). Always leads with the agent tool (via [`humanTool`],
 * honoring `other_label` for an `agent === "other"` invocation); a deny/allow
 * decision additionally surfaces the rule id when present.
 */
export function hookTitle(hook: HookEvidence): string {
  const label = hook.kind === 'hook_invocation' ? hook.other_label : undefined;
  const tool = humanTool(hook.agent, label);
  switch (hook.kind) {
    case 'hook_invocation':
      return `Hook activity · ${tool}`;
    case 'hook_decision': {
      const verb = HOOK_DECISION_VERB[hook.decision] ?? hook.decision;
      const base = `Hook ${verb} · ${tool}`;
      return hook.rule_id ? `${base} — ${hook.rule_id}` : base;
    }
    case 'hook_config_drift':
      return `Hook config drift · ${tool}`;
    case 'possible_hook_activity_silent':
      return `Possible silent hook · ${tool}`;
  }
}

/** Collapses a long filesystem path to "…/last/two" segments; short paths (≤2 segments) pass through unchanged. Empty/missing → "". */
export function shortPath(p: string | null | undefined): string {
  if (!p) return '';
  const parts = p.split('/').filter(Boolean);
  if (parts.length <= 2) return p;
  return `…/${parts.slice(-2).join('/')}`;
}

/** Humanizes an AI Guard scope (§14.5) for display: "project · …/path",
 *  "app · name", or "user global". Shared by the alerts SlideOver and the
 *  host-detail AI Guard block so the rendering can't diverge. Tolerates a
 *  null/absent scope and a project scope missing its `path` (omitempty wire). */
export function scopeLabel(scope: Scope | null | undefined): string {
  if (!scope) return '—';
  if (scope.kind === 'project') return `project · ${shortPath(scope.path)}`;
  if (scope.kind === 'application') return `app · ${scope.app}`;
  return 'user global';
}
