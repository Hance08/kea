import { render, screen, waitFor, within } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
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
      if (url === '/api/ledgers') {
        return Promise.resolve(
          okResponse({
            active: 'personal',
            items: [{ name: 'personal', path: '/p/personal.db', active: true }],
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

test('partitions rows into Assets and Liabilities columns', async () => {
  render(makeTestApp('/balances'));

  // Column header bars
  const assetsHeader = await screen.findByText('Assets');
  const liabilitiesHeader = await screen.findByText('Liabilities');

  // Each column is a Card — the closest Card ancestor of the label header
  // holds the rows. Find the Card by walking up to the element with a role
  // or by scoping queries to the body container.
  const assetsCol = assetsHeader.closest('[class*="overflow-hidden"]');
  const liabilitiesCol = liabilitiesHeader.closest('[class*="overflow-hidden"]');
  if (!assetsCol || !liabilitiesCol) throw new Error('columns not found');

  expect(within(assetsCol as HTMLElement).getByText('Assets:Bank')).toBeInTheDocument();
  expect(within(assetsCol as HTMLElement).getByText('Assets:Cash')).toBeInTheDocument();
  expect(within(assetsCol as HTMLElement).queryByText('Liab:Card')).not.toBeInTheDocument();

  expect(within(liabilitiesCol as HTMLElement).getByText('Liab:Card')).toBeInTheDocument();
  expect(within(liabilitiesCol as HTMLElement).queryByText('Assets:Bank')).not.toBeInTheDocument();
});

test('default sort is descending by natural amount on both sides', async () => {
  render(makeTestApp('/balances'));

  // Assets: 125000 should come before 3500 (biggest first)
  const assetsHeader = await screen.findByText('Assets');
  const assetsCol = assetsHeader.closest('[class*="overflow-hidden"]') as HTMLElement;
  const assetsNames = within(assetsCol).getAllByText(/Assets:/);
  expect(assetsNames[0]).toHaveTextContent('Assets:Bank');
  expect(assetsNames[1]).toHaveTextContent('Assets:Cash');

  // Both Balance headers show ▼ by default
  const balanceButtons = await screen.findAllByRole('button', { name: /Balance/i });
  expect(balanceButtons).toHaveLength(2);
  for (const btn of balanceButtons) {
    expect(btn).toHaveTextContent('▼');
  }
});

test('toggling the Assets sort changes only the Assets arrow', async () => {
  render(makeTestApp('/balances'));

  const assetsHeader = await screen.findByText('Assets');
  const assetsCol = assetsHeader.closest('[class*="overflow-hidden"]') as HTMLElement;
  const assetsBtn = within(assetsCol).getByRole('button', { name: /Balance/i });
  const liabilitiesHeader = await screen.findByText('Liabilities');
  const liabilitiesCol = liabilitiesHeader.closest('[class*="overflow-hidden"]') as HTMLElement;
  const liabilitiesBtn = within(liabilitiesCol).getByRole('button', { name: /Balance/i });

  expect(assetsBtn).toHaveTextContent('▼');
  expect(liabilitiesBtn).toHaveTextContent('▼');

  await userEvent.click(assetsBtn);

  expect(assetsBtn).toHaveTextContent('▲');
  expect(liabilitiesBtn).toHaveTextContent('▼');
});

test('sorts liabilities by natural amount so biggest debt comes first by default', async () => {
  // Override the global fetch stub with multiple liabilities of different sizes.
  vi.stubGlobal(
    'fetch',
    vi.fn((url: string) => {
      if (url === '/api/config') {
        return Promise.resolve(okResponse({ defaults: { currency: 'USD' } }));
      }
      if (url === '/api/ledgers') {
        return Promise.resolve(
          okResponse({
            active: 'personal',
            items: [{ name: 'personal', path: '/p/personal.db', active: true }],
          }),
        );
      }
      if (url === '/api/balances') {
        return Promise.resolve(
          okResponse({
            items: [
              {
                account_id: 1,
                name: 'Liab:Small',
                type: 'L',
                currency: 'USD',
                amount: -1000, // smallest debt
                is_hidden: false,
              },
              {
                account_id: 2,
                name: 'Liab:Big',
                type: 'L',
                currency: 'USD',
                amount: -50000, // biggest debt
                is_hidden: false,
              },
            ],
            total_count: 2,
            limit: 0,
            offset: 0,
          }),
        );
      }
      throw new Error(`unexpected fetch: ${url}`);
    }),
  );

  render(makeTestApp('/balances'));

  const liabilitiesHeader = await screen.findByText('Liabilities');
  const liabilitiesCol = liabilitiesHeader.closest('[class*="overflow-hidden"]') as HTMLElement;
  const liabNames = within(liabilitiesCol).getAllByText(/Liab:/);
  // Biggest absolute debt first under natural-direction descending.
  expect(liabNames[0]).toHaveTextContent('Liab:Big');
  expect(liabNames[1]).toHaveTextContent('Liab:Small');
});

test('Assets pagination advances a_offset without touching the Liabilities column', async () => {
  // Build 10 assets (so pagination appears with 8-per-page) and 1 liability.
  const assetItems = Array.from({ length: 10 }, (_, i) => ({
    account_id: i + 1,
    name: `Assets:Acct${String(i + 1).padStart(2, '0')}`,
    type: 'A' as const,
    currency: 'USD',
    amount: (10 - i) * 1000, // descending so Acct01 is biggest
    is_hidden: false,
  }));
  const liabItem = {
    account_id: 100,
    name: 'Liab:Card',
    type: 'L' as const,
    currency: 'USD',
    amount: -4200,
    is_hidden: false,
  };

  vi.stubGlobal(
    'fetch',
    vi.fn((url: string) => {
      if (url === '/api/config') {
        return Promise.resolve(okResponse({ defaults: { currency: 'USD' } }));
      }
      if (url === '/api/ledgers') {
        return Promise.resolve(
          okResponse({
            active: 'personal',
            items: [{ name: 'personal', path: '/p/personal.db', active: true }],
          }),
        );
      }
      if (url === '/api/balances') {
        return Promise.resolve(
          okResponse({
            items: [...assetItems, liabItem],
            total_count: 11,
            limit: 0,
            offset: 0,
          }),
        );
      }
      throw new Error(`unexpected fetch: ${url}`);
    }),
  );

  render(makeTestApp('/balances'));

  // Page 1 shows the 8 biggest assets — Acct09 and Acct10 are off-screen.
  await screen.findByText('Assets:Acct01');
  expect(screen.queryByText('Assets:Acct09')).not.toBeInTheDocument();
  expect(screen.queryByText('Assets:Acct10')).not.toBeInTheDocument();

  // Liabilities side: 1 row, no pagination on that side.
  const liabilitiesHeader = screen.getByText('Liabilities');
  const liabilitiesCol = liabilitiesHeader.closest('[class*="overflow-hidden"]') as HTMLElement;
  expect(within(liabilitiesCol).getByText('Liab:Card')).toBeInTheDocument();
  expect(within(liabilitiesCol).queryByText(/Page \d+ of/)).not.toBeInTheDocument();

  // Click Next on the Assets pagination.
  const assetsHeader = screen.getByText('Assets');
  const assetsCol = assetsHeader.closest('[class*="overflow-hidden"]') as HTMLElement;
  const nextBtn = within(assetsCol).getByRole('button', { name: /Next/i });
  await userEvent.click(nextBtn);

  // Page 2 shows the smallest two assets.
  await screen.findByText('Assets:Acct09');
  expect(screen.getByText('Assets:Acct10')).toBeInTheDocument();
  expect(screen.queryByText('Assets:Acct01')).not.toBeInTheDocument();

  // Liabilities side unchanged.
  expect(within(liabilitiesCol).getByText('Liab:Card')).toBeInTheDocument();
});

test('default view is list', async () => {
  render(makeTestApp('/balances'));
  // List-mode subheader has Balance buttons (accessible name is just "Balance"
  // because the arrow span is aria-hidden) — one per populated column.
  const btns = await screen.findAllByRole('button', { name: /^Balance$/i });
  expect(btns.length).toBeGreaterThan(0);
  for (const btn of btns) {
    expect(btn).toHaveTextContent('▼');
  }
  // Cards-mode header sort button is absent.
  expect(screen.queryByRole('button', { name: /Sort by balance/i })).not.toBeInTheDocument();
});

test('clicking the cards toggle switches both columns to cards mode', async () => {
  render(makeTestApp('/balances'));
  await screen.findAllByRole('button', { name: /^Balance$/i });

  const cardsBtn = screen.getByRole('button', { name: /cards view/i });
  await userEvent.click(cardsBtn);

  // After switching: no list-mode subheader button.
  await waitFor(() => {
    expect(screen.queryByRole('button', { name: /^Balance$/i })).not.toBeInTheDocument();
  });
  // Cards-mode header sort buttons present on populated columns.
  expect(screen.getAllByRole('button', { name: /Sort by balance/i }).length).toBeGreaterThan(0);
});

test('switching view preserves a_offset and a_sort', async () => {
  render(makeTestApp('/balances?a_sort=balance_asc&view=list'));

  // Initially in list mode with asc sort on the Assets column — the visible
  // arrow becomes ▲.
  const assetsHeader = await screen.findByText('Assets');
  const assetsCol = assetsHeader.closest('[class*="overflow-hidden"]') as HTMLElement;
  const balanceBtn = within(assetsCol).getByRole('button', { name: /^Balance$/i });
  expect(balanceBtn).toHaveTextContent('▲');

  await userEvent.click(screen.getByRole('button', { name: /cards view/i }));

  // Cards-mode sort button on the Assets column still shows ascending.
  await waitFor(() => {
    const headerSort = within(assetsCol).getByRole('button', { name: /Sort by balance/i });
    expect(headerSort).toHaveTextContent('▲');
  });
});
