import { expect, test } from 'vitest';
import { activeBudgets, activeVersions } from '../lib/budgets';
import { remainingLabel, savingsStatus } from '../lib/savings';
import type { Budget, SavingsTarget } from '../lib/types';

test('savingsStatus', () => {
  expect(savingsStatus(1000, 1000, null)).toBe('achieved');
  expect(savingsStatus(1000, 1500, 0.2)).toBe('achieved');
  expect(savingsStatus(1000, -1, 0.9)).toBe('negative');
  expect(savingsStatus(0, 0, null)).toBe('achieved');
  expect(savingsStatus(0, -500, null)).toBe('negative');
  // Current month: compare with the elapsed fraction.
  expect(savingsStatus(1000, 600, 0.5)).toBe('on-track');
  expect(savingsStatus(1000, 400, 0.5)).toBe('behind');
  expect(savingsStatus(1000, 0, 0)).toBe('on-track');
  // A month that is not the current one is complete: anything short is behind.
  expect(savingsStatus(1000, 999, null)).toBe('behind');
});

test('remainingLabel', () => {
  const fmt = (c: number) => (c / 100).toFixed(2);
  expect(remainingLabel(300000, fmt)).toBe('3000.00 to go');
  expect(remainingLabel(-650000, fmt)).toBe('6500.00 over target');
  expect(remainingLabel(0, fmt)).toBe('Target met');
});

test('activeVersions works for savings targets and keeps activeBudgets behavior', () => {
  const targets: SavingsTarget[] = [
    {
      id: 1,
      account_id: 1,
      account_name: 'Assets:Savings',
      effective_month: '2026-01',
      amount: 100,
      stopped: false,
    },
    {
      id: 2,
      account_id: 1,
      account_name: 'Assets:Savings',
      effective_month: '2026-03',
      amount: 0,
      stopped: true,
    },
    {
      id: 3,
      account_id: 2,
      account_name: 'Assets:Emergency',
      effective_month: '2026-02',
      amount: 50,
      stopped: false,
    },
  ];
  expect(activeVersions(targets, '2026-02').map((t) => t.id)).toEqual([3, 1]);
  expect(activeVersions(targets, '2026-04').map((t) => t.id)).toEqual([3]);

  const budgets: Budget[] = [
    {
      id: 9,
      account_id: 5,
      account_name: 'Expenses:Food',
      effective_month: '2026-01',
      amount: 1,
      stopped: false,
    },
  ];
  expect(activeBudgets(budgets, '2026-01').map((b) => b.id)).toEqual([9]);
});
