import { describe, expect, it } from 'vitest';
import type { ComplianceRow } from '@/api/fleet';
import { deriveComplianceStatus } from './compliance';

function row(partial: Partial<ComplianceRow>): ComplianceRow {
  return {
    host_id: 'h',
    hostname: 'h',
    last_applied_policy_version: 18,
    server_current_policy_version: 18,
    version_drift: 0,
    policy_expired_active: false,
    last_policy_reload_ts: null,
    signature_failures_24h: 0,
    ...partial,
  };
}

describe('deriveComplianceStatus', () => {
  it('returns in_sync when all signals are clean', () => {
    expect(deriveComplianceStatus(row({}))).toBe('in_sync');
  });

  it('returns drift when version_drift > 0', () => {
    expect(deriveComplianceStatus(row({ version_drift: 2 }))).toBe('drift');
  });

  it('returns expired when policy_expired_active', () => {
    expect(deriveComplianceStatus(row({ policy_expired_active: true }))).toBe('expired');
  });

  it('returns failing_signature when signature_failures_24h > 0', () => {
    expect(deriveComplianceStatus(row({ signature_failures_24h: 1 }))).toBe('failing_signature');
  });

  it('prioritizes expired over everything', () => {
    expect(
      deriveComplianceStatus(
        row({ policy_expired_active: true, signature_failures_24h: 5, version_drift: 3 }),
      ),
    ).toBe('expired');
  });

  it('prioritizes failing_signature over drift', () => {
    expect(deriveComplianceStatus(row({ signature_failures_24h: 2, version_drift: 3 }))).toBe(
      'failing_signature',
    );
  });
});
