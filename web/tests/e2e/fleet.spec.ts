import { expect, type Page, test } from '@playwright/test';

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
    // ...and the AI-Guard events are a strict, non-empty subset. Poll until the
    // count settles BELOW the unfiltered count: keepPreviousData keeps the old
    // rows visible during the refetch, so we must wait for the narrowed set to
    // land rather than snapshotting immediately after the click.
    await expect.poll(async () => page.locator('tbody tr').count()).toBeLessThan(before);
    await expect.poll(async () => page.locator('tbody tr').count()).toBeGreaterThan(0);
  });

  test('clicking a hostname opens the host detail page', async ({ page }) => {
    await login(page);
    await page.goto('/fleet/risk');
    // alice-mbp is a risk row; its hostname cell is now a link.
    await page.getByRole('link', { name: /alice/ }).first().click();
    await expect(page).toHaveURL(/\/hosts\/5a7c3e91-aaaa-bbbb-cccc-111111111111/);
    // Header + AI Guard block render. Scope "AI Guard risk" to the section
    // heading — the literal also appears as "Ai Guard Risk Assessed" event-kind
    // cells in the Recent Events table below (strict-mode would match both).
    await expect(page.getByRole('heading', { name: /alice-mbp/ })).toBeVisible({ timeout: 5_000 });
    await expect(page.getByRole('heading', { name: /AI Guard risk/i })).toBeVisible();
    await expect(page.getByText(/claude code/i).first()).toBeVisible();
  });

  test('host detail shows metadata + per-host events', async ({ page }) => {
    await login(page);
    await page.goto('/hosts/5a7c3e91-aaaa-bbbb-cccc-111111111111');
    await expect(page.getByText(/Host metadata/i)).toBeVisible({ timeout: 5_000 });
    await expect(page.getByText(/Policy & agent health/i)).toBeVisible();
    await expect(page.getByText(/Recent events/i)).toBeVisible();
  });

  test('unknown host id shows the not-found panel', async ({ page }) => {
    await login(page);
    await page.goto('/hosts/00000000-0000-0000-0000-000000000000');
    await expect(page.getByText(/Host not found/i)).toBeVisible({ timeout: 5_000 });
  });

  test('a disconnected null-heavy host renders without crashing', async ({ page }) => {
    await login(page);
    // dave-vm: hostname null, no host_meta / ai_guard / agent_health — the
    // null-tolerance path (this is where the null-reasons crash first hid).
    await page.goto('/hosts/5a7c3e91-aaaa-bbbb-cccc-444444444444');
    await expect(page.getByText(/No AI Guard assessments yet/i)).toBeVisible({ timeout: 5_000 });
    await expect(page.getByText(/No host metadata reported yet/i)).toBeVisible();
  });
});

test('observed controls survive the host API and remain inspectable at low risk', async ({
  page,
}) => {
  await login(page);
  const path = '/api/v1/fleet/hosts/5a7c3e91-aaaa-bbbb-cccc-222222222222';
  const response = await page.request.get(path);
  expect(response.ok()).toBeTruthy();
  const host = await response.json();
  expect(host.ai_guard.by_tool.codex.controls[0].value).toBe(false);
  await page.goto('/hosts/5a7c3e91-aaaa-bbbb-cccc-222222222222');
  const settings = page.getByRole('region', { name: 'Observed security settings' });
  await expect(settings.getByText('false', { exact: true })).toBeVisible();
  await expect(settings.getByText('/etc/codex/requirements.toml').first()).toBeVisible();
  await expect(
    settings.getByText('["read-only","workspace-write"]', { exact: true }),
  ).toBeVisible();
  await page.screenshot({ path: '/tmp/sigil-manager-observed-controls-host.png', fullPage: true });

  // Same observation on a quiet tool must remain accessible by keyboard.
  host.ai_guard.by_tool.codex.bucket = 'low';
  host.ai_guard.by_tool.codex.score = 0;
  await page.route(`**${path}`, (route) => route.fulfill({ json: host }));
  await page.reload();
  const summary = page.locator('summary').filter({ hasText: 'Low (1)' });
  await summary.focus();
  await page.keyboard.press('Enter');
  await expect(settings.getByText('false', { exact: true })).toBeVisible();
  await expect(page.getByText('low 0.00', { exact: true })).toBeVisible();
});
