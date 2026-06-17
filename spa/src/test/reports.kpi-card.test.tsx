import { render as rtlRender, screen } from '@testing-library/react';
import type { ReactNode } from 'react';
import { expect, test } from 'vitest';
import { KpiCard } from '../components/reports/KpiCard';
import { withServerConfig } from './test-app';

const render = (ui: ReactNode) => rtlRender(withServerConfig(ui));

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

test('diff: positive delta with goodWhen=up renders ▲, green color, amount, and percent', () => {
  const { container } = render(
    <KpiCard
      label="Income"
      amount={524800}
      currency="USD"
      variant="green"
      diff={{ delta: 120000, prevAmount: 404800, goodWhen: 'up' }}
    />,
  );
  const diff = container.querySelector('[data-testid="kpi-diff"]');
  expect(diff).not.toBeNull();
  expect(diff?.textContent).toContain('▲');
  expect(diff?.textContent).toContain('+$1,200.00');
  expect(diff?.textContent).toContain('+29.6%');
  expect(diff?.textContent).toContain('vs last period');
  expect(diff?.className).toContain('text-green');
});

test('diff: positive delta with goodWhen=down renders red (e.g. expense grew)', () => {
  const { container } = render(
    <KpiCard
      label="Expense"
      amount={253000}
      currency="USD"
      variant="red"
      diff={{ delta: 30000, prevAmount: 223000, goodWhen: 'down' }}
    />,
  );
  const diff = container.querySelector('[data-testid="kpi-diff"]');
  expect(diff?.className).toContain('text-red');
  expect(diff?.textContent).toContain('▲');
  expect(diff?.textContent).toContain('+$300.00');
});

test('diff: negative delta with goodWhen=up renders ▼ and red', () => {
  const { container } = render(
    <KpiCard
      label="Income"
      amount={400000}
      currency="USD"
      variant="green"
      diff={{ delta: -50000, prevAmount: 450000, goodWhen: 'up' }}
    />,
  );
  const diff = container.querySelector('[data-testid="kpi-diff"]');
  expect(diff?.className).toContain('text-red');
  expect(diff?.textContent).toContain('▼');
  expect(diff?.textContent).toContain('-$500.00');
  expect(diff?.textContent).toContain('-11.1%');
});

test('diff: zero delta renders em-dash and muted color, no percent', () => {
  const { container } = render(
    <KpiCard
      label="Income"
      amount={100000}
      currency="USD"
      variant="green"
      diff={{ delta: 0, prevAmount: 100000, goodWhen: 'up' }}
    />,
  );
  const diff = container.querySelector('[data-testid="kpi-diff"]');
  expect(diff?.textContent).toContain('—');
  expect(diff?.className).toContain('text-muted-foreground');
  expect(diff?.textContent).not.toMatch(/%/);
});

test('diff: omits percent when prevAmount is 0', () => {
  const { container } = render(
    <KpiCard
      label="Income"
      amount={500}
      currency="USD"
      variant="green"
      diff={{ delta: 500, prevAmount: 0, goodWhen: 'up' }}
    />,
  );
  const diff = container.querySelector('[data-testid="kpi-diff"]');
  expect(diff?.textContent).toContain('+$5.00');
  expect(diff?.textContent).not.toMatch(/%/);
});

test('diff: coexists with subLine — both render', () => {
  const { container } = render(
    <KpiCard
      label="Net"
      amount={271800}
      currency="USD"
      variant="neutral"
      subLine="▲ 6.2% net worth"
      diff={{ delta: 10000, prevAmount: 261800, goodWhen: 'up' }}
    />,
  );
  expect(container.querySelector('[data-testid="kpi-subline"]')).not.toBeNull();
  expect(container.querySelector('[data-testid="kpi-diff"]')).not.toBeNull();
});

test('diff: no kpi-diff element when diff prop omitted', () => {
  const { container } = render(
    <KpiCard label="Income" amount={524800} currency="USD" variant="green" />,
  );
  expect(container.querySelector('[data-testid="kpi-diff"]')).toBeNull();
});
