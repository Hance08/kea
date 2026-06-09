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

const errorResponse = (status: number, body: unknown) =>
  new Response(JSON.stringify(body), {
    status,
    headers: { 'Content-Type': 'application/json' },
  });
// used in Task 7; referenced here so TS noUnusedLocals is satisfied
void errorResponse;

const LEDGERS_OK = {
  active: 'personal',
  items: [
    { name: 'personal', path: '/p/personal.db', active: true },
    { name: 'business', path: '/p/business.db', active: false },
  ],
};

// vi.mock factories are hoisted above top-level `const`s, so referencing
// outer `vi.fn()` values directly throws. Use vi.hoisted to share state safely.
const { toastSuccess, toastError } = vi.hoisted(() => ({
  toastSuccess: vi.fn(),
  toastError: vi.fn(),
}));
vi.mock('sonner', () => ({
  toast: {
    success: (...args: unknown[]) => toastSuccess(...args),
    error: (...args: unknown[]) => toastError(...args),
  },
}));

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

let fetchCalls: Array<{ url: string; init?: RequestInit }>;

beforeEach(() => {
  fetchCalls = [];
  toastSuccess.mockReset();
  toastError.mockReset();
  vi.stubGlobal(
    'fetch',
    vi.fn((url: string, init?: RequestInit) => {
      fetchCalls.push({ url, init });
      if (url === '/api/ledgers') {
        return Promise.resolve(okResponse(LEDGERS_OK));
      }
      if (url === '/api/ledgers/switch') {
        return Promise.resolve(
          okResponse({ name: 'business', path: '/p/business.db', active: true }),
        );
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

test('clicking non-active row POSTs /api/ledgers/switch and shows success toast', async () => {
  renderSwitcher();
  const trigger = await screen.findByRole('button', { name: /personal/i });
  await userEvent.click(trigger);

  const business = await screen.findByRole('menuitem', { name: /business/i });
  await userEvent.click(business);

  const switchCall = fetchCalls.find((c) => c.url === '/api/ledgers/switch');
  expect(switchCall).toBeDefined();
  expect(switchCall?.init?.method).toBe('POST');
  expect(switchCall?.init?.body).toBe(JSON.stringify({ name: 'business' }));

  await vi.waitFor(() => {
    expect(toastSuccess).toHaveBeenCalledWith('Switched to business');
  });
});
