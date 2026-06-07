import { describe, expect, test } from 'vitest';
import { summarizeBalances } from './balances';
import type { AccountBalance } from './types';

const row = (overrides: Partial<AccountBalance>): AccountBalance => ({
  account_id: 0,
  name: 'X',
  type: 'A',
  currency: 'USD',
  amount: 0,
  is_hidden: false,
  ...overrides,
});

describe('summarizeBalances', () => {
  test('includes Asset and Liability rows in the target currency', () => {
    const rows: AccountBalance[] = [
      row({ account_id: 1, name: 'Assets:Bank', type: 'A', amount: 125000 }),
      row({ account_id: 2, name: 'Assets:Cash', type: 'A', amount: 3500 }),
      row({ account_id: 3, name: 'Liab:Card', type: 'L', amount: -42000 }),
    ];
    const s = summarizeBalances(rows, 'USD');
    expect(s.assetsTotal).toBe(128500);
    expect(s.liabilitiesTotal).toBe(-42000);
    expect(s.netWorth).toBe(86500);
    expect(s.included).toHaveLength(3);
    expect(s.excludedByCurrency).toHaveLength(0);
    expect(s.excludedByType).toHaveLength(0);
  });

  test('files non-asset/liability accounts under excludedByType, not excludedByCurrency', () => {
    const rows: AccountBalance[] = [
      row({ account_id: 1, name: 'Assets:Bank', type: 'A', amount: 100 }),
      row({ account_id: 2, name: 'Income:Salary', type: 'R', amount: -50000 }),
      row({ account_id: 3, name: 'Expenses:Food', type: 'E', amount: 25000 }),
      row({ account_id: 4, name: 'Equity:Opening', type: 'C', amount: 1000 }),
    ];
    const s = summarizeBalances(rows, 'USD');
    expect(s.included).toHaveLength(1);
    expect(s.excludedByType).toHaveLength(3);
    expect(s.excludedByCurrency).toHaveLength(0);
    expect(s.assetsTotal).toBe(100);
    expect(s.liabilitiesTotal).toBe(0);
    expect(s.netWorth).toBe(100);
  });

  test('files A/L accounts in a different currency under excludedByCurrency', () => {
    const rows: AccountBalance[] = [
      row({ account_id: 1, name: 'Assets:USDBank', type: 'A', currency: 'USD', amount: 1000 }),
      row({ account_id: 2, name: 'Assets:TWDBank', type: 'A', currency: 'TWD', amount: 999999 }),
    ];
    const s = summarizeBalances(rows, 'USD');
    expect(s.included).toHaveLength(1);
    expect(s.excludedByCurrency).toHaveLength(1);
    expect(s.excludedByCurrency[0].account_id).toBe(2);
    expect(s.excludedByType).toHaveLength(0);
    expect(s.assetsTotal).toBe(1000);
  });

  test('type mismatch wins over currency mismatch when both apply', () => {
    // An Income account in another currency is filed under excludedByType,
    // not excludedByCurrency — it would never appear on a Net Worth view
    // regardless of the chosen currency.
    const rows: AccountBalance[] = [
      row({ account_id: 1, name: 'Income:TWDSalary', type: 'R', currency: 'TWD', amount: -10000 }),
    ];
    const s = summarizeBalances(rows, 'USD');
    expect(s.excludedByType).toHaveLength(1);
    expect(s.excludedByCurrency).toHaveLength(0);
    expect(s.included).toHaveLength(0);
  });

  test('returns zeros and empty arrays for empty input', () => {
    const s = summarizeBalances([], 'USD');
    expect(s.assetsTotal).toBe(0);
    expect(s.liabilitiesTotal).toBe(0);
    expect(s.netWorth).toBe(0);
    expect(s.included).toEqual([]);
    expect(s.excludedByCurrency).toEqual([]);
    expect(s.excludedByType).toEqual([]);
  });
});
