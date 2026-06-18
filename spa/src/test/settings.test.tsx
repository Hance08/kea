import { Route as SettingsRoute } from '@/routes/settings';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { render, screen, waitFor } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import type React from 'react';
import { afterEach, beforeEach, expect, test, vi } from 'vitest';
import { withServerConfig } from './test-app';

let fetchSpy: ReturnType<typeof vi.fn>;

beforeEach(() => {
  fetchSpy = vi.fn();
  vi.stubGlobal('fetch', fetchSpy);
});

afterEach(() => {
  vi.unstubAllGlobals();
});

function renderSettings(hideDecimals: boolean) {
  const SettingsPage = SettingsRoute.options.component as React.FC;

  const queryClient = new QueryClient({
    defaultOptions: { queries: { retry: false, gcTime: 0 } },
  });
  const invalidateSpy = vi.spyOn(queryClient, 'invalidateQueries');

  const tree = render(
    <QueryClientProvider client={queryClient}>
      {withServerConfig(<SettingsPage />, { display: { hide_decimals: hideDecimals } })}
    </QueryClientProvider>,
  );
  return { ...tree, queryClient, invalidateSpy };
}

test('renders the hide_decimals switch with the current value', () => {
  renderSettings(false);
  const sw = screen.getByRole('switch', { name: /hide decimal places/i });
  expect(sw).toHaveAttribute('aria-checked', 'false');
});

test('clicking the switch PATCHes /api/config and invalidates server-config', async () => {
  fetchSpy.mockResolvedValueOnce(
    new Response(
      JSON.stringify({ defaults: { currency: 'USD' }, display: { hide_decimals: true } }),
      { status: 200, headers: { 'Content-Type': 'application/json' } },
    ),
  );

  const { invalidateSpy } = renderSettings(false);

  await userEvent.click(screen.getByRole('switch', { name: /hide decimal places/i }));

  await waitFor(() => expect(fetchSpy).toHaveBeenCalledTimes(1));
  const [url, init] = fetchSpy.mock.calls[0];
  expect(url).toBe('/api/config');
  expect(init?.method).toBe('PATCH');
  expect(init?.body).toBe('{"display":{"hide_decimals":true}}');

  await waitFor(() =>
    expect(invalidateSpy).toHaveBeenCalledWith({ queryKey: ['server-config'] }),
  );
});

test('does not invalidate the server-config query when PATCH fails', async () => {
  fetchSpy.mockResolvedValueOnce(
    new Response(JSON.stringify({ error: 'validation_failed', message: 'bad' }), {
      status: 400,
      headers: { 'Content-Type': 'application/json' },
    }),
  );

  const { invalidateSpy } = renderSettings(false);

  await userEvent.click(screen.getByRole('switch', { name: /hide decimal places/i }));

  await waitFor(() => expect(fetchSpy).toHaveBeenCalledTimes(1));
  expect(invalidateSpy).not.toHaveBeenCalledWith({ queryKey: ['server-config'] });
});
