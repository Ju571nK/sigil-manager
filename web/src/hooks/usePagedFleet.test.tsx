import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { act, cleanup, renderHook, waitFor } from '@testing-library/react';
import type { ReactNode } from 'react';
import { afterEach, describe, expect, it, vi } from 'vitest';
import { usePagedFleet } from './usePagedFleet';

afterEach(cleanup);
type Row = { id: number; value: string };
type Page = { rows: Row[]; next_cursor: string | null };
const select = (p: Page) => p.rows;
const identity = (r: Row) => String(r.id);
function setup() {
  const client = new QueryClient({ defaultOptions: { queries: { retry: false, gcTime: 0 } } });
  return ({ children }: { children: ReactNode }) => (
    <QueryClientProvider client={client}>{children}</QueryClientProvider>
  );
}

describe('cursor pagination', () => {
  it('loads beyond 100, deduplicates boundaries, and rebuilds cursors on refresh', async () => {
    let refreshed = false;
    const first = Array.from({ length: 100 }, (_, id) => ({ id, value: 'original' }));
    const fetch = vi.fn(async (cursor: string | undefined): Promise<Page> => {
      if (!cursor)
        return {
          rows: refreshed ? [{ id: -1, value: 'new' }, ...first.slice(0, 99)] : first,
          next_cursor: refreshed ? 'after98' : 'after99',
        };
      return {
        rows: refreshed
          ? [
              { id: 99, value: 'updated' },
              { id: 100, value: 'last' },
            ]
          : [
              { id: 99, value: 'duplicate' },
              { id: 100, value: 'last' },
            ],
        next_cursor: null,
      };
    });
    const { result } = renderHook(
      () => usePagedFleet(['test'], fetch, select, identity, { interval: false }),
      { wrapper: setup() },
    );
    await waitFor(() => expect(result.current.rows).toHaveLength(100));
    await act(async () => {
      await result.current.loadMore();
    });
    await waitFor(() => expect(result.current.rows).toHaveLength(101));
    expect(result.current.rows[99].value).toBe('original');
    refreshed = true;
    await act(async () => {
      await result.current.refetch();
    });
    await waitFor(() => expect(result.current.rows).toHaveLength(102));
    expect(fetch.mock.calls.at(-1)?.[0]).toBe('after98');
    expect(result.current.rows.find((r) => r.id === 99)?.value).toBe('updated');
  });
  it('retains loaded rows after a failed page and retries that cursor', async () => {
    let fail = true;
    const fetch = async (cursor: string | undefined): Promise<Page> => {
      if (cursor && fail) throw new Error('offline');
      return cursor
        ? { rows: [{ id: 2, value: 'last' }], next_cursor: null }
        : { rows: [{ id: 1, value: 'first' }], next_cursor: 'next' };
    };
    const { result } = renderHook(
      () => usePagedFleet(['error'], fetch, select, identity, { interval: false }),
      { wrapper: setup() },
    );
    await waitFor(() => expect(result.current.rows).toHaveLength(1));
    await act(async () => {
      await result.current.loadMore();
    });
    expect(result.current.rows).toHaveLength(1);
    await waitFor(() => expect(result.current.error?.message).toBe('offline'));
    fail = false;
    await act(async () => {
      await result.current.loadMore();
    });
    await waitFor(() => expect(result.current.rows).toHaveLength(2));
    expect(result.current.error).toBeNull();
  });
  it('resets to the first page for a different filter and stops repeated cursors', async () => {
    const fetch = vi.fn(
      async (cursor: string | undefined): Promise<Page> => ({
        rows: [{ id: cursor ? 2 : 1, value: 'row' }],
        next_cursor: 'same',
      }),
    );
    const { result, rerender } = renderHook(
      ({ filter }) =>
        usePagedFleet(['filter', filter], fetch, select, identity, { interval: false }),
      { initialProps: { filter: 'a' }, wrapper: setup() },
    );
    await waitFor(() => expect(result.current.rows).toHaveLength(1));
    await act(async () => {
      await result.current.loadMore();
    });
    await waitFor(() => expect(result.current.cursorRepeated).toBe(true));
    expect(result.current.hasMore).toBe(false);
    rerender({ filter: 'b' });
    await waitFor(() => expect(result.current.rows).toHaveLength(1));
    expect(fetch.mock.calls.at(-1)?.[0]).toBeUndefined();
  });
});
