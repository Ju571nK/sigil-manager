import { type Page, expect, test } from '@playwright/test';

const ADMIN = 'admin';
const PASSWORD = 'test-password';

async function login(page: Page) {
  await page.goto('/login');
  await page.getByLabel('Username').fill(ADMIN);
  await page.getByLabel('Password').fill(PASSWORD);
  await page.getByRole('button', { name: /^Sign in$/ }).click();
  await expect(page).toHaveURL(/\/alerts/);
}

test.describe('fleet pages', () => {
  test.beforeEach(async ({ context }) => {
    await context.clearCookies();
  });

  test('Fleet nav lands on /fleet/risk via redirect', async ({ page }) => {
    await login(page);
    await page.getByRole('link', { name: /^Fleet$/ }).click();
    await expect(page).toHaveURL(/\/fleet\/risk/);
    // Risk table paints rows from the mock. NOTE: hostnames are plain-text
    // cells today; Plan 04 wraps them in <a>. getByRole('cell') matches by
    // accessible name so it should survive, but re-check when that lands.
    await expect(page.getByRole('cell', { name: /alice|bob|carol|eve/ }).first()).toBeVisible({
      timeout: 5_000,
    });
  });

  test('bare /fleet redirects to /fleet/risk', async ({ page }) => {
    await login(page);
    await page.goto('/fleet');
    await expect(page).toHaveURL(/\/fleet\/risk/);
  });

  test('tab switching changes URL and content', async ({ page }) => {
    await login(page);
    await page.goto('/fleet/risk');

    await page.getByRole('link', { name: /^Compliance$/ }).click();
    await expect(page).toHaveURL(/\/fleet\/compliance/);
    // Compliance derives pills across the mock's mixed hosts.
    await expect(page.getByText(/^In sync$/i).first()).toBeVisible({ timeout: 5_000 });
    await expect(page.getByText(/^Expired$/i).first()).toBeVisible();
    await expect(page.getByText(/^Failing signature$/i).first()).toBeVisible();

    await page.getByRole('link', { name: /^Events$/ }).click();
    await expect(page).toHaveURL(/\/fleet\/events/);
    // Events timeline paints too (symmetry with the compliance assertions).
    await expect(page.locator('tbody tr').first()).toBeVisible({ timeout: 5_000 });
  });

  test('risk min_bucket chip narrows the row set', async ({ page }) => {
    await login(page);
    await page.goto('/fleet/risk');
    // Default min_bucket=low lists all 4 risk-bearing hosts (dave has no risk).
    await expect(page.locator('tbody tr')).toHaveCount(4, { timeout: 5_000 });

    await page.getByRole('button', { name: /^critical$/ }).click();
    // The chip writes the filter to the URL (proves it wired through nav)...
    await expect(page).toHaveURL(/minBucket=critical/);
    // ...and narrows to exactly the two critical-bucket hosts (alice + eve).
    // toHaveCount auto-retries past the re-fetch skeleton frame.
    await expect(page.locator('tbody tr')).toHaveCount(2);
  });

  test('events tab AI Guard chip narrows the timeline', async ({ page }) => {
    await login(page);
    await page.goto('/fleet/events');
    await expect(page.locator('tbody tr').first()).toBeVisible({ timeout: 5_000 });
    const before = await page.locator('tbody tr').count();

    await page.getByRole('button', { name: /^AI Guard$/ }).click();
    // The chip writes the kind filter to the URL (JSON-encoded array)...
    await expect(page).toHaveURL(/ai_guard_risk_assessed/);
    // ...and the AI-Guard events are a strict, non-empty subset. Poll for the
    // settled (post-skeleton, >0) row set so we don't sample the 0-row
    // skeleton frame, then assert a genuine narrowing.
    await expect.poll(async () => page.locator('tbody tr').count()).toBeGreaterThan(0);
    const after = await page.locator('tbody tr').count();
    expect(after).toBeLessThan(before);
  });
});
