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
  it('distinguishes absent and null without inferring disabled signing', () => {
    expect(auditSigningSummary(null)).toBe('no signed head available');
    expect(auditSigningSummary(undefined)).toBe('not reported by this server');
  });

  it('reports enabled with seq and key id', () => {
    expect(auditSigningSummary(head)).toBe(
      'signed head reported · seq 4211 · key key-1 (not verified)',
    );
  });
});
