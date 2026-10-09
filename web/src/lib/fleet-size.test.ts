import { describe, expect, it } from 'vitest';
import { activeHostSummary } from './fleet-size';

describe('activeHostSummary', () => {
  it('returns null when the server reports no fleet size', () => {
    expect(activeHostSummary(null)).toBeNull();
    expect(activeHostSummary(undefined)).toBeNull();
  });

  it('formats count and window', () => {
    expect(activeHostSummary({ active_host_count: 42, active_window_days: 7 })).toBe(
      '42 active hosts · last 7 days',
    );
  });

  it('uses singular forms for one host and one day', () => {
    expect(activeHostSummary({ active_host_count: 1, active_window_days: 1 })).toBe(
      '1 active host · last 1 day',
    );
  });

  it('reports zero hosts rather than hiding them', () => {
    expect(activeHostSummary({ active_host_count: 0, active_window_days: 7 })).toBe(
      '0 active hosts · last 7 days',
    );
  });
});
