/** Loading placeholder shared by the fleet tables — `count` full-width pulse bars. */
export function SkeletonRows({ count = 5 }: { count?: number }) {
  return (
    <div aria-hidden className="space-y-2 px-3 py-3">
      {Array.from({ length: count }).map((_, i) => (
        // biome-ignore lint/suspicious/noArrayIndexKey: static skeleton placeholders
        <div key={i} className="h-3 w-full animate-pulse rounded bg-bg-elevated" />
      ))}
    </div>
  );
}
