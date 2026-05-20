/** Wraps the `/api/v1/auth/*` handlers in [`./client`]. */

import { api } from './client';

export interface MeResponse {
  username: string;
  expires_at: string; // RFC3339 with 9-digit fractional seconds
}

export interface LoginResponse {
  username: string;
  expires_at: string;
}

/** POST /api/v1/auth/login. Throws [`UnauthorizedError`] on wrong creds. */
export function login(username: string, password: string): Promise<LoginResponse> {
  return api<LoginResponse>('/auth/login', {
    method: 'POST',
    body: JSON.stringify({ username, password }),
  });
}

/** POST /api/v1/auth/logout. Idempotent; never throws on no-cookie. */
export function logout(): Promise<{ status: string }> {
  return api<{ status: string }>('/auth/logout', { method: 'POST' });
}

/** GET /api/v1/auth/me. Throws on 401 → guard treats as redirect to /login. */
export function me(): Promise<MeResponse> {
  return api<MeResponse>('/auth/me');
}
