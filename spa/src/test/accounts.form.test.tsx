import { render, screen, waitFor } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { describe, expect, it, vi } from 'vitest';
import { makeTestApp } from './test-app';

const createAccount = vi.hoisted(() =>
  vi.fn().mockResolvedValue({
    id: 42,
    name: 'Assets:NewBank',
    type: 'A',
    currency: 'USD',
    description: '',
    is_hidden: false,
  }),
);

vi.mock('@/lib/accounts', async () => ({
  ...(await vi.importActual<object>('@/lib/accounts')),
  createAccount,
  getAccountTree: vi.fn().mockResolvedValue([]),
  searchAccounts: vi.fn().mockResolvedValue({
    items: [],
    total_count: 0,
    limit: 20,
    offset: 0,
  }),
}));

vi.mock('@/lib/api', async () => ({
  ...(await vi.importActual<object>('@/lib/api')),
  getConfig: vi.fn().mockResolvedValue({ defaults: { currency: 'USD' } }),
  getBalances: vi.fn().mockResolvedValue({
    items: [],
    total_count: 0,
    limit: 0,
    offset: 0,
  }),
}));

vi.mock('@/lib/transactions', async () => ({
  ...(await vi.importActual<object>('@/lib/transactions')),
  listTransactions: vi.fn().mockResolvedValue({
    items: [],
    total_count: 0,
    limit: 20,
    offset: 0,
  }),
}));

describe('accounts create form', () => {
  it('submits a top-level account with type/currency/name', async () => {
    render(makeTestApp('/accounts/new'));

    await waitFor(() => expect(screen.getByText(/new account/i)).toBeInTheDocument());

    await userEvent.click(screen.getByRole('button', { name: /asset/i }));
    await userEvent.type(screen.getByLabelText(/^name$/i), 'NewBank');
    await userEvent.click(screen.getByRole('button', { name: /create/i }));

    await waitFor(() =>
      expect(createAccount).toHaveBeenCalledWith(
        expect.objectContaining({ name: 'NewBank', type: 'A', currency: 'USD' }),
      ),
    );
  });

  it('rejects names containing a colon', async () => {
    render(makeTestApp('/accounts/new'));

    await waitFor(() => expect(screen.getByText(/new account/i)).toBeInTheDocument());

    await userEvent.click(screen.getByRole('button', { name: /asset/i }));
    await userEvent.type(screen.getByLabelText(/^name$/i), 'Assets:Bank');

    expect(screen.getByText(/should not contain/i)).toBeInTheDocument();
    expect(screen.getByRole('button', { name: /create/i })).toBeDisabled();
  });
});
