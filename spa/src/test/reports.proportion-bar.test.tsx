import { render, screen } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { expect, test } from 'vitest';
import { ProportionBar } from '../components/reports/ProportionBar';
import type { ReportRow } from '../lib/types';

const rows: ReportRow[] = [
  { account_name: 'Salary', offset_account: '', amount: 520000, currency: 'USD', tx_count: 1 },
  { account_name: 'Interest', offset_account: '', amount: 4800, currency: 'USD', tx_count: 2 },
];

test('renders one bar per row up to limit', () => {
  const many: ReportRow[] = Array.from({ length: 10 }, (_, i) => ({
    account_name: `Row${i}`,
    offset_account: '',
    amount: 100 * (10 - i),
    currency: 'USD',
    tx_count: 1,
  }));
  const { container } = render(<ProportionBar rows={many} total={5500} currency="USD" limit={5} />);
  expect(container.querySelectorAll('[data-testid="prop-bar"]').length).toBe(5);
});

test('shows expand button when rows exceed limit', () => {
  const many: ReportRow[] = Array.from({ length: 10 }, (_, i) => ({
    account_name: `Row${i}`,
    offset_account: '',
    amount: 100,
    currency: 'USD',
    tx_count: 1,
  }));
  render(<ProportionBar rows={many} total={1000} currency="USD" limit={5} />);
  expect(screen.getByRole('button', { name: /5 more/i })).toBeInTheDocument();
});

test('clicking expand renders all rows', async () => {
  const many: ReportRow[] = Array.from({ length: 10 }, (_, i) => ({
    account_name: `Row${i}`,
    offset_account: '',
    amount: 100,
    currency: 'USD',
    tx_count: 1,
  }));
  const { container } = render(<ProportionBar rows={many} total={1000} currency="USD" limit={5} />);
  await userEvent.click(screen.getByRole('button', { name: /5 more/i }));
  expect(container.querySelectorAll('[data-testid="prop-bar"]').length).toBe(10);
});

test('bar width is proportional to amount/total', () => {
  const { container } = render(<ProportionBar rows={rows} total={520000} currency="USD" />);
  const bars = container.querySelectorAll('[data-testid="prop-bar"] [data-testid="bar-fill"]');
  expect(bars[0].getAttribute('style')).toContain('width: 100%');
  // 4800/520000 ≈ 0.92%
  expect(bars[1].getAttribute('style')).toMatch(/width: 0\.\d+%|width: 1%/);
});

test('renders empty state when no rows', () => {
  render(<ProportionBar rows={[]} total={0} currency="USD" />);
  expect(screen.getByText(/no data/i)).toBeInTheDocument();
});

test('handles total=0 without dividing by zero', () => {
  const r: ReportRow[] = [
    { account_name: 'A', offset_account: '', amount: 0, currency: 'USD', tx_count: 1 },
  ];
  const { container } = render(<ProportionBar rows={r} total={0} currency="USD" />);
  const fill = container.querySelector('[data-testid="bar-fill"]');
  expect(fill?.getAttribute('style')).toContain('width: 0%');
});

test('uses absolute values for negative amounts', () => {
  const r: ReportRow[] = [
    { account_name: 'Refund', offset_account: '', amount: -10000, currency: 'USD', tx_count: 1 },
    { account_name: 'Other', offset_account: '', amount: 10000, currency: 'USD', tx_count: 1 },
  ];
  const { container } = render(<ProportionBar rows={r} total={-20000} currency="USD" />);
  const bars = container.querySelectorAll('[data-testid="bar-fill"]');
  expect(bars[0].getAttribute('style')).toContain('width: 50%');
  expect(bars[1].getAttribute('style')).toContain('width: 50%');
});
