import { ServerConfigProvider, useServerConfig } from '@/lib/server-config';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { render, screen } from '@testing-library/react';
import { afterEach, beforeEach, expect, test, vi } from 'vitest';

const okResponse = (body: unknown) =>
  new Response(JSON.stringify(body), {
    status: 200,
    headers: { 'Content-Type': 'application/json' },
  });

function CurrencyReadout() {
  const cfg = useServerConfig();
  return <div data-testid="currency">{cfg.defaults.currency}</div>;
}

function renderWithProvider() {
  const queryClient = new QueryClient({
    defaultOptions: { queries: { retry: false, gcTime: 0 } },
  });
  return render(
    <QueryClientProvider client={queryClient}>
      <ServerConfigProvider fallback={<div data-testid="loading">Loading...</div>}>
        {() => <CurrencyReadout />}
      </ServerConfigProvider>
    </QueryClientProvider>,
  );
}

beforeEach(() => {
  vi.stubGlobal(
    'fetch',
    vi.fn((url: string) => {
      if (url === '/api/config') {
        return Promise.resolve(okResponse({ defaults: { currency: 'TWD' } }));
      }
      throw new Error(`unexpected fetch: ${url}`);
    }),
  );
});

afterEach(() => {
  vi.unstubAllGlobals();
});

test('renders fallback while /api/config is pending, then exposes config to children', async () => {
  renderWithProvider();
  expect(screen.getByTestId('loading')).toBeInTheDocument();
  expect(await screen.findByTestId('currency')).toHaveTextContent('TWD');
});

test('keeps fallback visible when /api/config fails', async () => {
  vi.stubGlobal(
    'fetch',
    vi.fn(() => Promise.resolve(new Response('{}', { status: 500 }))),
  );
  renderWithProvider();
  await Promise.resolve();
  expect(screen.getByTestId('loading')).toBeInTheDocument();
  expect(screen.queryByTestId('currency')).not.toBeInTheDocument();
});
