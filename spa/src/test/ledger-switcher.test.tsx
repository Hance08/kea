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

test('failed switch shows error toast with API message', async () => {
  // Replace the default fetch stub for this test: make /switch fail.
  vi.stubGlobal(
    'fetch',
    vi.fn((url: string) => {
      if (url === '/api/ledgers') {
        return Promise.resolve(okResponse(LEDGERS_OK));
      }
      if (url === '/api/ledgers/switch') {
        return Promise.resolve(errorResponse(404, { message: 'ledger not found: "business"' }));
      }
      throw new Error(`unexpected fetch: ${url}`);
    }),
  );

  renderSwitcher();
  await userEvent.click(await screen.findByRole('button', { name: /personal/i }));
  await userEvent.click(await screen.findByRole('menuitem', { name: /business/i }));

  await vi.waitFor(() => {
    expect(toastError).toHaveBeenCalledWith('ledger not found: "business"');
  });
});

test('while switch pending, other menu items are disabled', async () => {
  // Build a fetch stub where /switch returns a Promise we can resolve manually.
  let resolveSwitch: (value: Response) => void = () => {
    /* assigned below */
  };
  const switchPromise = new Promise<Response>((res) => {
    resolveSwitch = res;
  });

  const threeLedgers = {
    active: 'personal',
    items: [
      { name: 'personal', path: '/p/personal.db', active: true },
      { name: 'business', path: '/p/business.db', active: false },
      { name: 'savings', path: '/p/savings.db', active: false },
    ],
  };

  vi.stubGlobal(
    'fetch',
    vi.fn((url: string) => {
      if (url === '/api/ledgers') {
        return Promise.resolve(okResponse(threeLedgers));
      }
      if (url === '/api/ledgers/switch') {
        return switchPromise;
      }
      throw new Error(`unexpected fetch: ${url}`);
    }),
  );

  renderSwitcher();
  await userEvent.click(await screen.findByRole('button', { name: /personal/i }));
  await userEvent.click(await screen.findByRole('menuitem', { name: /business/i }));

  // While the switch is in-flight, the third (other) row should be disabled.
  const savings = await screen.findByRole('menuitem', { name: /savings/i });
  expect(savings).toHaveAttribute('data-disabled');

  // Resolve to let the test tear down cleanly.
  resolveSwitch(okResponse({ name: 'business', path: '/p/business.db', active: true }));
});
