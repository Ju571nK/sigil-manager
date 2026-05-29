import { useMutation } from '@tanstack/react-query';
import { createFileRoute, redirect, useNavigate, useSearch } from '@tanstack/react-router';
import { type FormEvent, useState } from 'react';
import { type LoginResponse, login, me } from '@/api/auth';
import { SessionExpiredError, UnauthorizedError } from '@/api/client';
import { Button } from '@/components/ui/button';
import { Input } from '@/components/ui/input';
import { Label } from '@/components/ui/label';

interface Search {
  redirect?: string;
}

/**
 * `/login` is the only public route. If the visitor already has a valid
 * session we bounce them straight to `/alerts` (or the `?redirect=` they
 * carried in from the auth guard). The form submits to `/api/v1/auth/login`;
 * 401 surfaces inline as "username or password is wrong".
 */
export const Route = createFileRoute('/login')({
  validateSearch: (raw: Record<string, unknown>): Search => ({
    redirect: typeof raw.redirect === 'string' ? raw.redirect : undefined,
  }),
  beforeLoad: async ({ search }) => {
    try {
      await me();
      // Already authed — go where they were headed (or /alerts).
      throw redirect({ to: search.redirect ?? '/alerts' });
    } catch (err) {
      // me() throwing means not authed (no cookie OR an expired one); show the form.
      if (err instanceof UnauthorizedError || err instanceof SessionExpiredError) return;
      // A `redirect` is itself thrown — let TanStack Router handle it.
      throw err;
    }
  },
  component: LoginPage,
});

function LoginPage() {
  const navigate = useNavigate();
  const search = useSearch({ from: '/login' });

  const [username, setUsername] = useState('');
  const [password, setPassword] = useState('');

  const loginMutation = useMutation<LoginResponse, Error, { username: string; password: string }>({
    mutationFn: ({ username, password }) => login(username, password),
    onSuccess: async () => {
      await navigate({ to: search.redirect ?? '/alerts' });
    },
  });

  const onSubmit = (e: FormEvent) => {
    e.preventDefault();
    loginMutation.mutate({ username, password });
  };

  const errorMessage =
    loginMutation.error instanceof UnauthorizedError
      ? 'Username or password is wrong'
      : loginMutation.error?.message;

  return (
    <div className="min-h-screen bg-bg-page text-text-body flex items-center justify-center px-4">
      <div className="w-full max-w-sm rounded-md border border-border bg-bg-surface p-6 shadow-lg">
        <div className="mb-6 flex items-center gap-2">
          <span className="text-accent text-lg">◆</span>
          <span className="font-semibold text-text-primary">sigil-manager</span>
        </div>
        <form onSubmit={onSubmit} className="space-y-4">
          <div className="space-y-1.5">
            <Label htmlFor="username">Username</Label>
            <Input
              id="username"
              autoComplete="username"
              autoFocus
              value={username}
              onChange={(e) => setUsername(e.target.value)}
              required
            />
          </div>
          <div className="space-y-1.5">
            <Label htmlFor="password">Password</Label>
            <Input
              id="password"
              type="password"
              autoComplete="current-password"
              value={password}
              onChange={(e) => setPassword(e.target.value)}
              required
            />
          </div>
          {errorMessage && (
            <p className="text-sm text-sev-critical" role="alert">
              {errorMessage}
            </p>
          )}
          <Button type="submit" className="w-full" disabled={loginMutation.isPending}>
            {loginMutation.isPending ? 'Signing in…' : 'Sign in'}
          </Button>
        </form>
        <p className="mt-4 text-xs text-text-subtle">
          Single-admin console (UI/UX §9). Configure credentials via
          <code className="mx-1 text-text-muted">ADMIN_USERNAME</code> +
          <code className="mx-1 text-text-muted">ADMIN_PASSWORD_BCRYPT</code>.
        </p>
      </div>
    </div>
  );
}
