import type { ComplianceRow } from '@/api/fleet';

export type ComplianceStatus = 'in_sync' | 'drift' | 'expired' | 'failing_signature';

/**
 * Derives the per-host compliance pill from raw signals (contract §5.6).
 * The server exposes no compliance_score (F13); the rule lives here.
 * Single worst state, priority: expired > failing_signature > drift > in_sync.
 */
export function deriveComplianceStatus(row: ComplianceRow): ComplianceStatus {
  if (row.policy_expired_active) return 'expired';
  if (row.signature_failures_24h > 0) return 'failing_signature';
  if (row.version_drift > 0) return 'drift';
  return 'in_sync';
}

/** Display metadata for each status — label + the severity token to color it. */
export const COMPLIANCE_META: Record<
  ComplianceStatus,
  { label: string; tone: 'healthy' | 'degraded' | 'down' }
> = {
  in_sync: { label: 'In sync', tone: 'healthy' },
  drift: { label: 'Drift', tone: 'degraded' },
  expired: { label: 'Expired', tone: 'down' },
  failing_signature: { label: 'Failing signature', tone: 'down' },
};
