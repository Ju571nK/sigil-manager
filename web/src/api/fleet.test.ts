import { describe, expect, it } from 'vitest';
import { type Event, extractHook, extractToggleDrift } from './fleet';

/** Minimal Event wrapper around an evidence payload for extractor tests. */
function ev(evidence: Record<string, unknown>): Event {
  return {
    schema_version: 1,
    event_id: 'e1',
    ts: '2026-06-25T00:00:00Z',
    host_id: 'h1',
    agent_version: '0.6.2',
    severity: 'warn',
    source: { kind: 'agent_hook' },
    subject: {},
    evidence: evidence as Event['evidence'],
    target_id: null,
  };
}

describe('extractHook', () => {
  it('returns the typed hook view for each of the four hook kinds (§14.9.2)', () => {
    const kinds = [
      'hook_invocation',
      'hook_decision',
      'hook_config_drift',
      'possible_hook_activity_silent',
    ];
    for (const kind of kinds) {
      expect(extractHook(ev({ kind, agent: 'grok' }))?.kind).toBe(kind);
    }
  });

  it('returns null for non-hook evidence', () => {
    expect(extractHook(ev({ kind: 'ai_guard_risk_assessed', tool: 'codex' }))).toBeNull();
    expect(extractHook(ev({ kind: 'heartbeat' }))).toBeNull();
  });
});

describe('extractToggleDrift', () => {
  it('returns the typed view for ai_guard_toggle_drift (§14.10)', () => {
    const d = extractToggleDrift(
      ev({
        kind: 'ai_guard_toggle_drift',
        tool: 'claude_code',
        scope: { kind: 'user_global' },
        toggle: 'auto_approval_enabled',
      }),
    );
    expect(d?.kind).toBe('ai_guard_toggle_drift');
    expect(d?.toggle).toBe('auto_approval_enabled');
    expect(d?.tool).toBe('claude_code');
  });

  it('returns null for other evidence (incl. ai_guard_risk_assessed)', () => {
    expect(extractToggleDrift(ev({ kind: 'ai_guard_risk_assessed', tool: 'codex' }))).toBeNull();
    expect(extractToggleDrift(ev({ kind: 'hook_decision', agent: 'grok' }))).toBeNull();
  });
});
