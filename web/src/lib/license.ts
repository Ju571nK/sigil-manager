import type { LicenseStatus } from '@/api/fleet';

/** A license condition that needs operator attention, for the banner. */
export interface LicenseAlert {
  severity: 'warn' | 'error';
  message: string;
}

/**
 * Returns a [`LicenseAlert`] when the license needs attention — expired
 * (error) or over the host limit (warn) — else null. Expired outranks
 * over-limit. A null/absent license (open-source / older server, contract
 * §14.9.3) yields no alert.
 */
export function licenseAlert(license: LicenseStatus | null | undefined): LicenseAlert | null {
  if (!license) return null;
  if (license.expired) {
    return {
      severity: 'error',
      message: `License expired — ${license.current_host_count} of ${license.effective_max_hosts} licensed hosts in use.`,
    };
  }
  if (license.state === 'over_limit') {
    return {
      severity: 'warn',
      message: `Over host limit — ${license.current_host_count} hosts active, ${license.effective_max_hosts} licensed.`,
    };
  }
  return null;
}

/**
 * Short "N / M licensed hosts" summary for the fleet header, or null when no
 * license block is present.
 */
export function licenseHostSummary(license: LicenseStatus | null | undefined): string | null {
  if (!license) return null;
  return `${license.current_host_count} / ${license.effective_max_hosts} licensed hosts`;
}
