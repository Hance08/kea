import { ReconcileHeader } from '@/components/reconcile/ReconcileHeader';
import { fireEvent, render, screen } from '@testing-library/react';
import { describe, expect, it, vi } from 'vitest';
import { withServerConfig } from '../test-app';

function renderHeader(overrides: Partial<React.ComponentProps<typeof ReconcileHeader>> = {}) {
  const props = {
    accountName: 'Assets:Checking',
    currency: 'USD',
    unreconciledCount: 12,
    statement: '',
    onStatementChange: vi.fn(),
    statementInvalid: false,
    lastReconciledCents: 180000,
    clearedCents: 0,
    differenceCents: null,
    ...overrides,
  };
  render(withServerConfig(<ReconcileHeader {...props} />));
  return props;
}

describe('ReconcileHeader', () => {
  it('renders the account name, currency, and unreconciled count', () => {
    renderHeader();
    expect(screen.getByText('Assets:Checking')).toBeInTheDocument();
    expect(screen.getByText(/USD/)).toBeInTheDocument();
    expect(screen.getByText(/12 unreconciled/)).toBeInTheDocument();
  });

  it('shows last reconciled and cleared as formatted dollars', () => {
    renderHeader({ clearedCents: 64000 });
    expect(screen.getByText('$1,800.00')).toBeInTheDocument();
    expect(screen.getByText('$640.00')).toBeInTheDocument();
  });

  it('shows the em-dash for difference when statement is empty', () => {
    renderHeader();
    expect(screen.getByText('—')).toBeInTheDocument();
  });

  it('renders "Balanced" in green when diff is zero', () => {
    renderHeader({ statement: '2450', differenceCents: 0 });
    const el = screen.getByText('Balanced');
    expect(el.className).toContain('text-green');
  });

  it('renders "Short $10.00" in red when diff > 0', () => {
    renderHeader({ statement: '2450', differenceCents: 1000 });
    const el = screen.getByText(/Short \$10\.00/);
    expect(el.className).toContain('text-red');
  });

  it('renders "Over $10.00" in amber when diff < 0', () => {
    renderHeader({ statement: '2450', differenceCents: -1000 });
    const el = screen.getByText(/Over \$10\.00/);
    expect(el.className).toMatch(/text-(amber|orange)/);
  });

  it('marks the statement input aria-invalid when statementInvalid is true', () => {
    renderHeader({ statement: 'abc', statementInvalid: true });
    const input = screen.getByLabelText(/statement/i);
    expect(input).toHaveAttribute('aria-invalid', 'true');
  });

  it('fires onStatementChange when the input changes', () => {
    const props = renderHeader();
    const input = screen.getByLabelText(/statement/i);
    fireEvent.change(input, { target: { value: '2450' } });
    expect(props.onStatementChange).toHaveBeenCalledWith('2450');
  });
});
