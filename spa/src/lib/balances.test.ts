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
    expect(s.excluded).toHaveLength(0);
  });

  test('excludes non-asset/liability accounts', () => {
    const rows: AccountBalance[] = [
      row({ account_id: 1, name: 'Assets:Bank', type: 'A', amount: 100 }),
      row({ account_id: 2, name: 'Income:Salary', type: 'R', amount: -50000 }),
      row({ account_id: 3, name: 'Expenses:Food', type: 'E', amount: 25000 }),
      row({ account_id: 4, name: 'Equity:Opening', type: 'C', amount: 1000 }),
    ];
    const s = summarizeBalances(rows, 'USD');
    expect(s.included).toHaveLength(1);
    expect(s.excluded).toHaveLength(3);
    expect(s.assetsTotal).toBe(100);
    expect(s.liabilitiesTotal).toBe(0);
    expect(s.netWorth).toBe(100);
  });

  test('excludes accounts in a different currency', () => {
    const rows: AccountBalance[] = [
      row({ account_id: 1, name: 'Assets:USDBank', type: 'A', currency: 'USD', amount: 1000 }),
      row({ account_id: 2, name: 'Assets:TWDBank', type: 'A', currency: 'TWD', amount: 999999 }),
    ];
    const s = summarizeBalances(rows, 'USD');
    expect(s.included).toHaveLength(1);
    expect(s.excluded).toHaveLength(1);
    expect(s.excluded[0].account_id).toBe(2);
    expect(s.assetsTotal).toBe(1000);
  });

  test('returns zeros for empty input', () => {
    const s = summarizeBalances([], 'USD');
    expect(s.assetsTotal).toBe(0);
    expect(s.liabilitiesTotal).toBe(0);
    expect(s.netWorth).toBe(0);
    expect(s.included).toEqual([]);
    expect(s.excluded).toEqual([]);
  });
});
