import { expect, type Page, test } from '@playwright/test';

async function login(page: Page) {
  await page.goto('/login');
  await page.getByLabel('Username').fill('admin');
  await page.getByLabel('Password').fill('test-password');
  await page.getByRole('button', { name: /^Sign in$/ }).click();
  await expect(page).toHaveURL(/\/alerts/);
}
const alice = '5a7c3e91-aaaa-bbbb-cccc-111111111111';
const eventID = (n: number) => `01990000-0000-7000-8000-${String(n).padStart(12, '0')}`;
const hook = {
  schema_version: 1,
  event_id: eventID(101),
  ts: '2026-09-13T01:00:00Z',
  host_id: alice,
  agent_version: '0.8.0',
  severity: 'warn',
  source: { kind: 'agent_hook' },
  subject: {},
  target_id: null,
  triage: null,
  evidence: {
    kind: 'hook_decision',
    agent: 'antigravity',
    peer_uid: 0,
    agent_session_id: 'session-101',
    tool_use_id: 'tool-use-101',
    action_kind: 'shell',
    action_hash: 'action-sha',
    action_preview: 'git push',
    decision: 'deny',
    rule_id: 'rule-101',
    deny_reason: 'policy match',
    enforcement_mode: 'observe',
    capture_level: 'preview',
  },
};
const eventsPattern = /\/api\/v1\/fleet\/events(?:\?|$)/;

test('Hook investigation opens from Fleet and Host, survives reload, and handles missing events', async ({
  page,
}, testInfo) => {
  await login(page);
  await page.route(eventsPattern, (route) =>
    route.fulfill({ json: { events: [hook], next_cursor: null } }),
  );
  await page.route(`**/api/v1/fleet/events/${hook.event_id}`, (route) =>
    route.fulfill({ json: hook }),
  );
  await page.goto('/fleet/events');
  await page.getByRole('button', { name: 'Hooks', exact: true }).click();
  await expect(page).toHaveURL(/hook_decision/);
  await expect(page.locator('tbody').getByText('Antigravity')).toBeVisible();
  await page.getByRole('button', { name: /Inspect Hook Decision/ }).click();
  await expect(page).toHaveURL(new RegExp(hook.event_id));
  await expect(page.getByText('session-101', { exact: true })).toBeVisible();
  await expect(page.getByRole('heading', { name: /Hook decision deny.*observe/ })).toBeVisible();
  await expect(page.getByText(/does not establish that the action was blocked/)).toBeVisible();
  await page.screenshot({ path: testInfo.outputPath('hook-investigation.png') });
  // Deep URL must resolve by ID when the event is absent from the loaded page.
  await page.unroute(eventsPattern);
  await page.route(eventsPattern, (route) =>
    route.fulfill({ json: { events: [], next_cursor: null } }),
  );
  await page.reload();
  await expect(page.getByText('tool-use-101', { exact: true })).toBeVisible();
  await page.keyboard.press('Escape');
  await expect(page).not.toHaveURL(new RegExp(hook.event_id));
  await page.goto(`/hosts/${alice}?event=${hook.event_id}`);
  await expect(page.getByText('session-101', { exact: true })).toBeVisible();
  await page.route(`**/api/v1/fleet/events/${eventID(999)}`, (route) =>
    route.fulfill({ status: 404, json: { error: { code: 'not_found', message: 'gone' } } }),
  );
  await page.goto(`/fleet/events?event=${eventID(999)}`);
  await expect(page.getByText(/Event unavailable for this view/)).toBeVisible();
});

