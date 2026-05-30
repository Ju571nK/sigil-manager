import { createFileRoute, useNavigate, useSearch } from '@tanstack/react-router';
import { useCallback, useEffect, useMemo, useRef, useState } from 'react';
import type { EventWithTriage } from '@/api/fleet';
import { FilterRow, SEARCH_INPUT_ID } from '@/components/AlertsQueue/FilterRow';
import { QueueTable, type SortMode } from '@/components/AlertsQueue/QueueTable';
import { ShortcutsCheatsheet } from '@/components/AlertsQueue/ShortcutsCheatsheet';
import { SlideOver } from '@/components/AlertsQueue/SlideOver';
import { type AlertFilter, DEFAULT_FILTER, useAlerts } from '@/hooks/useAlerts';
import { useShortcuts } from '@/hooks/useShortcuts';
import { cn } from '@/lib/utils';

/**
 * `/alerts` is Plan 02's landing page (UI/UX D2). Filter + selected
 * event live in the URL so links + reload preserve view. The slide-over
 * mounts when `?alert=:event_id` is present (UI/UX §7.4).
 */
export interface AlertsSearch {
  minBucket?: AlertFilter['minBucket'];
  triageStatuses?: AlertFilter['triageStatuses'];
  since?: string | null;
  query?: string;
  /** Selected alert id; presence opens the slide-over (UI/UX §7.4). */
  alert?: string;
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
    if (typeof raw.alert === 'string' && raw.alert.length > 0) out.alert = raw.alert;
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

  // Memoize so the filter object keeps a stable identity across renders (search
  // params are structurally shared by the router, so these deps only change on a
  // real URL change). Downstream useMemo(applyClientFilter) + the selected/visible
  // memos depend on this object, so an unstable ref would recompute every render.
  const filter: AlertFilter = useMemo(
    () => ({
      minBucket: search.minBucket ?? DEFAULT_FILTER.minBucket,
      triageStatuses: search.triageStatuses ?? DEFAULT_FILTER.triageStatuses,
      since: search.since ?? DEFAULT_FILTER.since,
      query: search.query ?? DEFAULT_FILTER.query,
    }),
    [search.minBucket, search.triageStatuses, search.since, search.query],
  );

  const sort: SortMode = search.sort ?? 'severity_desc';
  const selectedAlertID = search.alert ?? null;

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

  const selectedEvent = useMemo(
    () => rows.find((ev) => ev.event_id === selectedAlertID) ?? null,
    [rows, selectedAlertID],
  );

  const setSearch = useCallback(
    (next: Partial<AlertsSearch>) => {
      navigate({
        to: '/alerts',
        search: { ...search, ...next },
        replace: true,
      });
    },
    [navigate, search],
  );

  const onFilterChange = (next: Partial<AlertFilter>) => setSearch(next);
  const onSortChange = (next: SortMode) => setSearch({ sort: next });
  const onSelect = (event: EventWithTriage) => setSearch({ alert: event.event_id });
  const closeSlideOver = () => setSearch({ alert: undefined });

  // -----------------------------------------------------------------------
  // Keyboard shortcuts (UI/UX §7.1)
  // -----------------------------------------------------------------------
  const focusAssign = useRef<() => void>(() => undefined);
  const focusNote = useRef<() => void>(() => undefined);
  const registerFocusAssign = useCallback((fn: () => void) => {
    focusAssign.current = fn;
  }, []);
  const registerFocusNote = useCallback((fn: () => void) => {
    focusNote.current = fn;
  }, []);

  const [cheatsheetOpen, setCheatsheetOpen] = useState(false);

  // Sorted rows the user actually sees — must match QueueTable's order so
  // j/k arrow navigation lines up with what's on screen.
  const visibleRows = useMemo(() => sortRows(rows, sort), [rows, sort]);
  const cursorIndex = useMemo(
    () => visibleRows.findIndex((ev) => ev.event_id === selectedAlertID),
    [visibleRows, selectedAlertID],
  );

  const moveCursor = useCallback(
    (delta: number) => {
      if (visibleRows.length === 0) return;
      const next =
        cursorIndex < 0
          ? delta > 0
            ? 0
            : visibleRows.length - 1
          : Math.min(Math.max(0, cursorIndex + delta), visibleRows.length - 1);
      setSearch({ alert: visibleRows[next].event_id });
    },
    [cursorIndex, visibleRows, setSearch],
  );

  useShortcuts({
    j: () => moveCursor(1),
    ArrowDown: () => moveCursor(1),
    k: () => moveCursor(-1),
    ArrowUp: () => moveCursor(-1),
    Escape: () => {
      if (cheatsheetOpen) setCheatsheetOpen(false);
      else if (selectedAlertID) closeSlideOver();
    },
    Enter: () => {
      if (cursorIndex < 0 && visibleRows.length > 0) {
        setSearch({ alert: visibleRows[0].event_id });
      }
    },
    a: () => focusAssign.current(),
    n: () => focusNote.current(),
    c: () => triggerStatusButton('Acknowledge'),
    r: () => triggerStatusButton('Resolve'),
    i: () => triggerStatusButton('Investigating'),
    '/': () => {
      const el = document.getElementById(SEARCH_INPUT_ID) as HTMLInputElement | null;
      el?.focus();
      el?.select();
    },
    '?': () => setCheatsheetOpen(true),
    'g a': () => navigate({ to: '/alerts' }),
    // Plans 03/05 — leave bound so users hitting the shortcut hear a
    // soft "no-op" instead of unrelated text input.
    'g f': () => undefined,
    'g s': () => undefined,
  });

