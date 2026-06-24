import { render } from '@testing-library/react';
import { expect, test, vi } from 'vitest';
import type { ReportRow } from '../../lib/types';
import { withServerConfig } from '../test-app';

vi.mock('@tanstack/react-router', async (importOriginal) => {
  const actual = await importOriginal<typeof import('@tanstack/react-router')>();
  return {
    ...actual,
    // biome-ignore lint/a11y/useValidAnchor: test stub for Link, not a real navigable anchor
    Link: ({ children }: { children: React.ReactNode }) => <a>{children}</a>,
  };
});

// Import AFTER the mock so the component picks up the stub.
import { ReportRowTable } from '../../components/reports/ReportRowTable';

const rows: ReportRow[] = [
  { account_name: 'Expenses:Rent', offset_account: '', amount: 1800, currency: 'USD', tx_count: 1 },
  { account_name: 'Expenses:Food', offset_account: '', amount: 642, currency: 'USD', tx_count: 1 },
];
const nameToId = new Map<string, number>([
  ['Expenses:Rent', 1],
  ['Expenses:Food', 2],
]);

test('ReportRowTable: renders no swatches when swatchColors is undefined', () => {
  const { container } = render(
    withServerConfig(
      <ReportRowTable rows={rows} currency="USD" nameToId={nameToId} period={null} />,
    ),
  );
  expect(container.querySelectorAll('[data-testid="row-swatch"]').length).toBe(0);
});

test('ReportRowTable: renders one swatch per row when swatchColors provided', () => {
  const { container } = render(
    withServerConfig(
      <ReportRowTable
        rows={rows}
        currency="USD"
        nameToId={nameToId}
        period={null}
        swatchColors={['bg-red-700', 'bg-red-600']}
      />,
    ),
  );
  const swatches = container.querySelectorAll('[data-testid="row-swatch"]');
  expect(swatches).toHaveLength(2);
  expect(swatches[0].className).toContain('bg-red-700');
  expect(swatches[1].className).toContain('bg-red-600');
});
