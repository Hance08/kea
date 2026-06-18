import { ApiError, setConfig } from '@/lib/api';
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

test('sends PATCH /api/config with the supplied body', async () => {
  fetchSpy.mockResolvedValueOnce(
    okResponse({ defaults: { currency: 'USD' }, display: { hide_decimals: true } }),
  );

  const result = await setConfig({ display: { hide_decimals: true } });

  expect(fetchSpy).toHaveBeenCalledTimes(1);
  const [url, init] = fetchSpy.mock.calls[0];
  expect(url).toBe('/api/config');
  expect(init?.method).toBe('PATCH');
  expect(init?.headers).toEqual({ 'Content-Type': 'application/json' });
  expect(init?.body).toBe('{"display":{"hide_decimals":true}}');
  expect(result).toEqual({ defaults: { currency: 'USD' }, display: { hide_decimals: true } });
});

test('rejects with ApiError when the server returns 400', async () => {
  fetchSpy.mockResolvedValueOnce(
    errorResponse(400, {
      error: 'validation_failed',
      message: 'bad body',
      field: 'display.hide_decimals',
    }),
  );

  await expect(setConfig({ display: { hide_decimals: true } })).rejects.toMatchObject({
    name: 'ApiError',
    status: 400,
    message: 'bad body',
    field: 'display.hide_decimals',
  });
});

test('thrown error is an ApiError instance', async () => {
  fetchSpy.mockResolvedValueOnce(errorResponse(500, { message: 'boom' }));

  await expect(setConfig({ display: { hide_decimals: false } })).rejects.toBeInstanceOf(ApiError);
});
