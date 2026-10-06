import { afterEach, beforeEach, expect, test, vi } from 'vitest';
import {
  deleteSavingsTarget,
  fetchSavingsReport,
  fetchSavingsTargets,
  setSavingsTarget,
  stopSavingsTarget,
} from '../lib/api/savings';

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

test('fetchSavingsTargets GETs the list', async () => {
  await fetchSavingsTargets();
  expect(fetchSpy.mock.calls[0][0]).toBe('/api/savings-targets');
});

test('setSavingsTarget PUTs JSON', async () => {
  await setSavingsTarget({
    account_name: 'Assets:Savings',
    effective_month: '2026-01',
    amount: 100,
  });
  const [url, init] = fetchSpy.mock.calls[0];
  expect(url).toBe('/api/savings-targets');
  expect(init.method).toBe('PUT');
  expect(JSON.parse(init.body)).toEqual({
    account_name: 'Assets:Savings',
    effective_month: '2026-01',
    amount: 100,
  });
});

test('stopSavingsTarget POSTs to /stop', async () => {
  await stopSavingsTarget({ account_name: 'Assets:Savings', effective_month: '2026-02' });
  expect(fetchSpy.mock.calls[0][0]).toBe('/api/savings-targets/stop');
  expect(fetchSpy.mock.calls[0][1].method).toBe('POST');
});

test('deleteSavingsTarget DELETEs by id', async () => {
  await deleteSavingsTarget(7);
  expect(fetchSpy.mock.calls[0][0]).toBe('/api/savings-targets/7');
  expect(fetchSpy.mock.calls[0][1].method).toBe('DELETE');
});

test('fetchSavingsReport omits month when undefined', async () => {
  await fetchSavingsReport();
  expect(fetchSpy.mock.calls[0][0]).toBe('/api/reports/savings');
  await fetchSavingsReport('2026-03');
  expect(fetchSpy.mock.calls[1][0]).toBe('/api/reports/savings?month=2026-03');
});
