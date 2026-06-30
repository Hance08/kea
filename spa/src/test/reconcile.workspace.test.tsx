import { ApiError } from '@/lib/api';
import { fireEvent, render, screen, waitFor } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { makeTestApp } from './test-app';

const { getUnreconciledMock, commitReconcileMock } = vi.hoisted(() => ({
  getUnreconciledMock: vi.fn(),
  commitReconcileMock: vi.fn(),
}));

vi.mock('@/lib/accounts', async () => {
  const assetLeaf = {
    id: 3,
    name: 'Assets:Bank:Checking',
    type: 'A' as const,
    parent_id: 2,
    currency: 'USD',
    description: '',
    is_hidden: false,
  };
  const parent = {
    id: 2,
    name: 'Assets:Bank',
    type: 'A' as const,
    parent_id: 1,
    currency: 'USD',
    description: '',
    is_hidden: false,
  };
  const expenseLeaf = {
    id: 7,
    name: 'Expenses:Coffee',
    type: 'E' as const,
    currency: 'USD',
    description: '',
    is_hidden: false,
  };
  const tree = [
    {
      account: parent,
      children: [{ account: assetLeaf, children: [] }],
    },
    { account: expenseLeaf, children: [] },
  ];
  const { ApiError: RealApiError } = await vi.importActual<typeof import('@/lib/api')>('@/lib/api');
  return {
    ...(await vi.importActual<object>('@/lib/accounts')),
    getAccount: vi.fn(async (id: number) => {
      if (id === 3) return assetLeaf;
      if (id === 2) return parent;
      if (id === 7) return expenseLeaf;
      throw new RealApiError(404, 'not found');
    }),
    getAccountTree: vi.fn().mockResolvedValue(tree),
  };
});

vi.mock('@/lib/reconcile', async () => ({
  ...(await vi.importActual<object>('@/lib/reconcile')),
  getUnreconciled: (...args: unknown[]) => getUnreconciledMock(...args),
  commitReconcile: (...args: unknown[]) => commitReconcileMock(...args),
}));

vi.mock('@/lib/api', async () => ({
  ...(await vi.importActual<object>('@/lib/api')),
  getConfig: vi
    .fn()
    .mockResolvedValue({ defaults: { currency: 'USD' }, display: { hide_decimals: false } }),
  getBalances: vi.fn().mockResolvedValue({ items: [], total_count: 0, limit: 0, offset: 0 }),
}));

beforeEach(() => {
  getUnreconciledMock.mockReset();
  commitReconcileMock.mockReset();
  getUnreconciledMock.mockResolvedValue({
    entries: [
      {
        id: 18,
        timestamp: 1735689600,
        description: 'Bluebottle',
        status: 'Cleared',
        amount: -450,
        offset_account: 'Expenses:Coffee',
      },
      {
        id: 19,
        timestamp: 1735776000,
        description: 'Paycheck',
        status: 'Cleared',
        amount: 500000,
        offset_account: 'Income:Salary',
      },
    ],
    last_reconciled_balance: 180000,
  });
});

afterEach(() => {
  vi.clearAllMocks();
});

