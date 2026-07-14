import { ApiError, apiFetch } from '@/lib/api';
import { afterEach, beforeEach, expect, test, vi } from 'vitest';

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

test('apiFetch surfaces the body difference field on 409 errors', async () => {
  fetchSpy.mockResolvedValueOnce(
    errorResponse(409, {
      error: 'balance_mismatch',
      message: 'statement balance off by 1050',
      difference: 1050,
    }),
  );

  await expect(apiFetch('/api/anything')).rejects.toMatchObject({
    name: 'ApiError',
    status: 409,
    message: 'statement balance off by 1050',
    difference: 1050,
  });
});

test('difference is undefined when the body omits it', async () => {
  fetchSpy.mockResolvedValueOnce(
    errorResponse(400, { error: 'validation_failed', message: 'bad', field: 'foo' }),
  );

  try {
    await apiFetch('/api/anything');
    throw new Error('expected to throw');
  } catch (e) {
    expect(e).toBeInstanceOf(ApiError);
    expect((e as ApiError).difference).toBeUndefined();
    expect((e as ApiError).field).toBe('foo');
  }
});
