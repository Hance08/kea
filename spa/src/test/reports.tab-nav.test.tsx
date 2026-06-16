import { render, screen } from '@testing-library/react';
import { expect, test } from 'vitest';
import { makeTestApp } from './test-app';

test('renders all 5 report tabs', () => {
  render(makeTestApp('/reports/income-statement'));
  expect(screen.getByRole('link', { name: /Income Statement/i })).toBeInTheDocument();
  expect(screen.getByRole('link', { name: /Income Breakdown/i })).toBeInTheDocument();
  expect(screen.getByRole('link', { name: /Expense Breakdown/i })).toBeInTheDocument();
  expect(screen.getByRole('link', { name: /Balance Sheet/i })).toBeInTheDocument();
  expect(screen.getByRole('link', { name: /Net Worth/i })).toBeInTheDocument();
});

test('marks the active tab', () => {
  render(makeTestApp('/reports/balance-sheet'));
  const active = screen.getByRole('link', { name: /Balance Sheet/i });
  expect(active.getAttribute('aria-current')).toBe('page');
});
