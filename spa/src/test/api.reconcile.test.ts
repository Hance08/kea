import { ApiError } from '@/lib/api';
import { commitReconcile, getUnreconciled, previewReconcile } from '@/lib/reconcile';
import { afterEach, beforeEach, expect, test, vi } from 'vitest';

const okResponse = (body: unknown) =>
  new Response(JSON.stringify(body), {
    status: 200,
    headers: { 'Content-Type': 'application/json' },
  });

const errorResponse = (status: number, body: unknown) =>
  new Response(JSON.stringify(body), {
    status,
    headers: { 'Content-Type': 'application/json' },
  });

let fetchSpy: ReturnType<typeof vi.fn>;

beforeEach(() => {
  fetchSpy = vi.fn();
  vi.stubGlobal('fetch', fetchSpy);
});

afterEach(() => {
  vi.unstubAllGlobals();
});

test('getUnreconciled GETs the correct path and returns entries + last balance', async () => {
  fetchSpy.mockResolvedValueOnce(
    okResponse({
      entries: [
        {
          id: 18,
          timestamp: 1735689600,
          description: 'Coffee',
          status: 'Cleared',
          amount: -450,
          offset_account: 'Expenses:Coffee',
        },
      ],
      last_reconciled_balance: 250000,
    }),
  );

  const result = await getUnreconciled(42);

  expect(fetchSpy).toHaveBeenCalledWith('/api/accounts/42/unreconciled', undefined);
  expect(result.last_reconciled_balance).toBe(250000);
  expect(result.entries).toHaveLength(1);
  expect(result.entries[0].offset_account).toBe('Expenses:Coffee');
});

test('previewReconcile POSTs statement_balance + transaction_ids and returns difference', async () => {
  fetchSpy.mockResolvedValueOnce(okResponse({ difference: 5000 }));

  const result = await previewReconcile(42, 749550, [18, 19]);

  const [url, init] = fetchSpy.mock.calls[0];
  expect(url).toBe('/api/accounts/42/reconcile/preview');
  expect(init?.method).toBe('POST');
  expect(init?.headers).toEqual({ 'Content-Type': 'application/json' });
  expect(JSON.parse(init?.body as string)).toEqual({
    statement_balance: 749550,
    transaction_ids: [18, 19],
  });
  expect(result.difference).toBe(5000);
});

test('commitReconcile POSTs allow_mismatch and returns counts + last balance', async () => {
  fetchSpy.mockResolvedValueOnce(
    okResponse({ reconciled_count: 2, difference: 0, last_reconciled_balance: 749550 }),
  );

  const result = await commitReconcile(42, 749550, [18, 19], false);

  const [url, init] = fetchSpy.mock.calls[0];
  expect(url).toBe('/api/accounts/42/reconcile');
  expect(init?.method).toBe('POST');
  expect(JSON.parse(init?.body as string)).toEqual({
    statement_balance: 749550,
    transaction_ids: [18, 19],
    allow_mismatch: false,
  });
  expect(result.reconciled_count).toBe(2);
  expect(result.last_reconciled_balance).toBe(749550);
});

test('commitReconcile rejects with ApiError carrying difference on 409', async () => {
  const mismatchBody = {
    error: 'balance_mismatch',
    message: 'statement balance off by 50450',
    difference: 50450,
  };
  fetchSpy.mockResolvedValueOnce(errorResponse(409, mismatchBody));
  fetchSpy.mockResolvedValueOnce(errorResponse(409, mismatchBody));

  await expect(commitReconcile(42, 800000, [18, 19], false)).rejects.toMatchObject({
    name: 'ApiError',
    status: 409,
    difference: 50450,
  });
  await expect(commitReconcile(42, 800000, [18, 19], false)).rejects.toBeInstanceOf(ApiError);
});
