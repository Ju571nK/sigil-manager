import { formatDistanceToNowStrict } from 'date-fns';

/**
 * Bare relative age of an ISO timestamp, e.g. "3m", "2h". Returns "—" if the
 * timestamp can't be parsed. No suffix — callers append " ago" where wanted.
 */
export function relativeAge(ts: string): string {
  try {
    return formatDistanceToNowStrict(new Date(ts));
  } catch {
    return '—';
  }
}
