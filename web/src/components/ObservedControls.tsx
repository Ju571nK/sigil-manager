import type { AiGuardControl, Scope } from '@/api/fleet';
import { scopeLabel } from '@/lib/labels';

interface Props {
  controls?: AiGuardControl[] | null;
  assessedAt: string;
  scope: Scope;
}

/** Shared by host snapshots and historical event details. */
export function ObservedControls({ controls, assessedAt, scope }: Props) {
  return (
    <section aria-label="Observed security settings" className="mt-3 min-w-0 text-xs">
      <h3 className="font-medium text-text-primary">Observed security settings</h3>
      <p className="mt-1 text-text-muted">
        Configuration observations, not proof of runtime enforcement. Risk score is unchanged.
      </p>
      <p className="mt-1 break-all text-text-muted">
        Assessment: <time dateTime={assessedAt}>{assessedAt}</time> · {scopeLabel(scope)}
      </p>
      {controls == null ? (
        <p className="mt-2 text-text-muted">Not reported for this assessment.</p>
      ) : controls.length === 0 ? (
        <p className="mt-2 text-text-muted">Inspected; no active settings observed.</p>
      ) : (
        <ul className="mt-2 space-y-2">
          {controls.map((control) => (
            <li
              key={JSON.stringify(control)}
              className="min-w-0 rounded border border-border-subtle p-2"
            >
              <p className="break-all font-mono text-text-primary">{control.setting}</p>
              <dl className="mt-1 space-y-1 text-text-muted">
                <div>
                  <dt className="inline">Value: </dt>
                  <dd className="inline whitespace-pre-wrap break-all font-mono">
                    {JSON.stringify(control.value)}
                  </dd>
                </div>
                <div>
                  <dt className="inline">Source: </dt>
                  <dd className="inline break-all font-mono">{control.source_path}</dd>
                </div>
                <div>
                  <dt className="inline">ID: </dt>
                  <dd className="inline break-all font-mono">{control.id}</dd>
                </div>
              </dl>
            </li>
          ))}
        </ul>
      )}
    </section>
  );
}
