import type { AgentHealth, PolicyState } from '@/api/fleet';
import { Card, Empty } from '@/components/Host/HostMetaCard';
import { relativeAge } from '@/lib/time';

interface Props {
  policy: PolicyState | null;
  health: AgentHealth | null;
  /** From the compliance row (F13); shown alongside policy when present. */
  signatureFailures24h?: number;
}

/** Policy state + agent health (§5.4). Disconnected hosts carry many nulls;
 *  every field degrades to "—" / "not reported yet". */
export function PolicyHealthCard({ policy, health, signatureFailures24h }: Props) {
  return (
    <Card title="Policy & agent health">
      <dl className="space-y-1 text-xs">
        {policy ? (
          <>
            <Row label="Policy">
              v{policy.last_applied_policy_version}
              {policy.policy_expired_active ? ' · expired' : ''}
            </Row>
            <Row label="Reloaded">
              {policy.last_policy_reload_ts
                ? `${relativeAge(policy.last_policy_reload_ts)} ago`
                : '—'}
            </Row>
          </>
        ) : (
          <Row label="Policy">
            <Empty>not reported yet</Empty>
          </Row>
        )}
        {typeof signatureFailures24h === 'number' && (
          <Row label="Sig fails 24h">{signatureFailures24h}</Row>
        )}
        <div className="my-1 border-t border-border-subtle" />
        {health ? (
          <>
            <Row label="Heartbeat">
              {health.last_heartbeat_ts ? `${relativeAge(health.last_heartbeat_ts)} ago` : '—'}
            </Row>
            <Row label="Stalls 24h">{health.recent_channel_stalls_24h}</Row>
            <Row label="Watcher 24h">{health.recent_watcher_degraded_24h}</Row>
            <Row label="Sender lag 24h">{health.recent_sender_lag_critical_24h}</Row>
            <Row label="Hash p99">
              {health.hash_p99_ms_latest != null ? `${health.hash_p99_ms_latest} ms` : '—'}
            </Row>
          </>
        ) : (
          <Row label="Health">
            <Empty>not reported yet</Empty>
          </Row>
        )}
      </dl>
    </Card>
  );
}

function Row({ label, children }: { label: string; children: React.ReactNode }) {
  return (
    <div className="flex gap-2">
      <dt className="w-28 shrink-0 text-text-subtle">{label}</dt>
      <dd className="text-text-body">{children}</dd>
    </div>
  );
}
