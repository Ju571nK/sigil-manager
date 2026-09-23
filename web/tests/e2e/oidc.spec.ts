import { expect, test } from '@playwright/test';

test('organization sign-in is optional and errors retain local recovery', async ({ page }, testInfo) => {
  await page.route('**/api/v1/auth/methods', route => route.fulfill({ json: { local: true, oidc: true } }));
  await page.route('**/api/v1/auth/oidc/start', route => route.fulfill({ status: 302, headers: { Location: '/login?oidc_error=1' } }));
  await page.goto('/login');
  await page.getByRole('link', { name: 'Sign in with your organization' }).click();
  await expect(page.getByRole('alert')).toContainText('Organization sign-in failed');
  await expect(page.getByLabel('Username')).toBeVisible();
  await page.screenshot({ path: testInfo.outputPath('organization-login.png') });
  await page.getByLabel('Username').fill('admin');
  await page.getByLabel('Password').fill('test-password');
  await page.getByRole('button', { name: /^Sign in$/ }).click();
  await expect(page).toHaveURL(/\/alerts/);
});

test('unconfigured provider leaves local login available', async ({ page }) => {
  await page.goto('/login');
  await expect(page.getByRole('link', { name: 'Sign in with your organization' })).toHaveCount(0);
  await expect(page.getByRole('button', { name: /^Sign in$/ })).toBeVisible();
});

test('failed method discovery can be retried', async ({ page }) => {
  let unavailable = true;
  await page.route('**/api/v1/auth/methods', route => unavailable
    ? route.fulfill({ status: 503, json: { error: { code: 'unavailable', message: 'Unavailable' } } })
    : route.fulfill({ json: { local: true, oidc: true } }));
  await page.goto('/login');
  await expect(page.getByRole('alert')).toContainText('Could not load organization sign-in options');
  unavailable = false;
  await page.getByRole('button', { name: 'Retry', exact: true }).click();
  await expect(page.getByRole('link', { name: 'Sign in with your organization' })).toBeVisible();
});
