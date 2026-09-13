import '@testing-library/jest-dom/vitest';
import { cleanup, render, screen } from '@testing-library/react';
import { afterEach, describe, expect, it } from 'vitest';
import type { AiGuardControl } from '@/api/fleet';
import { AiGuardByTool } from '@/components/Host/AiGuardByTool';
import { ObservedControls } from './ObservedControls';

afterEach(cleanup);
const assessedAt = '2026-09-13T00:00:00Z';
const scope = { kind: 'project' as const, path: '/work/project' };
const controls: AiGuardControl[] = [
  { id: 'future.unknown', source_path: '/etc/policy', setting: 'restricted', value: false },
  {
    id: 'codex.configured.allowed_sandbox_modes',
    source_path: '/etc/codex/requirements.toml',
    setting: 'allowed_sandbox_modes',
    value: ['read-only'],
  },
  { id: 'future.object', source_path: '/etc/policy', setting: 'future', value: { nested: 0 } },
];

describe('observed security settings', () => {
  it.each([undefined, null])('treats %s as unreported, never disabled', (value) => {
    render(<ObservedControls controls={value} assessedAt={assessedAt} scope={scope} />);
    expect(screen.getByText('Not reported for this assessment.')).toBeInTheDocument();
    expect(screen.queryByText(/Inspected;/)).not.toBeInTheDocument();
  });
  it('distinguishes an inspected empty list', () => {
    render(<ObservedControls controls={[]} assessedAt={assessedAt} scope={scope} />);
    expect(screen.getByText('Inspected; no active settings observed.')).toBeInTheDocument();
    expect(screen.queryByText(/Not reported/)).not.toBeInTheDocument();
  });
  it('preserves false, arrays, objects, unknown IDs, source and assessment context', () => {
    render(<ObservedControls controls={controls} assessedAt={assessedAt} scope={scope} />);
    for (const text of [
      'false',
      '["read-only"]',
      '{"nested":0}',
      'future.unknown',
      '/etc/codex/requirements.toml',
      assessedAt,
    ]) {
      expect(screen.getByText(text)).toBeInTheDocument();
    }
    expect(screen.getByText(/project/)).toBeInTheDocument();
    expect(screen.getByText(/not proof of runtime enforcement/)).toBeInTheDocument();
  });
  it('retains controls for low-risk tools without changing the score', () => {
    render(
      <AiGuardByTool
        byTool={{
          codex: {
            score: 0,
            bucket: 'low',
            assessed_ts: assessedAt,
            scope,
            reasons: [],
            is_reattestation: false,
            controls,
          },
        }}
      />,
    );
    expect(screen.getByText('false')).toBeInTheDocument();
    expect(screen.getByText('low 0.00')).toBeInTheDocument();
    expect(screen.getByText('false').closest('details')).not.toBeNull();
  });
});
