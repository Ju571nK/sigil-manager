import type { AuditHead } from '@/api/fleet';

/**
 * One-line audit-signing status for the Settings page (contract §14.9.3).
 * The console treats `audit_head` as opaque — it reports presence, never
 * verifies the chain. Absent/null (signing disabled or older server) →
 * "disabled".
 */
export function auditSigningSummary(head: AuditHead | null | undefined): string {
  if (!head) return 'disabled';
  return `enabled · seq ${head.seq} · key ${head.pubkey_id}`;
}
