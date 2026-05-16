import { describe, it, expect } from 'vitest';
import { render, screen } from '@testing-library/react';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';

import { Button } from '@/components/ui/button';

describe('Button', () => {
  it('renders children', () => {
    const qc = new QueryClient();
    render(
      <QueryClientProvider client={qc}>
        <Button>Click</Button>
      </QueryClientProvider>
    );
    expect(screen.getByRole('button', { name: 'Click' })).toBeInTheDocument();
  });
});
