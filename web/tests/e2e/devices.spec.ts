import { expect, test } from '@playwright/test';

test('Devices includes unassessed hosts, pages, compares versions and filters at server', async ({ page }, testInfo) => {
  await page.goto('/login');
  await page.getByLabel('Username').fill('admin');
  await page.getByLabel('Password').fill('test-password');
  await page.getByRole('button', { name: /^Sign in$/ }).click();
  await expect(page).toHaveURL(/\/alerts/);
  const host = {
    host_id: '5a7c3e91-aaaa-bbbb-cccc-111111111111', hostname: 'Unassessed laptop',
    agent_version: '0.8.3', status: 'healthy', last_seen_ts: new Date().toISOString(),
    current_risk: null, open_event_counts_24h: {},
  };
  const requests: URL[] = [];
  let fail = false;
  await page.route(/\/api\/v1\/fleet\/hosts(?:\?|$)/, route => {
    const url = new URL(route.request().url());
    requests.push(url);
    if (fail) return route.fulfill({ status: 503, json: { error: { code: 'service_unavailable', message: 'Rebuilding index' } } });
    const filtered = url.searchParams.has('status');
    const next = url.searchParams.has('cursor');
    return route.fulfill({ json: {
      hosts: filtered ? [{ ...host, status: 'disconnected' }] : next ? [host, { ...host, host_id: 'second-host', hostname: 'Old laptop', agent_version: '0.8.0' }] : [host],
      next_cursor: filtered || next ? null : 'next-page', total_estimated: 2,
    } });
  });
  await page.goto('/fleet/devices');
  await expect(page.getByRole('cell', { name: 'Not assessed', exact: true })).toBeVisible();
  await page.getByLabel('Target agent version').fill('v0.8.3');
  await expect(page.getByText('Matches target', { exact: true })).toBeVisible();
  await page.getByRole('button', { name: 'Load more', exact: true }).click();
  await expect(page.locator('tbody tr')).toHaveCount(2);
  await expect(page.getByText('Differs from target', { exact: true })).toBeVisible();
  await expect(page.getByText(/2 loaded · end of results/)).toBeVisible();
  await page.screenshot({ path: testInfo.outputPath('devices.png'), fullPage: true });
  await page.getByLabel('Search loaded devices').fill('0.8.0');
  await expect(page.locator('tbody tr')).toHaveCount(1);
  await expect(page.getByRole('link', { name: 'Old laptop' })).toBeVisible();
  await page.getByLabel('Search loaded devices').fill('');
  await page.getByLabel('Connection status').selectOption('disconnected');
  await expect(page.locator('tbody tr')).toHaveCount(1);
  await expect(page).toHaveURL(/status=disconnected/);
  expect(requests.at(-1)?.searchParams.get('status')).toBe('disconnected');
  expect(requests.at(-1)?.searchParams.has('cursor')).toBe(false);
  fail = true;
  await page.getByRole('button', { name: 'Refresh', exact: true }).click();
  await expect(page.getByRole('alert')).toContainText('Rebuilding index');
  await expect(page.locator('tbody tr')).toHaveCount(1);
  fail = false;
  await page.getByRole('button', { name: 'Retry', exact: true }).click();
  await expect(page.getByRole('alert')).toHaveCount(0);
  await page.getByRole('link', { name: 'Unassessed laptop' }).click();
  await expect(page).toHaveURL(new RegExp(`/hosts/${host.host_id}`));
});
