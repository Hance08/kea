import { render } from '@testing-library/react';
import { expect, test } from 'vitest';
import { Sparkline } from '@/components/balances/Sparkline';

test('renders nothing when fewer than 2 points', () => {
  const { container } = render(
    <Sparkline points={[{ month: '2024-01', balance: 100 }]} strokeClassName="stroke-green-600" />,
  );
  expect(container.querySelector('svg')).toBeNull();
});

test('renders a polyline when given >= 2 points', () => {
  const { container } = render(
    <Sparkline
      points={[
        { month: '2024-01', balance: 100 },
        { month: '2024-02', balance: 200 },
      ]}
      strokeClassName="stroke-green-600"
    />,
  );
  const line = container.querySelector('polyline');
  expect(line).not.toBeNull();
  expect(line?.getAttribute('class')).toContain('stroke-green-600');
});

test('flat history renders a horizontal line at vertical center', () => {
  const { container } = render(
    <Sparkline
      points={[
        { month: '2024-01', balance: 500 },
        { month: '2024-02', balance: 500 },
        { month: '2024-03', balance: 500 },
      ]}
      strokeClassName="stroke-green-600"
    />,
  );
  const line = container.querySelector('polyline');
  // viewBox is 0 0 100 30 → vertical center y = 15.
  const points = line?.getAttribute('points') ?? '';
  const ys = points.split(' ').map((p) => Number(p.split(',')[1]));
  for (const y of ys) {
    expect(y).toBeCloseTo(15);
  }
});

test('scales points to the line min/max', () => {
  const { container } = render(
    <Sparkline
      points={[
        { month: '2024-01', balance: 0 },
        { month: '2024-02', balance: 100 },
      ]}
      strokeClassName="stroke-green-600"
    />,
  );
  // 2 points → x = 0 and 100. Min balance maps to y=30 (bottom), max to y=0 (top).
  const points = container.querySelector('polyline')?.getAttribute('points') ?? '';
  expect(points).toBe('0,30 100,0');
});
