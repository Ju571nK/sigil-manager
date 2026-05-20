import { expect, test } from '@playwright/test';

const ADMIN = 'admin';
const PASSWORD = 'test-password';

test.describe('login flow', () => {
  test.beforeEach(async ({ context }) => {
    // Start each test with no cookies so we always exercise the
    // unauthed → /login redirect.
    await context.clearCookies();
  });

  test('unauthed visit to / lands on /login', async ({ page }) => {
    await page.goto('/');
    await expect(page).toHaveURL(/\/login$/);
    // The login card shows the brand label + Sign in button.
    await expect(page.getByText('sigil-manager').first()).toBeVisible();
    await expect(page.getByRole('button', { name: /^Sign in$/ })).toBeVisible();
  });

  test('valid credentials redirect to /alerts and persist across reload', async ({ page }) => {
    await page.goto('/login');
    await page.getByLabel('Username').fill(ADMIN);
    await page.getByLabel('Password').fill(PASSWORD);
    await page.getByRole('button', { name: /^Sign in$/ }).click();

    await expect(page).toHaveURL(/\/alerts/);
    await expect(page.getByRole('heading', { name: /^Alerts$/ })).toBeVisible();

    // Cookie persistence: reload and we should still be on /alerts.
    await page.reload();
    await expect(page).toHaveURL(/\/alerts/);
  });

  test('wrong password surfaces inline error and keeps user on /login', async ({ page }) => {
    await page.goto('/login');
    await page.getByLabel('Username').fill(ADMIN);
    await page.getByLabel('Password').fill('definitely-wrong');
    await page.getByRole('button', { name: /^Sign in$/ }).click();

    await expect(page.getByRole('alert')).toContainText(/wrong/i);
    await expect(page).toHaveURL(/\/login/);
  });

  test('logout bounces back to /login and the session cookie clears', async ({ page, context }) => {
    // Log in first.
    await page.goto('/login');
    await page.getByLabel('Username').fill(ADMIN);
    await page.getByLabel('Password').fill(PASSWORD);
    await page.getByRole('button', { name: /^Sign in$/ }).click();
    await expect(page).toHaveURL(/\/alerts/);

    // Click the logout icon (Log out aria-label).
    await page.getByRole('button', { name: /log out/i }).click();
    await expect(page).toHaveURL(/\/login/);

    const cookies = await context.cookies();
    const session = cookies.find((c) => c.name === 'sigil_session');
    // Either the cookie is gone, or it's empty / negative max-age — all
    // valid post-logout states depending on browser eviction timing.
    expect(session?.value ?? '').toBe('');
  });
});
