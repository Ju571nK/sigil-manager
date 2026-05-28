import type { ToolAiGuard } from '@/api/fleet';
import { ReasonList } from '@/components/ReasonList';
import { humanTool, scopeLabel } from '@/lib/labels';
import { cn } from '@/lib/utils';

interface Props {
  byTool: Record<string, ToolAiGuard>;
}

/**
 * AI Guard per-tool risk (UI/UX §5.3). Risky tools (bucket > low) get a card
 * with their reasons inline; quiet (low/0) tools collapse into one strip so
 * the signal isn't drowned out — most tools are quiet in real fleets.
 */
export function AiGuardByTool({ byTool }: Props) {
  const tools = Object.entries(byTool);
  if (tools.length === 0) {
    return (
      <Section>
        <p className="px-1 py-3 text-sm text-text-muted">No AI Guard assessments yet.</p>
      </Section>
    );
  }
  const risky = tools.filter(([, t]) => t.bucket !== 'low').sort((a, b) => b[1].score - a[1].score);
  const quiet = tools.filter(([, t]) => t.bucket === 'low');

  return (
    <Section>
      {risky.length === 0 && (
        <p className="px-1 pb-2 text-sm text-text-muted">No tools above the low bucket.</p>
      )}
      <div className="space-y-2">
        {risky.map(([tool, t]) => (
          <ToolCard key={tool} tool={tool} t={t} />
        ))}
      </div>
      {quiet.length > 0 && (
        <details className="mt-3 text-xs text-text-muted">
          <summary className="cursor-pointer select-none">
            Low ({quiet.length}): {quiet.map(([tool]) => humanTool(tool)).join(' · ')}
          </summary>
          <div className="mt-1 space-y-0.5 pl-3">
            {quiet.map(([tool, t]) => (
              <div key={tool}>
                {humanTool(tool)} — {t.score.toFixed(1)}
              </div>
            ))}
          </div>
        </details>
      )}
    </Section>
  );
}

function Section({ children }: { children: React.ReactNode }) {
  return (
    <section className="mb-4">
      <h2 className="mb-2 text-xs font-medium uppercase tracking-wide text-text-subtle">
        AI Guard risk
      </h2>
      {children}
    </section>
  );
}

function ToolCard({ tool, t }: { tool: string; t: ToolAiGuard }) {
  return (
    <div
      className={cn('rounded-md border border-border bg-bg-surface p-3', bucketBorder(t.bucket))}
    >
      <div className="flex flex-wrap items-center gap-2 text-xs">
        <span className="font-medium text-text-primary">{humanTool(tool)}</span>
        <span className={cn('uppercase tracking-wide font-medium', bucketText(t.bucket))}>
          {t.bucket} {t.score.toFixed(2)}
        </span>
        <span className="text-text-subtle">{scopeLabel(t.scope)}</span>
        {t.is_reattestation && <span className="text-text-subtle">· re-attested</span>}
      </div>
      {t.reasons && t.reasons.length > 0 && (
        <div className="mt-2 text-xs">
          <ReasonList reasons={t.reasons} />
        </div>
      )}
    </div>
  );
}

function bucketText(bucket: string): string {
  switch (bucket) {
    case 'critical':
      return 'text-sev-critical';
    case 'high':
      return 'text-sev-high';
    case 'medium':
      return 'text-sev-medium';
    case 'low':
      return 'text-sev-low';
    default:
      return 'text-sev-info';
  }
}

function bucketBorder(bucket: string): string {
  switch (bucket) {
    case 'critical':
      return 'border-l-2 border-l-sev-critical';
    case 'high':
      return 'border-l-2 border-l-sev-high';
    case 'medium':
      return 'border-l-2 border-l-sev-medium';
    default:
      return '';
  }
}
