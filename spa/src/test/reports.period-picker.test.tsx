import { fireEvent, render, screen } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { expect, test } from 'vitest';
import { PeriodPicker } from '../components/reports/PeriodPicker';
import type { PeriodSearchParams } from '../lib/reports-search-params';

test('renders all preset chips', () => {
  const noop = (_: Partial<PeriodSearchParams>) => {};
  render(<PeriodPicker value={{ range: 'this-month' }} onChange={noop} label="June 2026" />);
  expect(screen.getByRole('button', { name: /this month/i })).toBeInTheDocument();
  expect(screen.getByRole('button', { name: /last month/i })).toBeInTheDocument();
  expect(screen.getByRole('button', { name: /ytd/i })).toBeInTheDocument();
  expect(screen.getByRole('button', { name: /last 12 months/i })).toBeInTheDocument();
  expect(screen.getByRole('button', { name: /custom/i })).toBeInTheDocument();
});

test('clicking a chip calls onChange with new range', async () => {
  const calls: Partial<PeriodSearchParams>[] = [];
  render(
    <PeriodPicker
      value={{ range: 'this-month' }}
      onChange={(v) => calls.push(v)}
      label="June 2026"
    />,
  );
  await userEvent.click(screen.getByRole('button', { name: /ytd/i }));
  expect(calls).toEqual([{ range: 'ytd' }]);
});

test('active chip has aria-pressed=true', () => {
  const noop = (_: Partial<PeriodSearchParams>) => {};
  render(<PeriodPicker value={{ range: 'ytd' }} onChange={noop} label="YTD 2026" />);
  expect(screen.getByRole('button', { name: /ytd/i }).getAttribute('aria-pressed')).toBe('true');
  expect(screen.getByRole('button', { name: /this month/i }).getAttribute('aria-pressed')).toBe(
    'false',
  );
});

test('custom mode shows from/to inputs and emits change', async () => {
  const calls: Partial<PeriodSearchParams>[] = [];
  render(
    <PeriodPicker
      value={{ range: 'custom', from: '2026-01-01', to: '2026-01-31' }}
      onChange={(v) => calls.push(v)}
      label="Jan 1 – Jan 31, 2026"
    />,
  );
  const fromInput = screen.getByLabelText(/from/i) as HTMLInputElement;
  const toInput = screen.getByLabelText(/to/i) as HTMLInputElement;
  expect(fromInput.value).toBe('2026-01-01');
  expect(toInput.value).toBe('2026-01-31');
  fireEvent.change(fromInput, { target: { value: '2026-02-01' } });
  // Assert at least one custom call carries the new from
  expect(calls.some((c) => c.range === 'custom' && c.from === '2026-02-01')).toBe(true);
});

test('renders human-readable label', () => {
  const noop = (_: Partial<PeriodSearchParams>) => {};
  render(<PeriodPicker value={{ range: 'this-month' }} onChange={noop} label="June 2026" />);
  expect(screen.getByText('June 2026')).toBeInTheDocument();
});
