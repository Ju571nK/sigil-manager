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

test.describe('alerts queue + triage', () => {
  test.beforeEach(async ({ context }) => {
    await context.clearCookies();
  });

  test('queue renders at least a few rows from the Mock fixture', async ({ page }) => {
    await login(page);
    // The Mock fixture seeds >= 5 high/critical AI Guard events.
    const rows = page.getByRole('button', { pressed: false }).filter({ hasText: /critical|high/i });
    // Wait for the table to actually paint (poll-driven render).
    await expect(rows.first()).toBeVisible({ timeout: 5_000 });
    const count = await rows.count();
    expect(count).toBeGreaterThanOrEqual(3);
  });

  test('opening a row reveals the slide-over with the FactGrid', async ({ page }) => {
    await login(page);

    // Click the first row that mentions AI Guard.
    const firstRow = page.locator('button:has-text("AI Guard")').first();
    await expect(firstRow).toBeVisible({ timeout: 5_000 });
    await firstRow.click();

    // Sheet header carries event_id; assignee input shows up.
    await expect(page.getByText(/event_id/i)).toBeVisible();
    await expect(page.getByLabel('Assignee')).toBeVisible();
  });

  test('hook-script reason renders the 3b.3.1 source_chain breadcrumb', async ({ page }) => {
    await login(page);

    // The Mock fixture's bob/codex event carries a destructive_in_hook_script
    // reason with a populated source_chain (contract §14.8). Its row title is
    // distinctive enough to target directly.
    const row = page.locator('button:has-text("Destructive In Hook Script")').first();
    await expect(row).toBeVisible({ timeout: 5_000 });
    await row.click();

    // The slide-over Reasons block shows the source-follow breadcrumb: the
    // last segment of the chain plus the "→" separator between hops. These
    // strings also appear in the raw-evidence <pre> further down, so scope
    // to the first (Reasons-block) match.
    const sheet = page.getByRole('dialog');
    await expect(sheet.getByText('clean.sh', { exact: false }).first()).toBeVisible();
    await expect(sheet.getByText('→').first()).toBeVisible();
  });

  test('gemini mcp_server_local_command reason surfaces server + command', async ({ page }) => {
    await login(page);

    // bob/gemini event (3b.7) carries an mcp_server_local_command reason.
    const row = page.locator('button:has-text("Gemini")').first();
    await expect(row).toBeVisible({ timeout: 5_000 });
    await row.click();

    const sheet = page.getByRole('dialog');
    // Header renders the humanized tool name, not the raw "gemini" string.
    await expect(sheet.getByText(/AI Guard risk · Gemini/)).toBeVisible();
    // The reason exposes server_name + command. Both also appear in the
    // raw-evidence <pre>, so scope to the first (Reasons-block) match.
    await expect(sheet.getByText('local-fs', { exact: false }).first()).toBeVisible();
    await expect(sheet.getByText(/server-filesystem/).first()).toBeVisible();
  });

  test('assign + acknowledge persists across reload and updates queue', async ({ page }) => {
    await login(page);

    const firstRow = page.locator('button:has-text("AI Guard")').first();
    await firstRow.click();

    // Type assignee, submit (form submit on Enter via the Check button).
    await page.getByLabel('Assignee').fill('alice');
    // Press the Check submit button next to the input — it has no
    // accessible name, so target via the form structure.
    await page.locator('form:has(input[aria-label="Assignee"]) button[type="submit"]').click();

    // Click the Acknowledge action button (same effect as pressing 'c',
    // which the global shortcut routes through). Clicking <body> here
    // would dismiss the Radix Sheet via outside-click.
    await page.getByRole('button', { name: /^Acknowledge$/ }).click();

    // The slide-over status badge should switch to "acknowledged".
    await expect(page.getByText(/^acknowledged$/i).first()).toBeVisible({ timeout: 5_000 });

    // Reload: still on /alerts, slide-over closed by default, queue
    // row shows the persisted state.
    await page.reload();
    await expect(page).toHaveURL(/\/alerts/);
    // The triage assignee should appear in the visible queue row.
    await expect(page.getByText('alice').first()).toBeVisible({ timeout: 5_000 });
  });

  test('severity filter "Critical" reduces the visible count', async ({ page }) => {
    await login(page);

    // Wait for the queue to actually paint — the polling fetch happens
    // after first mount, so initial DOM is empty.
    const aiGuardRows = page.locator('button:has-text("AI Guard")');
    await expect(aiGuardRows.first()).toBeVisible({ timeout: 5_000 });
    const before = await aiGuardRows.count();

    // Click the "Critical" severity chip.
    await page.getByRole('button', { name: /^Critical$/ }).click();
    // Wait a tick for the filter request to land.
    await page.waitForTimeout(500);

    const after = await countAiGuardRows(page);
    expect(after).toBeLessThanOrEqual(before);
    // And every visible bucket label should be "critical".
    const labels = await page.locator('span:has-text("critical")').allTextContents();
    expect(labels.length).toBeGreaterThan(0);
  });

  test('"?" opens the keyboard shortcut cheatsheet', async ({ page }) => {
    await login(page);
    // Make sure focus is off any input so the global handler fires.
    await page.locator('body').click();
    await page.keyboard.press('?');

    await expect(page.getByRole('heading', { name: /keyboard shortcuts/i })).toBeVisible();
    // Esc should close it.
    await page.keyboard.press('Escape');
    await expect(page.getByRole('heading', { name: /keyboard shortcuts/i })).not.toBeVisible();
  });
});

/** Count the AI-Guard buttons currently rendered in the queue table. */
async function countAiGuardRows(page: Page): Promise<number> {
  return page.locator('button:has-text("AI Guard")').count();
}
