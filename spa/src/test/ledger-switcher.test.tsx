import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { render, screen } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { afterEach, beforeEach, expect, test, vi } from 'vitest';
import { LedgerSwitcher } from '../components/LedgerSwitcher';

const okResponse = (body: unknown) =>
  new Response(JSON.stringify(body), {
    status: 200,
    headers: { 'Content-Type': 'application/json' },
  });

const LEDGERS_OK = {
  active: 'personal',
  items: [
    { name: 'personal', path: '/p/personal.db', active: true },
    { name: 'business', path: '/p/business.db', active: false },
  ],
};

function renderSwitcher() {
  const queryClient = new QueryClient({
    defaultOptions: { queries: { retry: false, gcTime: 0 } },
  });
  return render(
    <QueryClientProvider client={queryClient}>
      <LedgerSwitcher />
    </QueryClientProvider>,
  );
}

beforeEach(() => {
  vi.stubGlobal(
    'fetch',
    vi.fn((url: string) => {
      if (url === '/api/ledgers') {
        return Promise.resolve(okResponse(LEDGERS_OK));
      }
      throw new Error(`unexpected fetch: ${url}`);
    }),
  );
});

afterEach(() => {
  vi.unstubAllGlobals();
});

test('trigger shows the active ledger name', async () => {
  renderSwitcher();
  expect(await screen.findByRole('button', { name: /personal/i })).toBeInTheDocument();
});

test('opens menu, lists ledgers, marks active row disabled with check', async () => {
  renderSwitcher();
  const trigger = await screen.findByRole('button', { name: /personal/i });
  await userEvent.click(trigger);

  const personal = await screen.findByRole('menuitem', { name: /personal/i });
  const business = await screen.findByRole('menuitem', { name: /business/i });

  // Active row carries an aria-disabled marker (Radix sets data-disabled).
  expect(personal).toHaveAttribute('data-disabled');
  expect(business).not.toHaveAttribute('data-disabled');

  // The active row contains the check-mark indicator we tag with a testid.
  expect(personal.querySelector('[data-testid="ledger-active-check"]')).not.toBeNull();
  expect(business.querySelector('[data-testid="ledger-active-check"]')).toBeNull();
});
