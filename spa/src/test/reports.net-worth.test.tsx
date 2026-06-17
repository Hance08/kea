import { render, screen, waitFor } from '@testing-library/react';
import { afterEach, beforeEach, expect, test, vi } from 'vitest';
import { makeTestApp } from './test-app';

const okResponse = (body: unknown) =>
  new Response(JSON.stringify(body), {
    status: 200,
    headers: { 'Content-Type': 'application/json' },
  });

beforeEach(() => {
  vi.stubGlobal(
    'fetch',
    vi.fn((url: string) => {
      if (url === '/api/config')
        return Promise.resolve(
          okResponse({ defaults: { currency: 'USD' }, display: { hide_decimals: false } }),
        );
      if (url === '/api/ledgers')
        return Promise.resolve(
          okResponse({ active: 'p', items: [{ name: 'p', path: '/p.db', active: true }] }),
        );
      if (url.startsWith('/api/reports/net-worth'))
        return Promise.resolve(
          okResponse({ at: 1781697600, net_worth: { USD: 5000000, JPY: 12000000 } }),
        );
      throw new Error(`unexpected fetch: ${url}`);
    }),
  );
});
afterEach(() => vi.unstubAllGlobals());

test('renders the default-currency net worth and currency footer', async () => {
  render(makeTestApp('/reports/net-worth'));
  await waitFor(() => {
    expect(screen.getByText('$50,000.00')).toBeInTheDocument();
  });
  // heading label rendered by the component
  expect(screen.getAllByText('Net Worth').length).toBeGreaterThan(0);
  // CurrencyFooter row for JPY
  expect(screen.getByText(/Also in this report:/i)).toBeInTheDocument();
});