test('Fleet events load 101 rows, apply tool/time filters, and reset paging', async ({ page }) => {
  await login(page);
  const requests: URL[] = [];
  await page.route(eventsPattern, (route) => {
    const url = new URL(route.request().url());
    requests.push(url);
    return route.fulfill({
      json: url.searchParams.has('cursor')
        ? { events: [{ ...hook, event_id: eventID(100) }, hook], next_cursor: null }
        : {
            events: Array.from({ length: 100 }, (_, i) => ({ ...hook, event_id: eventID(i + 1) })),
            next_cursor: 'after100',
          },
    });
  });
  await page.goto('/fleet/events');
  await expect(page.locator('tbody tr')).toHaveCount(100);
  await expect(page.getByText(/100 loaded · partial results/)).toBeVisible();
  await page.getByRole('button', { name: 'Load more', exact: true }).click();
  await expect(page.locator('tbody tr')).toHaveCount(101);
  await expect(page.getByText(/101 loaded · end of results/)).toBeVisible();
  await page.getByLabel('Tool (loaded results)').fill('antigravity');
  await page.getByLabel('Since (RFC3339)').fill('2026-09-01T00:00:00Z');
  await page.getByLabel('Until (RFC3339)').fill('2026-09-14T00:00:00Z');
  await page.getByRole('button', { name: 'Apply filters' }).click();
  await expect(page.locator('tbody tr')).toHaveCount(100);
  expect(requests.at(-1)?.searchParams.has('cursor')).toBe(false);
  expect(requests.at(-1)?.searchParams.get('since')).toBe('2026-09-01T00:00:00Z');
  expect(requests.at(-1)?.searchParams.get('until')).toBe('2026-09-14T00:00:00Z');
});

test('Alerts can search beyond the first page without claiming no alerts', async ({ page }) => {
  await login(page);
  const riskEvent = {
    ...hook,
    evidence: {
      kind: 'ai_guard_risk_assessed',
      tool: 'codex',
      scope: { kind: 'user_global' },
      score: 8,
      bucket: 'high',
      reasons: [],
      is_reattestation: false,
    },
  };
  await page.route(eventsPattern, (route) =>
    route.fulfill({
      json: new URL(route.request().url()).searchParams.has('cursor')
        ? { events: [{ ...riskEvent, event_id: eventID(101) }], next_cursor: null }
        : {
            events: Array.from({ length: 100 }, (_, i) => ({
              ...riskEvent,
              event_id: eventID(i + 1),
            })),
            next_cursor: 'after100',
          },
    }),
  );
  await page.goto(`/alerts?query=${eventID(101)}`);
  await expect(page.getByText(/No matching alerts in loaded results/)).toBeVisible();
  await expect(
    page.getByText(/Search and status filters apply to loaded results only/),
  ).toBeVisible();
  await page.getByRole('button', { name: 'Load more', exact: true }).click();
  await expect(page.getByText('1 shown of 101 fetched')).toBeVisible();
  await page.locator('button:has-text("AI Guard risk")').first().click();
  await expect(page.getByLabel('Assignee')).toBeVisible();
});

test('risk/compliance paging and host policy lookup reach row 101', async ({ page }) => {
  await login(page);
  const risk = {
    host_id: alice,
    hostname: 'target-101',
    score: 8,
    bucket: 'high',
    top_tool: 'codex',
    reasons_count: 1,
    assessed_ts: '2026-09-13T00:00:00Z',
    open_alert_count_24h: 1,
  };
  const policy = {
    host_id: alice,
    hostname: 'target-101',
    last_applied_policy_version: 1,
    server_current_policy_version: 2,
    version_drift: 1,
    policy_expired_active: false,
    last_policy_reload_ts: null,
    signature_failures_24h: 3,
  };
  for (const [endpoint, row] of [
    ['risk', risk],
    ['compliance', policy],
  ] as const) {
    await page.route(new RegExp(`/api/v1/fleet/${endpoint}(?:\\?|$)`), (route) =>
      route.fulfill({
        json: new URL(route.request().url()).searchParams.has('cursor')
          ? { rows: [row], next_cursor: null }
          : {
              rows: Array.from({ length: 100 }, (_, i) => ({
                ...row,
                host_id: `host-${i}`,
                hostname: `host-${i}`,
              })),
              next_cursor: 'page2',
            },
      }),
    );
  }
  await page.goto('/fleet/risk');
  await expect(page.locator('tbody tr')).toHaveCount(100);
  await page.getByRole('button', { name: 'Load more', exact: true }).click();
  await expect(page.locator('tbody tr')).toHaveCount(101);
  await page.getByRole('button', { name: 'critical', exact: true }).click();
  await expect(page.locator('tbody tr')).toHaveCount(100);
  // No compliance page has been opened yet: host view must advance the feed itself.
  await page.goto(`/hosts/${alice}`);
  await expect(page.getByText('Failing signature', { exact: true })).toBeVisible();
  await page.goto('/fleet/compliance');
  await page.reload();
  await expect(page.locator('tbody tr')).toHaveCount(100);
  await page.getByRole('button', { name: 'Load more', exact: true }).click();
  await expect(page.locator('tbody tr')).toHaveCount(101);
});

