import { UnreconciledTable } from '@/components/reconcile/UnreconciledTable';
import type { ReconcileEntry } from '@/lib/types';
import { fireEvent, render, screen } from '@testing-library/react';
import { describe, expect, it, vi } from 'vitest';
import { withServerConfig } from '../test-app';

const sample: ReconcileEntry[] = [
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
];

describe('UnreconciledTable', () => {
  it('renders one row per entry with description, offset, and amount', () => {
    render(
      withServerConfig(
        <UnreconciledTable entries={sample} selectedIds={new Set()} onToggle={vi.fn()} />,
      ),
    );
    expect(screen.getByText('Bluebottle')).toBeInTheDocument();
    expect(screen.getByText('Expenses:Coffee')).toBeInTheDocument();
    expect(screen.getByText('Paycheck')).toBeInTheDocument();
    expect(screen.getByText('Income:Salary')).toBeInTheDocument();
  });

  it('renders the absolute amount (sign implied by color)', () => {
    render(
      withServerConfig(
        <UnreconciledTable entries={sample} selectedIds={new Set()} onToggle={vi.fn()} />,
      ),
    );
    // -450 cents → "$4.50" (abs), not "-$4.50"
    expect(screen.getByText('$4.50')).toBeInTheDocument();
    expect(screen.getByText('$5,000.00')).toBeInTheDocument();
  });

  it('applies red class to negative amount cells', () => {
    render(
      withServerConfig(
        <UnreconciledTable entries={sample} selectedIds={new Set()} onToggle={vi.fn()} />,
      ),
    );
    const negCell = screen.getByText('$4.50');
    expect(negCell.className).toContain('text-red');
  });

  it('applies green class to positive amount cells', () => {
    render(
      withServerConfig(
        <UnreconciledTable entries={sample} selectedIds={new Set()} onToggle={vi.fn()} />,
      ),
    );
    const posCell = screen.getByText('$5,000.00');
    expect(posCell.className).toContain('text-green');
  });

  it('checks the box for ids in selectedIds', () => {
    render(
      withServerConfig(
        <UnreconciledTable
          entries={sample}
          selectedIds={new Set([18])}
          onToggle={vi.fn()}
        />,
      ),
    );
    const boxes = screen.getAllByRole('checkbox');
    expect((boxes[0] as HTMLInputElement).checked).toBe(true);
    expect((boxes[1] as HTMLInputElement).checked).toBe(false);
  });

  it('fires onToggle(id) when a checkbox is clicked', () => {
    const onToggle = vi.fn();
    render(
      withServerConfig(
        <UnreconciledTable entries={sample} selectedIds={new Set()} onToggle={onToggle} />,
      ),
    );
    fireEvent.click(screen.getAllByRole('checkbox')[1]);
    expect(onToggle).toHaveBeenCalledWith(19);
  });

  it('shows an empty-state row when entries is empty', () => {
    render(
      withServerConfig(
        <UnreconciledTable entries={[]} selectedIds={new Set()} onToggle={vi.fn()} />,
      ),
    );
    expect(screen.getByText(/No unreconciled transactions/i)).toBeInTheDocument();
  });
});
