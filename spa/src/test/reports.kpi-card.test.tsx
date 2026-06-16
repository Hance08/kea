import { render, screen } from '@testing-library/react';
import { expect, test } from 'vitest';
import { KpiCard } from '../components/reports/KpiCard';

test('renders label, formatted amount, and currency', () => {
  render(<KpiCard label="Income" amount={524800} currency="USD" variant="green" />);
  expect(screen.getByText('Income')).toBeInTheDocument();
  expect(screen.getByText('$5,248.00')).toBeInTheDocument();
});

test('applies the green variant class', () => {
  const { container } = render(
    <KpiCard label="Income" amount={524800} currency="USD" variant="green" />,
  );
  const amountEl = container.querySelector('[data-testid="kpi-amount"]');
  expect(amountEl?.className).toContain('text-green');
});

test('applies the red variant class', () => {
  const { container } = render(
    <KpiCard label="Expense" amount={253000} currency="USD" variant="red" />,
  );
  const amountEl = container.querySelector('[data-testid="kpi-amount"]');
  expect(amountEl?.className).toContain('text-red');
});

test('renders sub-line when provided', () => {
  render(
    <KpiCard
      label="Net"
      amount={271800}
      currency="USD"
      variant="neutral"
      subLine="▲ 6.2% net worth"
    />,
  );
  expect(screen.getByText('▲ 6.2% net worth')).toBeInTheDocument();
});

test('omits sub-line when not provided', () => {
  const { container } = render(
    <KpiCard label="Net" amount={271800} currency="USD" variant="neutral" />,
  );
  expect(container.querySelector('[data-testid="kpi-subline"]')).toBeNull();
});
