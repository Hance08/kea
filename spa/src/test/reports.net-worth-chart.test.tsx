import { NetWorthChart } from '@/components/reports/NetWorthChart';
import { render, screen } from '@testing-library/react';
import { describe, expect, test } from 'vitest';

const formatUSD = (cents: number) =>
  `$${(cents / 100).toLocaleString(undefined, { minimumFractionDigits: 2, maximumFractionDigits: 2 })}`;

describe('NetWorthChart', () => {
  test('renders nothing when fewer than 2 points', () => {
    const { container } = render(
      <NetWorthChart
        points={[{ date: '2026-01-01', balance: 100 }]}
        currency="USD"
        formatCents={formatUSD}
      />,
    );
    expect(container.querySelector('svg')).toBeNull();
  });

  test('renders an svg with polyline and min/max axis labels', () => {
    const points = [
      { date: '2026-01-01', balance: 100_00 },
      { date: '2026-01-02', balance: 150_00 },
      { date: '2026-01-03', balance: 200_00 },
    ];
    render(<NetWorthChart points={points} currency="USD" formatCents={formatUSD} />);

    const svg = screen.getByRole('img', { name: /net worth/i });
    expect(svg).toBeInTheDocument();
    expect(svg.querySelector('polyline')).not.toBeNull();
    expect(screen.getByText(formatUSD(100_00))).toBeInTheDocument();
    expect(screen.getByText(formatUSD(200_00))).toBeInTheDocument();
    expect(screen.getByText('2026-01-01')).toBeInTheDocument();
    expect(screen.getByText('2026-01-03')).toBeInTheDocument();
  });

  test('renders a marker circle at the asOfDate', () => {
    const points = [
      { date: '2026-01-01', balance: 100 },
      { date: '2026-01-02', balance: 200 },
      { date: '2026-01-03', balance: 300 },
    ];
    const { container } = render(
      <NetWorthChart
        points={points}
        currency="USD"
        formatCents={formatUSD}
        asOfDate="2026-01-02"
      />,
    );
    const circles = container.querySelectorAll('circle');
    expect(circles.length).toBeGreaterThanOrEqual(1);
  });
});
