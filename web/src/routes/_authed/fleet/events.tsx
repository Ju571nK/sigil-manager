import { createFileRoute, useNavigate, useSearch } from '@tanstack/react-router';
import { EventsTable } from '@/components/Fleet/EventsTable';
import {
  DEFAULT_FLEET_EVENTS_FILTER,
  type FleetEventsFilter,
  useFleetEvents,
} from '@/hooks/useFleetEvents';

interface EventsSearch {
  kind?: string[];
  host?: string[];
  since?: string;
}

export const Route = createFileRoute('/_authed/fleet/events')({
  validateSearch: (raw: Record<string, unknown>): EventsSearch => {
    const out: EventsSearch = {};
    const arr = (v: unknown): string[] =>
      Array.isArray(v)
        ? v.filter((x): x is string => typeof x === 'string')
        : typeof v === 'string'
          ? v.split(',').filter(Boolean)
          : [];
    const kind = arr(raw.kind);
    const host = arr(raw.host);
    if (kind.length) out.kind = kind;
    if (host.length) out.host = host;
    if (typeof raw.since === 'string' && raw.since.length > 0) out.since = raw.since;
    return out;
  },
  component: EventsTab,
});

function EventsTab() {
  const search = useSearch({ from: '/_authed/fleet/events' });
  const navigate = useNavigate();

  const filter: FleetEventsFilter = {
    evidenceKinds: search.kind ?? DEFAULT_FLEET_EVENTS_FILTER.evidenceKinds,
    hostIDs: search.host ?? DEFAULT_FLEET_EVENTS_FILTER.hostIDs,
    since: search.since ?? DEFAULT_FLEET_EVENTS_FILTER.since,
  };
  const { rows, isPending, error } = useFleetEvents(filter);

  const setKind = (kind: string | null) =>
    navigate({
      to: '/fleet/events',
      search: { ...search, kind: kind ? [kind] : undefined },
      replace: true,
    });

  return (
    <div>
      <div className="mb-3 flex items-center gap-1.5">
        <FilterChip active={filter.evidenceKinds.length === 0} onClick={() => setKind(null)}>
          All kinds
        </FilterChip>
        <FilterChip
          active={filter.evidenceKinds.includes('ai_guard_risk_assessed')}
          onClick={() => setKind('ai_guard_risk_assessed')}
        >
          AI Guard
        </FilterChip>
      </div>
      <div className="overflow-hidden rounded-md border border-border bg-bg-surface">
        {error ? (
          <div className="px-4 py-6 text-sm text-sev-critical">
            Failed to load events: {error.message}
          </div>
        ) : (
          <EventsTable rows={rows} isPending={isPending} />
        )}
      </div>
    </div>
  );
}

function FilterChip({
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
      className={
        active
          ? 'rounded border border-accent/40 bg-accent/10 px-2 py-0.5 text-xs text-accent'
          : 'rounded border border-border px-2 py-0.5 text-xs text-text-muted hover:text-text-primary'
      }
    >
      {children}
    </button>
  );
}
