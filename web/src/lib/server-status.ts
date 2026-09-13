import { ApiError, ServiceUnavailableError } from '@/api/client';

export function readAPIError(error: unknown): string {
  if (error instanceof ServiceUnavailableError) return 'Rebuilding index (503). Retry shortly.';
  if (error instanceof ApiError) {
    if (error.code === 'upstream_unauthorized')
      return 'Read API token rejected. Check the server read token configuration.';
    if (error.code === 'read_api_disabled')
      return 'Read API disabled. Configure SIGIL_SERVER_READ_TOKEN on sigil-server.';
    if (error.status === 401) return 'Session unavailable. Sign in again.';
  }
  return 'Read API unavailable. Check the connection and retry.';
}
