import { useQuery } from '@tanstack/react-query';
import { createFileRoute } from '@tanstack/react-router';
import { Button } from '@/components/ui/button';
import { apiFetch } from '@/lib/api';

interface Health {
  status: string;
  version: string;
  timestamp: string;
}

export const Route = createFileRoute('/')({
  component: IndexPage,
});

function IndexPage() {
  const { data, error, isFetching, refetch } = useQuery({
    queryKey: ['health'],
    queryFn: () => apiFetch<Health>('/api/health'),
  });

  return (
    <main className="font-sans p-8">
      <h1 className="text-2xl font-semibold text-text-primary">sigil-manager</h1>
      <p className="text-text-muted mt-1">Scaffolded. API probe:</p>
      <div className="mt-4 flex gap-2">
        <Button onClick={() => refetch()} disabled={isFetching}>
          {isFetching ? 'Probing…' : 'Probe again'}
        </Button>
      </div>
      {error && <pre className="mt-4 text-sev-critical">{(error as Error).message}</pre>}
      {data && (
        <pre className="mt-4 p-3 bg-bg-elevated border border-border rounded font-mono text-sm">
          {JSON.stringify(data, null, 2)}
        </pre>
      )}
    </main>
  );
}
