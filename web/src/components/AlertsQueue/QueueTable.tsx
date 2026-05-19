import { useEffect, useMemo, useRef, useState } from 'react';
import type { EventWithTriage } from '@/api/fleet';
import { QueueRow } from './QueueRow';

interface Props {
  events: EventWithTriage[];
  selectedEventID: string | null;
  onSelect: (event: EventWithTriage) => void;
  onRowHoverEnter: () => void;
  onRowHoverLeave: () => void;
  sort: SortMode;
  onSortChange: (next: SortMode) => void;
  /** True before the first successful fetch — show skeleton rows. */
  isPending?: boolean;
  /** True if the visible row set is empty because filters excluded everything. */
  filtersActive?: boolean;
  /** When the empty state is filter-driven, this resets the filter to defaults. */
  onResetFilters?: () => void;
}

export type SortMode = 'severity_desc' | 'age_desc' | 'age_asc';

const BUCKET_RANK: Record<string, number> = {
  critical: 4,
  high: 3,
  medium: 2,
  low: 1,
  info: 0,
};

/**
 * 28px-density alerts table per UI/UX §6.4 with sticky header. Sort modes
 * follow UI/UX §7.2: default severity desc, then age desc. Clicking a
 * sortable column header toggles its direction (severity desc only — no
 * asc for severity in v1).
 *
 * Rows that appear on this render but weren't in the previous render get
 * `isFresh=true` for one flash-cycle, driving a 1s box-shadow fade-in.
 */
export function QueueTable({
  events,
  selectedEventID,
  onSelect,
  onRowHoverEnter,
  onRowHoverLeave,
  sort,
  onSortChange,
  isPending,
  filtersActive,
  onResetFilters,
}: Props) {
  const sorted = useMemo(() => sortEvents(events, sort), [events, sort]);

  const previousIDs = useRef<Set<string>>(new Set());
  const [freshIDs, setFreshIDs] = useState<Set<string>>(new Set());

  useEffect(() => {
    const incoming = new Set(events.map((e) => e.event_id));
    const newOnes = new Set<string>();
    for (const id of incoming) {
      if (!previousIDs.current.has(id)) newOnes.add(id);
    }
    if (newOnes.size > 0 && previousIDs.current.size > 0) {
      setFreshIDs(newOnes);
      const t = setTimeout(() => setFreshIDs(new Set()), 1100);
      previousIDs.current = incoming;
      return () => clearTimeout(t);
    }
    previousIDs.current = incoming;
  }, [events]);

  return (
    <div className="flex flex-col overflow-hidden rounded-md border border-border bg-bg-surface">
      <Header sort={sort} onSortChange={onSortChange} />
      <div className="max-h-[calc(100vh-220px)] overflow-y-auto">
        {isPending ? (
          <SkeletonRows />
        ) : sorted.length === 0 ? (
          <EmptyHint filtersActive={filtersActive} onResetFilters={onResetFilters} />
        ) : (
          sorted.map((ev) => (
            <QueueRow
              key={`${ev.host_id}:${ev.event_id}`}
              event={ev}
              selected={ev.event_id === selectedEventID}
              onSelect={onSelect}
              onHoverEnter={onRowHoverEnter}
              onHoverLeave={onRowHoverLeave}
              isFresh={freshIDs.has(ev.event_id)}
            />
          ))
        )}
      </div>
    </div>
  );
}

function Header({
  sort,
  onSortChange,
}: {
  sort: SortMode;
  onSortChange: (next: SortMode) => void;
}) {
  return (
    <div className="grid h-[28px] w-full grid-cols-[16px_64px_minmax(0,1fr)_140px_72px_92px_120px] items-center gap-3 border-b border-border bg-bg-elevated px-3 text-[10px] uppercase tracking-wide text-text-subtle">
      <span aria-hidden />
      <button
        type="button"
        onClick={() => onSortChange(sort === 'age_desc' ? 'age_asc' : 'age_desc')}
        className="text-left hover:text-text-primary"
      >
        Age {sort === 'age_desc' ? '↓' : sort === 'age_asc' ? '↑' : ''}
      </button>
      <span>Event</span>
      <span>Host</span>
      <button
        type="button"
        onClick={() => onSortChange('severity_desc')}
        className="text-left hover:text-text-primary"
      >
        Severity {sort === 'severity_desc' ? '↓' : ''}
      </button>
      <span>Status</span>
      <span>Assignee</span>
    </div>
  );
}

