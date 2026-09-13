import type { AuditHead } from '@/api/fleet';

/** Presence of a head is reported, never cryptographically verified by the UI. */
export function auditSigningSummary(head: AuditHead | null | undefined): string {
  if (head === undefined) return 'not reported by this server';
  if (head === null) return 'no signed head available';
  return `signed head reported · seq ${head.seq} · key ${head.pubkey_id} (not verified)`;
}
