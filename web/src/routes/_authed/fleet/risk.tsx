import { createFileRoute, useNavigate, useSearch } from '@tanstack/react-router';
import { RiskTable } from '@/components/Fleet/RiskTable';
import { DEFAULT_RISK_FILTER, type RiskFilter, useFleetRisk } from '@/hooks/useFleetRisk';

interface RiskSearch {
  minBucket?: RiskFilter['minBucket'];
  tool?: string[];
}

const VALID_BUCKETS: RiskFilter['minBucket'][] = ['low', 'medium', 'high', 'critical'];

export const Route = createFileRoute('/_authed/fleet/risk')({
  validateSearch: (raw: Record<string, unknown>): RiskSearch => {
    const out: RiskSearch = {};
    if (typeof raw.minBucket === 'string' && (VALID_BUCKETS as string[]).includes(raw.minBucket)) {
      out.minBucket = raw.minBucket as RiskFilter['minBucket'];
    }
    const tool = Array.isArray(raw.tool)
      ? raw.tool.filter((t): t is string => typeof t === 'string')
      : typeof raw.tool === 'string'
        ? raw.tool.split(',').filter(Boolean)
        : [];
    if (tool.length) out.tool = tool;
    return out;
  },
  component: RiskTab,
});

function RiskTab() {
  const search = useSearch({ from: '/_authed/fleet/risk' });
  const navigate = useNavigate();

  const filter: RiskFilter = {
    minBucket: search.minBucket ?? DEFAULT_RISK_FILTER.minBucket,
    // `tool` is honored from the URL (deep-linkable) but has no chip UI yet —
    // useFleetRisk already plumbs it through; a tool picker lands in a later plan.
    tool: search.tool ?? DEFAULT_RISK_FILTER.tool,
  };
  const { rows, isPending, error } = useFleetRisk(filter);

  const setBucket = (minBucket: RiskFilter['minBucket']) =>
    navigate({ to: '/fleet/risk', search: { ...search, minBucket }, replace: true });

  return (
    <div>
      <div className="mb-3 flex items-center gap-1.5">
        {VALID_BUCKETS.map((b) => (
          <button
            key={b}
            type="button"
            onClick={() => setBucket(b)}
            aria-pressed={filter.minBucket === b}
            className={
              filter.minBucket === b
                ? 'rounded border border-accent/40 bg-accent/10 px-2 py-0.5 text-xs text-accent'
                : 'rounded border border-border px-2 py-0.5 text-xs text-text-muted hover:text-text-primary'
            }
          >
            {b}
          </button>
        ))}
      </div>
      <div className="overflow-hidden rounded-md border border-border bg-bg-surface">
        {error ? (
          <div className="px-4 py-6 text-sm text-sev-critical">
            Failed to load risk: {error.message}
          </div>
        ) : (
          <RiskTable rows={rows} isPending={isPending} />
        )}
      </div>
    </div>
  );
}
