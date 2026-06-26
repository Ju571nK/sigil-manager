import { describe, expect, it } from 'vitest';
import { type Event, extractHook } from './fleet';

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
