import { createFileRoute, useNavigate } from '@tanstack/react-router';
import { EventDetails } from '@/components/EventDetails';
import { EventsTable } from '@/components/Fleet/EventsTable';
import { Pagination } from '@/components/Fleet/Pagination';
import { useFleetEvents } from '@/hooks/useFleetEvents';

interface EventsSearch {
  kind?: string[];
  host?: string[];
  since?: string;
  until?: string;
  event?: string;
  tool?: string;
}
export const Route = createFileRoute('/_authed/fleet/events')({
  validateSearch: (raw: Record<string, unknown>): EventsSearch => {
    const out: EventsSearch = {};
    for (const key of ['kind', 'host'] as const) {
      const v = raw[key];
      const values = Array.isArray(v)
        ? v.filter((x): x is string => typeof x === 'string')
        : typeof v === 'string'
          ? v.split(',').filter(Boolean)
          : [];
      if (values.length) out[key] = values;
    }
    for (const key of ['since', 'until', 'event', 'tool'] as const) {
      if (typeof raw[key] === 'string' && raw[key].length > 0) out[key] = raw[key];
    }
    return out;
  },
  component: EventsTab,
});

const HOOK_KINDS = [
  'hook_invocation',
  'hook_decision',
  'hook_config_drift',
  'possible_hook_activity_silent',
];
function EventsTab() {
  const search = Route.useSearch();
  const navigate = useNavigate();
  const setSearch = (next: Partial<EventsSearch>) =>
    navigate({ to: '/fleet/events', search: { ...search, ...next }, replace: true });
  const events = useFleetEvents({
    evidenceKinds: search.kind ?? [],
    hostIDs: search.host ?? [],
    since: search.since ?? null,
    until: search.until,
    tool: search.tool,
  });
  return (
    <div>
      <div className="mb-3 flex flex-wrap items-center gap-2 text-xs">
        <button
          type="button"
          className="text-accent"
          onClick={() => setSearch({ kind: undefined })}
        >
          All kinds
        </button>
        <button
          type="button"
          className="text-accent"
          onClick={() => setSearch({ kind: ['ai_guard_risk_assessed'] })}
        >
          AI Guard
        </button>
        <button
          type="button"
          className="text-accent"
          onClick={() => setSearch({ kind: HOOK_KINDS })}
        >
          Hooks
        </button>
        <button
          type="button"
          className="text-accent"
          onClick={() => setSearch({ kind: ['ai_guard_toggle_drift'] })}
        >
          Toggle drift
        </button>
      </div>
      <form
        key={JSON.stringify([search.kind, search.host, search.since, search.until, search.tool])}
        className="mb-3 flex flex-wrap items-end gap-3 text-xs"
        onSubmit={(event) => {
          event.preventDefault();
          const data = new FormData(event.currentTarget);
          const value = (name: string) => String(data.get(name) ?? '').trim();
          setSearch({
            kind: value('kind')
              ? value('kind')
                  .split(',')
                  .map((x) => x.trim())
                  .filter(Boolean)
              : undefined,
            host: value('host')
              ? value('host')
                  .split(',')
                  .map((x) => x.trim())
                  .filter(Boolean)
              : undefined,
            since: value('since') || undefined,
            until: value('until') || undefined,
            tool: value('tool') || undefined,
          });
        }}
      >
        {(['kind', 'host', 'since', 'until', 'tool'] as const).map((name) => (
          <label key={name} className="flex flex-col gap-1 text-text-muted">
            {
              {
                kind: 'Evidence kinds',
                host: 'Host IDs',
                since: 'Since (RFC3339)',
                until: 'Until (RFC3339)',
                tool: 'Tool (loaded results)',
              }[name]
            }
            <input
              name={name}
              defaultValue={
                Array.isArray(search[name]) ? (search[name] as string[]).join(',') : search[name]
              }
              className="w-44 rounded border border-border bg-bg-surface p-1.5 text-text-primary"
            />
          </label>
        ))}
        <button className="rounded border border-border px-2 py-1.5" type="submit">
          Apply filters
        </button>
      </form>
      <div className="overflow-hidden rounded-md border border-border bg-bg-surface">
        <EventsTable
          rows={events.rows}
          isPending={events.isPending}
          onSelect={(event) => setSearch({ event: event.event_id })}
        />
        <Pagination {...events} count={events.rawCount} localFilter={!!search.tool} />
      </div>
      <EventDetails
        eventID={search.event ?? null}
        rows={events.rows}
        onClose={() => setSearch({ event: undefined })}
      />
    </div>
  );
}
