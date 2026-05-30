import { describe, expect, it } from 'vitest';
import { bucketTextColor } from './severity';

describe('bucketTextColor', () => {
  it('maps each known bucket to its sev text token', () => {
    expect(bucketTextColor('critical')).toBe('text-sev-critical');
    expect(bucketTextColor('high')).toBe('text-sev-high');
    expect(bucketTextColor('medium')).toBe('text-sev-medium');
    expect(bucketTextColor('low')).toBe('text-sev-low');
  });

  it('falls back to the info token for unknown/future buckets', () => {
    expect(bucketTextColor('info')).toBe('text-sev-info');
    expect(bucketTextColor('whatever')).toBe('text-sev-info');
    expect(bucketTextColor('')).toBe('text-sev-info');
  });

  it('derives a background token via the documented replace idiom', () => {
    expect(bucketTextColor('critical').replace('text-', 'bg-')).toBe('bg-sev-critical');
  });
});
