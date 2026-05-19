import { defineConfig, devices } from '@playwright/test';

/**
 * Playwright runs e2e specs against the **production Go binary** with the
 * embedded SPA, not the Vite dev server. That way we exercise the same
 * routing + cookie semantics + chi middleware that ship in Docker.
 *
 * Before running, the harness expects:
 *   - the binary built at `../sigil-manager` (Makefile `make e2e` handles this),
 *   - `MOCK_FLEET=1` so no live sigil-server is needed,
 *   - the env vars below (which match `.env.example`).
 *
 * Tests share one server instance; we put the per-test triage DB under a
 * temp dir so each `npx playwright test` run starts clean.
 */

const PORT = process.env.PORT ?? '18181';
const BASE_URL = `http://localhost:${PORT}`;

// Bcrypt hash of "test-password" at cost 10.
const TEST_ADMIN_HASH = '$2a$10$wfv94g2jgl.5WmoEZHXfw.WMQSDL6HzwEq0avcAT1QmtrqBWUf7wu';

export default defineConfig({
  testDir: './tests/e2e',
  fullyParallel: false, // single backend instance = serial tests
  workers: 1,
  forbidOnly: !!process.env.CI,
  retries: process.env.CI ? 1 : 0,
  reporter: process.env.CI ? [['github'], ['list']] : 'list',

  use: {
    baseURL: BASE_URL,
    trace: 'on-first-retry',
    screenshot: 'only-on-failure',
    actionTimeout: 5_000,
    navigationTimeout: 10_000,
  },

  webServer: {
    command: '../sigil-manager',
    url: `${BASE_URL}/api/health`,
    reuseExistingServer: !process.env.CI,
    timeout: 30_000,
    env: {
      LISTEN_ADDR: `:${PORT}`,
      MOCK_FLEET: '1',
      TRIAGE_DB_PATH: '/tmp/sigil-manager-e2e-triage.sqlite',
      ADMIN_USERNAME: 'admin',
      ADMIN_PASSWORD_BCRYPT: TEST_ADMIN_HASH,
      JWT_SECRET: '0123456789abcdefghijklmnopqrstuv',
      SIGIL_INSECURE_COOKIE: '1',
    },
  },

  projects: [
    {
      name: 'chromium',
      use: { ...devices['Desktop Chrome'] },
    },
  ],
});
