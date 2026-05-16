import { useEffect, useState } from 'react';
import { apiFetch } from './lib/api';

interface Health {
  status: string;
  version: string;
  timestamp: string;
}

export default function App() {
  const [health, setHealth] = useState<Health | null>(null);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    apiFetch<Health>('/api/health')
      .then(setHealth)
      .catch((e: Error) => setError(e.message));
  }, []);

  return (
    <main className="font-sans p-8 text-text-body bg-bg-page min-h-screen">
      <h1 className="text-2xl font-semibold text-text-primary">sigil-manager</h1>
      <p className="text-text-muted mt-1">Scaffolded. API probe:</p>
      {error && <pre className="mt-4 text-sev-critical">{error}</pre>}
      {health && (
        <pre className="mt-4 p-3 bg-bg-elevated border border-border rounded font-mono text-sm">
          {JSON.stringify(health, null, 2)}
        </pre>
      )}
      {!error && !health && <p className="mt-4 text-text-subtle">Loading…</p>}
    </main>
  );
}