describe('/reconcile/$id workspace', () => {
  it('renders header and table from the API', async () => {
    render(makeTestApp('/reconcile/3'));
    await waitFor(() => expect(screen.getByText('Assets:Bank:Checking')).toBeInTheDocument());
    expect(screen.getByText(/2 unreconciled/)).toBeInTheDocument();
    expect(screen.getByText('Bluebottle')).toBeInTheDocument();
    expect(screen.getByText('Paycheck')).toBeInTheDocument();
    expect(screen.getByText('$1,800.00')).toBeInTheDocument(); // last reconciled
  });

  it('disables Reconcile when nothing is selected', async () => {
    render(makeTestApp('/reconcile/3'));
    await waitFor(() => expect(screen.getByText('Bluebottle')).toBeInTheDocument());
    const stmt = screen.getByLabelText(/statement/i);
    fireEvent.change(stmt, { target: { value: '2545.50' } });
    expect(screen.getByRole('button', { name: /^reconcile/i })).toBeDisabled();
  });

  it('updates cleared and difference live as the user selects', async () => {
    const user = userEvent.setup();
    render(makeTestApp('/reconcile/3'));
    await waitFor(() => expect(screen.getByText('Bluebottle')).toBeInTheDocument());

    fireEvent.change(screen.getByLabelText(/statement/i), { target: { value: '6795.50' } });
    await user.click(screen.getAllByRole('checkbox')[0]); // -450
    await user.click(screen.getAllByRole('checkbox')[1]); // 500000

    expect(screen.getByText('$4,995.50')).toBeInTheDocument(); // cleared
    expect(screen.getByText(/Balanced/)).toBeInTheDocument();
  });

  it('commits with allow_mismatch:false on the happy path', async () => {
    commitReconcileMock.mockResolvedValueOnce({
      reconciled_count: 2,
      difference: 0,
      last_reconciled_balance: 679550,
    });
    const user = userEvent.setup();
    render(makeTestApp('/reconcile/3'));
    await waitFor(() => expect(screen.getByText('Bluebottle')).toBeInTheDocument());

    fireEvent.change(screen.getByLabelText(/statement/i), { target: { value: '6795.50' } });
    await user.click(screen.getAllByRole('checkbox')[0]);
    await user.click(screen.getAllByRole('checkbox')[1]);
    await user.click(screen.getByRole('button', { name: /^reconcile/i }));

    await waitFor(() => expect(commitReconcileMock).toHaveBeenCalledTimes(1));
    expect(commitReconcileMock).toHaveBeenCalledWith(3, 679550, [18, 19], false);
  });

  it('opens the mismatch dialog when the server returns 409', async () => {
    commitReconcileMock.mockRejectedValueOnce(
      new ApiError(409, 'statement balance off by 1000', undefined, 1000),
    );
    const user = userEvent.setup();
    render(makeTestApp('/reconcile/3'));
    await waitFor(() => expect(screen.getByText('Bluebottle')).toBeInTheDocument());

    fireEvent.change(screen.getByLabelText(/statement/i), { target: { value: '6885.50' } });
    await user.click(screen.getAllByRole('checkbox')[0]);
    await user.click(screen.getAllByRole('checkbox')[1]);
    await user.click(screen.getByRole('button', { name: /^reconcile/i }));

    await waitFor(() => expect(screen.getByRole('alertdialog')).toBeInTheDocument());
    expect(screen.getByText(/short \$10\.00/i)).toBeInTheDocument();
  });

  it('re-fires commit with allow_mismatch:true when user confirms mismatch', async () => {
    commitReconcileMock
      .mockRejectedValueOnce(new ApiError(409, 'off by 1000', undefined, 1000))
      .mockResolvedValueOnce({
        reconciled_count: 2,
        difference: 1000,
        last_reconciled_balance: 678550,
      });
    const user = userEvent.setup();
    render(makeTestApp('/reconcile/3'));
    await waitFor(() => expect(screen.getByText('Bluebottle')).toBeInTheDocument());

    fireEvent.change(screen.getByLabelText(/statement/i), { target: { value: '6885.50' } });
    await user.click(screen.getAllByRole('checkbox')[0]);
    await user.click(screen.getAllByRole('checkbox')[1]);
    await user.click(screen.getByRole('button', { name: /^reconcile/i }));
    await waitFor(() => screen.getByRole('alertdialog'));
    await user.click(screen.getByRole('button', { name: /reconcile anyway/i }));

    await waitFor(() => expect(commitReconcileMock).toHaveBeenCalledTimes(2));
    expect(commitReconcileMock.mock.calls[1]).toEqual([3, 688550, [18, 19], true]);
  });

  it('cancelling the mismatch dialog does not re-fire commit', async () => {
    commitReconcileMock.mockRejectedValueOnce(new ApiError(409, 'off by 1000', undefined, 1000));
    const user = userEvent.setup();
    render(makeTestApp('/reconcile/3'));
    await waitFor(() => expect(screen.getByText('Bluebottle')).toBeInTheDocument());

    fireEvent.change(screen.getByLabelText(/statement/i), { target: { value: '6885.50' } });
    await user.click(screen.getAllByRole('checkbox')[0]);
    await user.click(screen.getAllByRole('checkbox')[1]);
    await user.click(screen.getByRole('button', { name: /^reconcile/i }));
    await waitFor(() => screen.getByRole('alertdialog'));
    await user.click(screen.getByRole('button', { name: /cancel/i }));

    await waitFor(() => expect(screen.queryByRole('alertdialog')).not.toBeInTheDocument());
    expect(commitReconcileMock).toHaveBeenCalledTimes(1);
  });

  it('shows alert when account type is not A/L', async () => {
    render(makeTestApp('/reconcile/7'));
    await waitFor(() =>
      expect(
        screen.getByText(/reconcile only applies to asset and liability/i),
      ).toBeInTheDocument(),
    );
  });

  it('shows alert when account is a parent', async () => {
    render(makeTestApp('/reconcile/2'));
    await waitFor(() =>
      expect(screen.getByText(/has child accounts/i)).toBeInTheDocument(),
    );
  });

  it('shows empty-state row when there are no unreconciled transactions', async () => {
    getUnreconciledMock.mockResolvedValueOnce({ entries: [], last_reconciled_balance: 180000 });
    render(makeTestApp('/reconcile/3'));
    await waitFor(() =>
      expect(screen.getByText(/no unreconciled transactions/i)).toBeInTheDocument(),
    );
  });

  it('uses snapshot inputs on mismatch re-fire even if user edits form during dialog', async () => {
    commitReconcileMock
      .mockRejectedValueOnce(new ApiError(409, 'off by 1000', undefined, 1000))
      .mockResolvedValueOnce({
        reconciled_count: 2,
        difference: 1000,
        last_reconciled_balance: 678550,
      });
    const user = userEvent.setup();
    render(makeTestApp('/reconcile/3'));
    await waitFor(() => expect(screen.getByText('Bluebottle')).toBeInTheDocument());

    fireEvent.change(screen.getByLabelText(/statement/i), { target: { value: '6885.50' } });
    await user.click(screen.getAllByRole('checkbox')[0]);
    await user.click(screen.getAllByRole('checkbox')[1]);
    await user.click(screen.getByRole('button', { name: /^reconcile/i }));
    await waitFor(() => screen.getByRole('alertdialog'));

    // User edits the form while the dialog is open — should NOT affect the re-fire payload.
    fireEvent.change(screen.getByLabelText(/^statement$/i), { target: { value: '9999.99' } });
    await user.click(screen.getAllByRole('checkbox')[0]); // untoggle

    await user.click(screen.getByRole('button', { name: /reconcile anyway/i }));

    await waitFor(() => expect(commitReconcileMock).toHaveBeenCalledTimes(2));
    // Snapshot used the original 688550 cents + [18, 19], not the edited 999999 + [19].
    expect(commitReconcileMock.mock.calls[1]).toEqual([3, 688550, [18, 19], true]);
  });

  it('footer "Back" link navigates to /reconcile (not history-back)', async () => {
    render(makeTestApp('/reconcile/3'));
    await waitFor(() => expect(screen.getByText('Bluebottle')).toBeInTheDocument());
    const back = screen.getByRole('link', { name: /^back$/i });
    expect(back).toHaveAttribute('href', '/reconcile');
  });
});
