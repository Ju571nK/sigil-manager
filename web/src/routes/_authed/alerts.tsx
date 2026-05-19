import { createFileRoute, useNavigate, useSearch } from '@tanstack/react-router';
import { useState } from 'react';
import type { EventWithTriage } from '@/api/fleet';
import { FilterRow } from '@/components/AlertsQueue/FilterRow';
import { QueueTable, type SortMode } from '@/components/AlertsQueue/QueueTable';
import { type AlertFilter, DEFAULT_FILTER, useAlerts } from '@/hooks/useAlerts';
import { cn } from '@/lib/utils';

/**
 * `/alerts` is Plan 02's landing page (UI/UX D2). Filter state lives in
 * the URL so links + reload preserve view. Every search field is
 * optional in the type so `<Link to="/alerts" />` doesn't have to spell
 * out the full filter; `validateSearch` fills defaults at parse time and
 * the page reads everything through `?? DEFAULT_FILTER.x`.
 */
export interface AlertsSearch {
  minBucket?: AlertFilter['minBucket'];
  triageStatuses?: AlertFilter['triageStatuses'];
  since?: string | null;
  query?: string;
  selected?: string;
  sort?: SortMode;
}

const VALID_BUCKETS: AlertFilter['minBucket'][] = ['low', 'medium', 'high', 'critical'];
const VALID_STATUSES: AlertFilter['triageStatuses'][number][] = [
  'open',
  'acknowledged',
  'investigating',
  'resolved',
];
const VALID_SORTS: SortMode[] = ['severity_desc', 'age_desc', 'age_asc'];

export const Route = createFileRoute('/_authed/alerts')({
  validateSearch: (raw: Record<string, unknown>): AlertsSearch => {
    const out: AlertsSearch = {};
    if (typeof raw.minBucket === 'string' && (VALID_BUCKETS as string[]).includes(raw.minBucket)) {
      out.minBucket = raw.minBucket as AlertFilter['minBucket'];
    }
    const rawStatuses = Array.isArray(raw.triageStatuses)
      ? raw.triageStatuses
      : typeof raw.triageStatuses === 'string'
        ? raw.triageStatuses.split(',')
        : [];
    const triageStatuses = rawStatuses.filter(
      (s): s is AlertFilter['triageStatuses'][number] =>
        typeof s === 'string' && (VALID_STATUSES as string[]).includes(s),
    );
    if (triageStatuses.length > 0) out.triageStatuses = triageStatuses;
    if (typeof raw.since === 'string' && raw.since.length > 0) out.since = raw.since;
    if (typeof raw.query === 'string' && raw.query.length > 0) out.query = raw.query;
    if (typeof raw.selected === 'string' && raw.selected.length > 0) out.selected = raw.selected;
    if (typeof raw.sort === 'string' && (VALID_SORTS as string[]).includes(raw.sort)) {
      out.sort = raw.sort as SortMode;
    }
    return out;
  },
  component: AlertsPage,
});

function AlertsPage() {
  const search = useSearch({ from: '/_authed/alerts' });
  const navigate = useNavigate();

  const filter: AlertFilter = {
    minBucket: search.minBucket ?? DEFAULT_FILTER.minBucket,
    triageStatuses: search.triageStatuses ?? DEFAULT_FILTER.triageStatuses,
    since: search.since ?? DEFAULT_FILTER.since,
    query: search.query ?? DEFAULT_FILTER.query,
  };

  // Local state for the sort + selection so we can update fast without a
  // navigation per click; we sync to URL via `replace: true` to keep the
  // back stack clean.
  const [sort, setSort] = useState<SortMode>(search.sort ?? 'severity_desc');
  const [selectedID, setSelectedID] = useState<string | null>(search.selected ?? null);

  const {
    rows,
    rawCount,
    isPending,
    error,
    lastUpdatedAt,
    isFetching,
    isPaused,
    onRowHoverEnter,
    onRowHoverLeave,
  } = useAlerts(filter);

  const onFilterChange = (next: Partial<AlertFilter>) => {
    navigate({ to: '/alerts', search: { ...search, ...next }, replace: true });
  };

  const onSortChange = (next: SortMode) => {
    setSort(next);
    navigate({ to: '/alerts', search: { ...search, sort: next }, replace: true });
  };

  const onSelect = (event: EventWithTriage) => {
    setSelectedID(event.event_id);
    navigate({
      to: '/alerts',
      search: { ...search, selected: event.event_id },
      replace: true,
    });
    // T12 will mount the slide-over; for now we just record selection in URL.
  };

  return (
    <div className="flex flex-col py-4">
      <div className="mb-3 flex items-baseline justify-between">
        <div>
          <h1 className="text-lg font-semibold text-text-primary">Alerts</h1>
          <p className="text-xs text-text-muted">
            {isPending
              ? 'Loading…'
              : `${rows.length} shown${rawCount !== rows.length ? ` of ${rawCount} fetched` : ''}`}
          </p>
        </div>
        <FreshnessIndicator
          lastUpdatedAt={lastUpdatedAt}
          isFetching={isFetching}
          isPaused={isPaused}
        />
      </div>

      <div className="overflow-hidden rounded-md border border-border bg-bg-surface">
        <FilterRow filter={filter} onChange={onFilterChange} />
        {error ? (
          <div className="px-4 py-6 text-sm text-sev-critical">
            Failed to load alerts: {error.message}
          </div>
        ) : (
          <QueueTable
            events={rows}
            selectedEventID={selectedID}
            onSelect={onSelect}
            onRowHoverEnter={onRowHoverEnter}
            onRowHoverLeave={onRowHoverLeave}
            sort={sort}
            onSortChange={onSortChange}
          />
        )}
      </div>
    </div>
  );
}

function FreshnessIndicator({
  lastUpdatedAt,
  isFetching,
  isPaused,
}: {
  lastUpdatedAt: number;
  isFetching: boolean;
  isPaused: boolean;
}) {
  const [now, setNow] = useState(Date.now());
  // Re-render once a second so the "updated Ns ago" copy stays current.
  useState(() => {
    const id = window.setInterval(() => setNow(Date.now()), 1_000);
    return () => window.clearInterval(id);
  });

  if (!lastUpdatedAt) {
    return <span className="text-xs text-text-subtle">Connecting…</span>;
  }
  const ageSec = Math.max(0, Math.floor((now - lastUpdatedAt) / 1000));
  const tone =
    ageSec < 30 ? 'text-text-muted' : ageSec < 60 ? 'text-status-degraded' : 'text-sev-critical';
  const label = isPaused ? 'Paused (hover)' : isFetching ? 'Refreshing…' : `Updated ${ageSec}s ago`;
  return <span className={cn('text-xs', tone)}>{label}</span>;
}
