import { type QueryKey, useInfiniteQuery } from '@tanstack/react-query';
import { useCallback, useMemo } from 'react';
import { FLEET_POLL_INTERVAL_MS } from './useFleetQuery';

interface Page {
  next_cursor: string | null;
}

/** Poll all loaded pages from the first cursor; first occurrence wins at page boundaries. */
export function usePagedFleet<T, P extends Page>(
  key: QueryKey,
  fetchPage: (cursor: string | undefined, signal: AbortSignal) => Promise<P>,
  select: (page: P) => T[] | null | undefined,
  identity: (row: T) => string,
  options: { enabled?: boolean; interval?: number | false } = {},
) {
  const q = useInfiniteQuery({
    queryKey: key,
    queryFn: ({ pageParam, signal }) => fetchPage(pageParam, signal),
    initialPageParam: undefined as string | undefined,
    getNextPageParam: (last, _pages, _lastParam, params) =>
      last.next_cursor && !params.includes(last.next_cursor) ? last.next_cursor : undefined,
    enabled: options.enabled,
    refetchInterval: options.interval ?? FLEET_POLL_INTERVAL_MS,
    refetchIntervalInBackground: false,
    retry: false,
  });
  const rows = useMemo(() => {
    const seen = new Set<string>();
    return (q.data?.pages ?? [])
      .flatMap((page) => select(page) ?? [])
      .filter((row) => {
        const id = identity(row);
        if (seen.has(id)) return false;
        seen.add(id);
        return true;
      });
  }, [q.data, select, identity]);
  const next = q.data?.pages.at(-1)?.next_cursor;
  const cursorRepeated = !!next && !!q.data?.pageParams.includes(next);
  const loadMore = useCallback(() => q.fetchNextPage({ cancelRefetch: false }), [q.fetchNextPage]);
  const refetch = useCallback(() => q.refetch({ cancelRefetch: false }), [q.refetch]);
  return {
    rows,
    isPending: q.isPending && !q.data,
    error: q.error,
    lastUpdatedAt: q.dataUpdatedAt,
    isFetching: q.isFetching,
    hasMore: q.hasNextPage,
    cursorRepeated,
    loadMore,
    refetch,
  };
}
