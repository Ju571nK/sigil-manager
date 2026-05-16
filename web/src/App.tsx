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
    <main style={{ fontFamily: 'system-ui', padding: '2rem', color: '#fafafa', background: '#0a0a0c', minHeight: '100vh' }}>
      <h1>sigil-manager</h1>
      <p>Scaffolded. API probe:</p>
      {error && <pre style={{ color: '#ef4444' }}>{error}</pre>}
      {health && <pre>{JSON.stringify(health, null, 2)}</pre>}
      {!error && !health && <p>Loading…</p>}
    </main>
  );
}
