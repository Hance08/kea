import { render, screen } from '@testing-library/react';
import { expect, test } from 'vitest';
import { CurrencyFooter } from '../components/reports/CurrencyFooter';

test('renders nothing when only the default currency is present', () => {
  const { container } = render(
    <CurrencyFooter
      defaultCurrency="USD"
      entries={[{ label: 'Income', byCurrency: { USD: 524800 } }]}
    />,
  );
  expect(container.firstChild).toBeNull();
});

test('renders non-default currency lines', () => {
  render(
    <CurrencyFooter
      defaultCurrency="USD"
      entries={[
        { label: 'Income', byCurrency: { USD: 524800, JPY: 12000000 } },
        { label: 'Expense', byCurrency: { USD: 253000, JPY: 4000000 } },
      ]}
    />,
  );
  expect(screen.getByText(/Income/)).toBeInTheDocument();
  expect(screen.getByText(/Expense/)).toBeInTheDocument();
  // JPY values appear, USD does not duplicate
  expect(screen.queryAllByText(/JPY|¥/).length).toBeGreaterThan(0);
});

test('skips entries whose only currency is the default', () => {
  const { container } = render(
    <CurrencyFooter
      defaultCurrency="USD"
      entries={[
        { label: 'Income', byCurrency: { USD: 1 } },
        { label: 'Expense', byCurrency: { USD: 1, JPY: 1 } },
      ]}
    />,
  );
  // Only one line: Expense (Income's only currency is the default)
  expect(container.querySelectorAll('[data-testid="cur-footer-line"]').length).toBe(1);
});