  // Sync the cheatsheet's onOpenChange close path to ESC handler.
  useEffect(() => {
    if (!cheatsheetOpen) return;
    return () => undefined;
  }, [cheatsheetOpen]);

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
        <div className="flex items-center gap-3">
          <button
            type="button"
            onClick={() => setCheatsheetOpen(true)}
            className="text-xs text-text-subtle hover:text-text-primary"
            title="Show keyboard shortcuts (?)"
          >
            <kbd className="rounded border border-border bg-bg-elevated px-1.5 py-px font-mono text-[10px]">
              ?
            </kbd>{' '}
            shortcuts
          </button>
          <FreshnessIndicator
            lastUpdatedAt={lastUpdatedAt}
            isFetching={isFetching}
            isPaused={isPaused}
          />
        </div>
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
            selectedEventID={selectedAlertID}
            onSelect={onSelect}
            onRowHoverEnter={onRowHoverEnter}
            onRowHoverLeave={onRowHoverLeave}
            sort={sort}
            onSortChange={onSortChange}
            isPending={isPending}
            filtersActive={isFilterActive(filter)}
            onResetFilters={() =>
              setSearch({
                minBucket: undefined,
                triageStatuses: undefined,
                since: undefined,
                query: undefined,
              })
            }
          />
        )}
      </div>

      <SlideOver
        event={selectedEvent}
        onClose={closeSlideOver}
        registerFocusAssign={registerFocusAssign}
        registerFocusNote={registerFocusNote}
      />

      <ShortcutsCheatsheet open={cheatsheetOpen} onOpenChange={setCheatsheetOpen} />
    </div>
  );
}

/**
 * Fires a click on a Button inside the slide-over by matching its visible
 * label. We do this via document.querySelector rather than wiring another
 * registerFocus* callback per action — there are only 3 statuses + reopen,
 * and they're stable strings.
 */
function triggerStatusButton(label: 'Acknowledge' | 'Resolve' | 'Investigating') {
  const buttons = document.querySelectorAll<HTMLButtonElement>('button');
  for (const b of buttons) {
    if (b.textContent?.trim() === label && !b.disabled) {
      b.click();
      return;
    }
  }
}

/** True when any filter differs from defaults. Drives "Reset filters" copy. */
function isFilterActive(f: AlertFilter): boolean {
  return (
    f.minBucket !== DEFAULT_FILTER.minBucket ||
    f.triageStatuses.length !== DEFAULT_FILTER.triageStatuses.length ||
    f.since !== DEFAULT_FILTER.since ||
    f.query !== DEFAULT_FILTER.query
  );
}

/**
 * Mirror of [`QueueTable`]'s sorting so we can compute j/k cursor moves
 * from the parent route. Kept in sync manually — when QueueTable's
 * algorithm changes, this must too.
 */
const BUCKET_RANK: Record<string, number> = {
  critical: 4,
  high: 3,
  medium: 2,
  low: 1,
  info: 0,
};

function sortRows(rows: EventWithTriage[], mode: SortMode): EventWithTriage[] {
  const copy = [...rows];
  const tsDesc = (a: string, b: string) => (a < b ? 1 : a > b ? -1 : 0);
  const bucketRank = (ev: EventWithTriage): number => {
    if (ev.evidence?.kind === 'ai_guard_risk_assessed') {
      const bucket = (ev.evidence as { bucket?: string }).bucket;
      return bucket ? (BUCKET_RANK[bucket] ?? 0) : 0;
    }
    return ev.severity === 'warn' ? 2 : 0;
  };
  switch (mode) {
    case 'severity_desc':
      copy.sort((a, b) => {
        const sa = bucketRank(a);
        const sb = bucketRank(b);
        if (sa !== sb) return sb - sa;
        return tsDesc(a.ts, b.ts);
      });
      break;
    case 'age_desc':
      copy.sort((a, b) => tsDesc(a.ts, b.ts));
      break;
    case 'age_asc':
      copy.sort((a, b) => -tsDesc(a.ts, b.ts));
      break;
  }
  return copy;
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
  useEffect(() => {
    const id = window.setInterval(() => setNow(Date.now()), 1_000);
    return () => window.clearInterval(id);
  }, []);

  if (!lastUpdatedAt) {
    return <span className="text-xs text-text-subtle">Connecting…</span>;
  }
  const ageSec = Math.max(0, Math.floor((now - lastUpdatedAt) / 1000));
  const tone =
    ageSec < 30 ? 'text-text-muted' : ageSec < 60 ? 'text-status-degraded' : 'text-sev-critical';
  const label = isPaused ? 'Paused (hover)' : isFetching ? 'Refreshing…' : `Updated ${ageSec}s ago`;
  return <span className={cn('text-xs', tone)}>{label}</span>;
}
