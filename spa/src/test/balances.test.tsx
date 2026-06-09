import { render, screen } from '@testing-library/react';
import { afterEach, beforeEach, expect, test, vi } from 'vitest';
import { makeTestApp } from './setup';

const okResponse = (body: unknown) =>
  new Response(JSON.stringify(body), {
    status: 200,
    headers: { 'Content-Type': 'application/json' },
  });

beforeEach(() => {
  vi.stubGlobal(
    'fetch',
    vi.fn((url: string) => {
      if (url === '/api/config') {
        return Promise.resolve(okResponse({ defaults: { currency: 'USD' } }));
      }
      if (url === '/api/balances') {
        return Promise.resolve(
          okResponse({
            items: [
              {
                account_id: 1,
                name: 'Assets:Bank',
                type: 'A',
                currency: 'USD',
                amount: 125000,
                is_hidden: false,
              },
              {
                account_id: 2,
                name: 'Assets:Cash',
                type: 'A',
                currency: 'USD',
                amount: 3500,
                is_hidden: false,
              },
              {
                account_id: 3,
                name: 'Liab:Card',
                type: 'L',
                currency: 'USD',
                amount: -42000,
                is_hidden: false,
              },
            ],
            total_count: 3,
            limit: 0,
            offset: 0,
          }),
        );
      }
      throw new Error(`unexpected fetch: ${url}`);
    }),
  );
});

afterEach(() => {
  vi.unstubAllGlobals();
});

test('renders Net Worth headline from a balances response', async () => {
  render(makeTestApp('/balances'));

  // Net Worth headline appears.
  expect(await screen.findByText(/Net Worth/i)).toBeInTheDocument();

  // Math: 125000 + 3500 + (-42000) = 86500 cents = $865.00
  expect(await screen.findByText('$865.00')).toBeInTheDocument();
});
