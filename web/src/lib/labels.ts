import type { Scope } from '@/api/fleet';

/** Maps an AI-tool wire string (contract §14.5/§14.7) to a display name. */
export function humanTool(tool: string): string {
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

/** Collapses a long filesystem path to "…/last/two" segments; short paths (≤2 segments) pass through unchanged. */
export function shortPath(p: string): string {
  const parts = p.split('/').filter(Boolean);
  if (parts.length <= 2) return p;
  return `…/${parts.slice(-2).join('/')}`;
}

/** Humanizes an AI Guard scope (§14.5) for display: "project · …/path",
 *  "app · name", or "user global". Shared by the alerts SlideOver and the
 *  host-detail AI Guard block so the rendering can't diverge. */
export function scopeLabel(scope: Scope): string {
  if (scope.kind === 'project') return `project · ${shortPath(scope.path)}`;
  if (scope.kind === 'application') return `app · ${scope.app}`;
  return 'user global';
}
