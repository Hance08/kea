import { render, screen, waitFor } from '@testing-library/react';
import { describe, expect, it, vi } from 'vitest';
import { makeTestApp } from './test-app';

vi.mock('@/lib/accounts', async () => ({
  ...(await vi.importActual<object>('@/lib/accounts')),
  getAccountTree: vi.fn().mockResolvedValue([]),
}));

vi.mock('@/lib/api', async () => ({
  ...(await vi.importActual<object>('@/lib/api')),
  getConfig: vi
    .fn()
    .mockResolvedValue({ defaults: { currency: 'USD' }, display: { hide_decimals: false } }),
  getBalances: vi.fn().mockResolvedValue({ items: [], total_count: 0, limit: 0, offset: 0 }),
  getLedgers: vi
    .fn()
    .mockResolvedValue({ active: 'default', items: [{ name: 'default', path: '/x', active: true }] }),
}));

describe('Sidebar Reconcile link', () => {
  it('renders Reconcile as an active link on /reconcile', async () => {
    render(makeTestApp('/reconcile'));
    await waitFor(() => {
      const link = screen.getByRole('link', { name: /^reconcile$/i });
      expect(link).toHaveAttribute('href', '/reconcile');
      expect(link.className).toMatch(/bg-primary|font-medium/);
    });
  });

  it('also highlights Reconcile on /reconcile/$id (prefix match)', async () => {
    render(makeTestApp('/reconcile/3'));
    await waitFor(() => {
      const link = screen.getByRole('link', { name: /^reconcile$/i });
      expect(link.className).toMatch(/bg-primary|font-medium/);
    });
  });
});
