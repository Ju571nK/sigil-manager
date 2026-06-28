import { describe, expect, it } from 'vitest';
import type { LicenseStatus } from '@/api/fleet';
import { licenseAlert, licenseHostSummary } from './license';

function lic(partial: Partial<LicenseStatus>): LicenseStatus {
  return {
    state: 'ok',
    licensed: true,
    expired: false,
    effective_max_hosts: 50,
    current_host_count: 5,
    active_window_days: 30,
    customer_id: null,
    license_id: null,
    not_after: null,
    ...partial,
  };
}

describe('licenseAlert', () => {
  it('returns null when there is no license block (open-source / older server)', () => {
    expect(licenseAlert(null)).toBeNull();
    expect(licenseAlert(undefined)).toBeNull();
  });

  it('returns null for a healthy license (ok, not expired)', () => {
    expect(licenseAlert(lic({ state: 'ok', expired: false }))).toBeNull();
  });

  it('warns when over the host limit', () => {
    const a = licenseAlert(lic({ state: 'over_limit', current_host_count: 60 }));
    expect(a?.severity).toBe('warn');
    expect(a?.message).toContain('60');
    expect(a?.message).toContain('50');
  });

  it('errors when expired', () => {
    const a = licenseAlert(lic({ expired: true }));
    expect(a?.severity).toBe('error');
    expect(a?.message).toContain('expired');
  });

  it('ranks expired above over_limit when both hold', () => {
    expect(licenseAlert(lic({ expired: true, state: 'over_limit' }))?.severity).toBe('error');
  });
});

describe('licenseHostSummary', () => {
  it('returns null when there is no license block', () => {
    expect(licenseHostSummary(null)).toBeNull();
    expect(licenseHostSummary(undefined)).toBeNull();
  });

  it('formats current / effective host counts', () => {
    expect(licenseHostSummary(lic({ current_host_count: 5, effective_max_hosts: 50 }))).toBe(
      '5 / 50 licensed hosts',
    );
  });
});
