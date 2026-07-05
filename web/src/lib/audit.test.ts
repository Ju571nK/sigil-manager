import { describe, expect, it } from 'vitest';
import type { AuditHead } from '@/api/fleet';
import { auditSigningSummary } from './audit';

const head: AuditHead = {
  seq: 4211,
  hash: 'abc',
  sig: 'def',
  pubkey_id: 'key-1',
  pubkey: 'ed25519:AAAA',
};

describe('auditSigningSummary', () => {
  it('reports disabled when the server omits audit_head (§14.9.3)', () => {
    expect(auditSigningSummary(null)).toBe('disabled');
    expect(auditSigningSummary(undefined)).toBe('disabled');
  });

  it('reports enabled with seq and key id', () => {
    expect(auditSigningSummary(head)).toBe('enabled · seq 4211 · key key-1');
  });
});
