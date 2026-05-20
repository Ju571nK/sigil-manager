import type { ReactNode } from 'react';

interface Props {
  children: ReactNode;
}

/**
 * PageShell is the inner container every authed page sits inside. Width
 * cap matches UI/UX §6.4 (1280 max) so dense tables stay readable on
 * wide monitors without a min-width.
 */
export function PageShell({ children }: Props) {
  return <div className="mx-auto w-full max-w-[1280px] px-6 py-4">{children}</div>;
}
