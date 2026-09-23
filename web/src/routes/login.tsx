import { useMutation, useQuery } from '@tanstack/react-query';
import { createFileRoute, redirect, useNavigate, useSearch } from '@tanstack/react-router';
import { type FormEvent, useState } from 'react';
import { authMethods, type LoginResponse, login, me } from '@/api/auth';
import { SessionExpiredError, UnauthorizedError } from '@/api/client';
import { Button } from '@/components/ui/button';
import { Input } from '@/components/ui/input';
import { Label } from '@/components/ui/label';

interface Search {
  redirect?: string;
  oidc_error?: boolean;
}

/**
 * `/login` is the only public route. If the visitor already has a valid
 * session we bounce them straight to `/alerts` (or the `?redirect=` they
 * carried in from the auth guard). The form submits to `/api/v1/auth/login`;
 * 401 surfaces inline as "username or password is wrong".
 */
export const Route = createFileRoute('/login')({
  validateSearch: (raw: Record<string, unknown>): Search => ({
    redirect:
      typeof raw.redirect === 'string' &&
      raw.redirect.startsWith('/') &&
      !raw.redirect.startsWith('//') &&
      !raw.redirect.includes('\\')
        ? raw.redirect
        : undefined,
    oidc_error:
      raw.oidc_error === '1' || raw.oidc_error === 1 || raw.oidc_error === true ? true : undefined,
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

  const methods = useQuery({ queryKey: ['auth', 'methods'], queryFn: authMethods, retry: false });

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
        {search.oidc_error && (
          <p role="alert" className="mb-4 text-sm text-sev-critical">
            Organization sign-in failed or access was not granted. Try again or contact your
            administrator.
          </p>
        )}
        {methods.data?.oidc && (
          <a
            href="/api/v1/auth/oidc/start"
            className="mb-4 block rounded border border-border px-3 py-2 text-center text-sm text-accent hover:bg-bg-elevated"
          >
            Sign in with your organization
          </a>
        )}
        {methods.error && (
          <p role="alert" className="mb-4 text-sm text-text-muted">
            Could not load organization sign-in options.{' '}
            <button
              type="button"
              onClick={() => methods.refetch()}
              className="text-accent underline"
            >
              Retry
            </button>
          </p>
        )}
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
          Use your local administrator credentials
          {methods.data?.oidc ? ' for recovery access.' : ' to sign in.'}
        </p>
      </div>
    </div>
  );
}
