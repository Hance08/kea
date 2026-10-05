import { afterEach, beforeEach, expect, test, vi } from 'vitest';
import { deleteBudget, fetchBudgetReport, setBudget, stopBudget } from '../lib/api/budgets';

let fetchSpy: ReturnType<typeof vi.fn>;
const ok = (body: unknown) =>
  new Response(JSON.stringify(body), {
    status: 200,
    headers: { 'Content-Type': 'application/json' },
  });

beforeEach(() => {
  fetchSpy = vi.fn(() => Promise.resolve(ok({})));
  vi.stubGlobal('fetch', fetchSpy);
});
afterEach(() => vi.unstubAllGlobals());

test('setBudget PUTs JSON', async () => {
  await setBudget({ account_name: 'Expenses:Food', effective_month: '2026-01', amount: 100 });
  const [url, init] = fetchSpy.mock.calls[0];
  expect(url).toBe('/api/budgets');
  expect(init.method).toBe('PUT');
  expect(JSON.parse(init.body)).toEqual({
    account_name: 'Expenses:Food',
    effective_month: '2026-01',
    amount: 100,
  });
});

test('stopBudget POSTs to /stop', async () => {
  await stopBudget({ account_name: 'Expenses:Food', effective_month: '2026-02' });
  expect(fetchSpy.mock.calls[0][0]).toBe('/api/budgets/stop');
  expect(fetchSpy.mock.calls[0][1].method).toBe('POST');
});

test('deleteBudget DELETEs by id', async () => {
  await deleteBudget(7);
  expect(fetchSpy.mock.calls[0][0]).toBe('/api/budgets/7');
  expect(fetchSpy.mock.calls[0][1].method).toBe('DELETE');
});

test('fetchBudgetReport omits month when undefined', async () => {
  await fetchBudgetReport();
  expect(fetchSpy.mock.calls[0][0]).toBe('/api/reports/budget');
  await fetchBudgetReport('2026-03');
  expect(fetchSpy.mock.calls[1][0]).toBe('/api/reports/budget?month=2026-03');
});
