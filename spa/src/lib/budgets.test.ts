import { describe, expect, test } from 'vitest';
import {
  activeBudgets,
  budgetStatus,
  currentMonth,
  depthIn,
  elapsedFraction,
  parseCents,
  shiftMonth,
  usedPct,
} from './budgets';
import type { Budget } from './types';

describe('usedPct / budgetStatus', () => {
  test('normal ratios', () => {
    expect(usedPct(10000, 7300)).toBeCloseTo(73);
    expect(budgetStatus(10000, 7300)).toBe('ok');
    expect(budgetStatus(10000, 8000)).toBe('warning');
    expect(budgetStatus(10000, 10000)).toBe('warning');
    expect(budgetStatus(10000, 10001)).toBe('over');
  });
  test('zero budget', () => {
    expect(usedPct(0, 0)).toBeNull();
    expect(budgetStatus(0, 0)).toBe('ok');
    expect(budgetStatus(0, 1)).toBe('over');
    expect(budgetStatus(0, -100)).toBe('ok');
  });
});

describe('months (local time)', () => {
  test('currentMonth uses local getters', () => {
    // Local midnight on Nov 1 must be November even where UTC is still October.
    expect(currentMonth(new Date(2026, 10, 1, 0, 30))).toBe('2026-11');
  });
  test('shiftMonth crosses years', () => {
    expect(shiftMonth('2026-01', -1)).toBe('2025-12');
    expect(shiftMonth('2026-12', 1)).toBe('2027-01');
  });
  test('elapsedFraction only for the current month', () => {
    const now = new Date(2026, 9, 16, 0, 0); // Oct 16 local, 31-day month
    expect(elapsedFraction('2026-10', now)).toBeCloseTo(15 / 31);
    expect(elapsedFraction('2026-09', now)).toBeNull();
  });
});

describe('activeBudgets', () => {
  const b = (
    id: number,
    account: string,
    month: string,
    amount: number,
    stopped = false,
  ): Budget => ({
    id,
    account_id: account === 'Expenses:Food' ? 1 : 2,
    account_name: account,
    effective_month: month,
    amount,
    stopped,
  });
  const items = [
    b(1, 'Expenses:Food', '2026-01', 100),
    b(2, 'Expenses:Food', '2026-04', 200),
    b(3, 'Expenses:Food', '2026-07', 0, true),
    b(4, 'Expenses:Rent', '2026-09', 500),
  ];
  test('picks latest version <= month and drops stopped', () => {
    expect(activeBudgets(items, '2026-05').map((x) => x.id)).toEqual([2]);
    expect(activeBudgets(items, '2026-08')).toEqual([]);
    expect(activeBudgets(items, '2026-09').map((x) => x.id)).toEqual([4]);
  });
});

test('depthIn counts budgeted ancestors, not name prefixes', () => {
  const names = ['Expenses:Food', 'Expenses:Food:Dining', 'Expenses:FoodTruck'];
  expect(depthIn('Expenses:Food:Dining', names)).toBe(1);
  expect(depthIn('Expenses:FoodTruck', names)).toBe(0);
});

test('parseCents', () => {
  expect(parseCents('8000.5')).toBe(800050);
  expect(parseCents('abc')).toBeNaN();
  expect(parseCents('')).toBeNaN();
});
