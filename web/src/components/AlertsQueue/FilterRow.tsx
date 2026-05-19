import { Search } from 'lucide-react';
import { Input } from '@/components/ui/input';
import type { AlertFilter } from '@/hooks/useAlerts';
import { cn } from '@/lib/utils';

interface Props {
  filter: AlertFilter;
  onChange: (next: Partial<AlertFilter>) => void;
}

const BUCKETS: Array<{ value: AlertFilter['minBucket']; label: string }> = [
  { value: 'low', label: 'All' },
  { value: 'medium', label: 'Medium+' },
  { value: 'high', label: 'High+' },
  { value: 'critical', label: 'Critical' },
];

const STATUSES: Array<{
  value: NonNullable<AlertFilter['triageStatuses'][number]>;
  label: string;
}> = [
  { value: 'open', label: 'Open' },
  { value: 'acknowledged', label: 'Ack' },
  { value: 'investigating', label: 'Investigating' },
  { value: 'resolved', label: 'Resolved' },
];

const TIME_RANGES: Array<{ value: 'all' | '24h' | '7d'; label: string }> = [
  { value: 'all', label: 'All' },
  { value: '24h', label: '24h' },
  { value: '7d', label: '7d' },
];

/**
 * Filter row across the top of the Alerts queue. Per UI/UX §7.2 + §7.3:
 *   - Severity ramp (min bucket): server-side filter via `min_ai_guard_bucket`.
 *   - Triage status chips: client-side filter on the triage-join.
 *   - Time range: server-side via `since`.
 *   - Search: client-side fuzzy match against event_id / host / evidence kind
 *     (server-side `?q=` is post-v1 per contract §13).
 */
export function FilterRow({ filter, onChange }: Props) {
  const currentRange = filter.since === null ? 'all' : sinceToRange(filter.since);

  return (
    <div className="flex flex-wrap items-center gap-2 border-b border-border-subtle bg-bg-surface px-3 py-2 text-sm">
      <Group label="Severity">
        {BUCKETS.map((b) => (
          <Chip
            key={b.value}
            active={filter.minBucket === b.value}
            onClick={() => onChange({ minBucket: b.value })}
          >
            {b.label}
          </Chip>
        ))}
      </Group>

      <Divider />

      <Group label="Status">
        {STATUSES.map((s) => {
          const active = filter.triageStatuses.includes(s.value);
          return (
            <Chip
              key={s.value}
              active={active}
              onClick={() => {
                const next = active
                  ? filter.triageStatuses.filter((v) => v !== s.value)
                  : [...filter.triageStatuses, s.value];
                onChange({ triageStatuses: next });
              }}
            >
              {s.label}
            </Chip>
          );
        })}
      </Group>

      <Divider />

      <Group label="Range">
        {TIME_RANGES.map((r) => (
          <Chip
            key={r.value}
            active={currentRange === r.value}
            onClick={() => onChange({ since: rangeToSince(r.value) })}
          >
            {r.label}
          </Chip>
        ))}
      </Group>

      <div className="ml-auto relative">
        <Search className="pointer-events-none absolute left-2 top-1.5 h-3.5 w-3.5 text-text-subtle" />
        <Input
          value={filter.query}
          onChange={(e) => onChange({ query: e.target.value })}
          placeholder="event_id, host, kind…"
          className="h-7 pl-7 w-56 text-xs"
          aria-label="Search alerts"
        />
      </div>
    </div>
  );
}

function Group({ label, children }: { label: string; children: React.ReactNode }) {
  return (
    <div className="flex items-center gap-1">
      <span className="text-xs uppercase tracking-wide text-text-subtle">{label}</span>
      <div className="flex items-center gap-0.5">{children}</div>
    </div>
  );
}

function Divider() {
  return <span className="h-4 w-px bg-border" aria-hidden />;
}

function Chip({
  active,
  onClick,
  children,
}: {
  active: boolean;
  onClick: () => void;
  children: React.ReactNode;
}) {
  return (
    <button
      type="button"
      onClick={onClick}
      className={cn(
        'h-6 rounded px-2 text-xs transition-colors',
        active
          ? 'bg-accent text-bg-page font-medium'
          : 'text-text-muted hover:bg-bg-elevated hover:text-text-primary',
      )}
    >
      {children}
    </button>
  );
}

function rangeToSince(value: 'all' | '24h' | '7d'): string | null {
  if (value === 'all') return null;
  const now = Date.now();
  const offset = value === '24h' ? 24 * 60 * 60 * 1000 : 7 * 24 * 60 * 60 * 1000;
  return new Date(now - offset).toISOString();
}

function sinceToRange(since: string): 'all' | '24h' | '7d' {
  const sinceMs = Date.parse(since);
  if (Number.isNaN(sinceMs)) return 'all';
  const age = Date.now() - sinceMs;
  // Allow some slop for clock skew and rounding.
  if (age <= 36 * 60 * 60 * 1000) return '24h';
  return '7d';
}
