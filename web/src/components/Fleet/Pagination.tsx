interface Props {
  count: number;
  hasMore: boolean;
  isFetching: boolean;
  isPending: boolean;
  error: Error | null;
  cursorRepeated: boolean;
  loadMore: () => unknown;
  refetch: () => unknown;
  localFilter?: boolean;
}

export function Pagination({
  count,
  hasMore,
  isFetching,
  isPending,
  error,
  cursorRepeated,
  loadMore,
  refetch,
  localFilter,
}: Props) {
  return (
    <div className="flex flex-wrap items-center gap-3 border-t border-border-subtle px-3 py-3 text-xs text-text-muted">
      <p aria-live="polite">
        {isPending
          ? 'Loading…'
          : `${count} loaded · ${hasMore || cursorRepeated || error ? 'partial results' : 'end of results'}`}
        {localFilter && ' · Search and status filters apply to loaded results only.'}
      </p>
      {error && (
        <p role="alert" className="text-sev-critical">
          Could not load results: {error.message}. Previously loaded results are retained.
        </p>
      )}
      {cursorRepeated && (
        <p role="alert">
          The server repeated a cursor. Refresh to retry; results may be incomplete.
        </p>
      )}
      {hasMore && (
        <button
          type="button"
          disabled={isFetching}
          onClick={() => loadMore()}
          className="rounded border border-border px-2 py-1 text-accent disabled:opacity-50"
        >
          {isFetching ? 'Loading…' : 'Load more'}
        </button>
      )}
      <button
        type="button"
        disabled={isFetching}
        onClick={() => refetch()}
        className="rounded border border-border px-2 py-1 disabled:opacity-50"
      >
        {error ? 'Retry' : 'Refresh'}
      </button>
    </div>
  );
}
