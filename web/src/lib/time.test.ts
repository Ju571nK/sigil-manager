import { describe, expect, it } from 'vitest';
import { relativeAge } from './time';

describe('relativeAge', () => {
  it('renders a strict, suffix-less distance for a recent timestamp', () => {
    const recent = new Date(Date.now() - 3 * 60_000).toISOString();
    // Pins the *strict* contract: "3 minutes", not "about 3 minutes" or
    // "3 minutes ago" — guards against a swap to formatDistanceToNow.
    expect(relativeAge(recent)).toBe('3 minutes');
  });

  it('returns "—" for an unparseable timestamp', () => {
    expect(relativeAge('not-a-date')).toBe('—');
  });
});