function EmptyHint({
  filtersActive,
  onResetFilters,
}: {
  filtersActive?: boolean;
  onResetFilters?: () => void;
}) {
  if (filtersActive) {
    return (
      <div className="flex h-32 flex-col items-center justify-center gap-2 text-sm text-text-subtle">
        <span>No alerts match the current filters.</span>
        {onResetFilters && (
          <button
            type="button"
            onClick={onResetFilters}
            className="text-xs text-accent underline-offset-2 hover:underline"
          >
            Reset filters
          </button>
        )}
      </div>
    );
  }
  // Genuinely empty (no alerts at all under current filters) — celebrate
  // until UI/UX D2's "last incident N days ago" data is available
  // (Plan 04 host-detail will provide last-seen severity history).
  return (
    <div className="flex h-32 items-center justify-center text-sm text-text-muted">
      <span>🎉 No open alerts.</span>
    </div>
  );
}

/**
 * Five 28px skeleton rows with a shimmer animation while the first fetch
 * is in flight. Subsequent polling refreshes don't show this — the
 * FreshnessIndicator surfaces "Refreshing…" instead so the queue doesn't
 * jump.
 */
function SkeletonRows() {
  return (
    <div aria-hidden="true">
      {Array.from({ length: 5 }).map((_, i) => (
        <div
          // biome-ignore lint/suspicious/noArrayIndexKey: positional placeholder, no identity
          key={i}
          className="grid h-[28px] w-full grid-cols-[16px_64px_minmax(0,1fr)_140px_72px_92px_120px] items-center gap-3 border-b border-border-subtle px-3"
        >
          <span className="block h-2 w-2 rounded-full bg-bg-elevated" />
          <span className="block h-2 w-12 animate-pulse rounded bg-bg-elevated" />
          <span className="block h-2 w-3/4 animate-pulse rounded bg-bg-elevated" />
          <span className="block h-2 w-24 animate-pulse rounded bg-bg-elevated" />
          <span className="block h-2 w-10 animate-pulse rounded bg-bg-elevated" />
          <span className="block h-2 w-14 animate-pulse rounded bg-bg-elevated" />
          <span className="block h-2 w-16 animate-pulse rounded bg-bg-elevated" />
        </div>
      ))}
    </div>
  );
}

function sortEvents(events: EventWithTriage[], mode: SortMode): EventWithTriage[] {
  const copy = [...events];
  switch (mode) {
    case 'severity_desc':
      // Severity desc, then age desc as the secondary key.
      copy.sort((a, b) => {
        const sa = bucketRankFor(a);
        const sb = bucketRankFor(b);
        if (sa !== sb) return sb - sa;
        return tsCompareDesc(a.ts, b.ts);
      });
      break;
    case 'age_desc':
      copy.sort((a, b) => tsCompareDesc(a.ts, b.ts));
      break;
    case 'age_asc':
      copy.sort((a, b) => -tsCompareDesc(a.ts, b.ts));
      break;
  }
  return copy;
}

function bucketRankFor(ev: EventWithTriage): number {
  if (ev.evidence?.kind === 'ai_guard_risk_assessed') {
    const bucket = (ev.evidence as { bucket?: string }).bucket;
    return bucket ? (BUCKET_RANK[bucket] ?? 0) : 0;
  }
  // Non-AI-guard events: warn=2, info=0.
  return ev.severity === 'warn' ? 2 : 0;
}

function tsCompareDesc(a: string, b: string): number {
  // Lex compare on RFC3339 is chronological-correct.
  if (a < b) return 1;
  if (a > b) return -1;
  return 0;
}