for (const scenario of [
  { status: 502, code: 'upstream_unauthorized', message: 'Read API token rejected' },
  { status: 502, code: 'read_api_disabled', message: 'Read API disabled' },
  { status: 503, code: 'service_unavailable', message: 'Rebuilding index' },
  { status: 502, code: 'upstream_error', message: 'Read API unavailable' },
]) {
  test(`Settings keeps liveness separate from ${scenario.code}`, async ({ page }) => {
    await page.route('**/api/v1/fleet/meta', (route) =>
      route.fulfill({
        status: scenario.status,
        json: { error: { code: scenario.code, message: 'upstream failed' } },
      }),
    );
    await login(page);
    await page.goto('/settings');
    await expect(page.getByText('Reachable', { exact: true })).toBeVisible();
    await expect(page.locator('header').getByText('Read API error', { exact: true })).toBeVisible();
    await expect(page.getByRole('alert').filter({ hasText: scenario.message })).toBeVisible();
    await expect(
      page.getByText('Metadata unavailable. License and audit status are unknown.'),
    ).toBeVisible();
    await expect(page.getByText('none (open-source server)', { exact: true })).toHaveCount(0);
    await expect(page.getByText('disabled', { exact: true })).toHaveCount(0);
  });
}

test('Settings labels stale cached metadata and recovers with Retry', async ({
  page,
}, testInfo) => {
  await login(page);
  await page.goto('/settings');
  await expect(page.getByText('Connected (authenticated)', { exact: true })).toBeVisible();
  await page.route('**/api/v1/fleet/meta', (route) =>
    route.fulfill({
      status: 502,
      json: { error: { code: 'upstream_unauthorized', message: 'rejected' } },
    }),
  );
  await page.getByRole('button', { name: 'Retry connection', exact: true }).click();
  await expect(page.getByText(/Cached metadata — last successful fetch/)).toBeVisible();
  await expect(page.getByRole('heading', { name: 'Audit (cached)' })).toBeVisible();
  await page.screenshot({ path: testInfo.outputPath('settings-cached-error.png') });
  await page.unroute('**/api/v1/fleet/meta');
  await page.getByRole('button', { name: 'Retry connection', exact: true }).click();
  await expect(page.getByText('Connected (authenticated)', { exact: true })).toBeVisible();
  await expect(page.getByText(/Cached metadata — last successful fetch/)).toHaveCount(0);
});

test('Settings distinguishes successful legacy metadata from missing signed head', async ({
  page,
}) => {
  const meta = {
    server_version: '0.5.0',
    schema_version: 1,
    ts: '2026-09-13T00:00:00Z',
    alerts_definition_default: {
      evidence_kinds: ['ai_guard_risk_assessed'],
      ai_guard_buckets: ['high', 'critical'],
      additional_kinds: [],
    },
  };
  await page.route('**/api/v1/fleet/meta', (route) => route.fulfill({ json: meta }));
  await login(page);
  await page.goto('/settings');
  await expect(page.getByText('Not reported by this server', { exact: true })).toBeVisible();
  await expect(page.getByText('not reported by this server', { exact: true })).toBeVisible();
  await page.unroute('**/api/v1/fleet/meta');
  await page.route('**/api/v1/fleet/meta', (route) =>
    route.fulfill({ json: { ...meta, audit_head: null } }),
  );
  await page.getByRole('button', { name: 'Retry connection', exact: true }).click();
  await expect(page.getByText('no signed head available', { exact: true })).toBeVisible();
});
