import { displayAccount, displayAmount, displayOffsetAccount } from '@/lib/transactionDisplay';
import type { SplitDetail } from '@/lib/types';
import { describe, expect, test } from 'vitest';

const sp = (
  account_name: string,
  account_type: SplitDetail['account_type'],
  amount: number,
): SplitDetail => ({
  id: 0,
  account_id: 0,
  account_name,
  account_type,
  amount,
  currency: 'USD',
  memo: '',
});

describe('displayAccount', () => {
  test('Expense → returns the Expense account', () => {
    const splits = [sp('Assets:Bank', 'A', -500), sp('Expenses:Food', 'E', 500)];
    expect(displayAccount(splits, 'Expense')).toBe('Expenses:Food');
  });

  test('Income → returns the Revenue account', () => {
    const splits = [sp('Assets:Bank', 'A', 3000), sp('Revenue:Salary', 'R', -3000)];
    expect(displayAccount(splits, 'Income')).toBe('Revenue:Salary');
  });

  test('Transfer → returns the positive Asset/Liability account', () => {
    const splits = [sp('Assets:Checking', 'A', -1000), sp('Assets:Savings', 'A', 1000)];
    expect(displayAccount(splits, 'Transfer')).toBe('Assets:Savings');
  });

  test('Opening → returns the non-equity account', () => {
    const splits = [sp('Equity:OpeningBalances_USD', 'C', -1000), sp('Assets:Bank', 'A', 1000)];
    expect(displayAccount(splits, 'Opening')).toBe('Assets:Bank');
  });

  test('Other → returns the first positive amount account', () => {
    const splits = [sp('Assets:A', 'A', -100), sp('Assets:B', 'A', 100)];
    expect(displayAccount(splits, 'Other')).toBe('Assets:B');
  });

  test('empty splits → "-"', () => {
    expect(displayAccount([], 'Expense')).toBe('-');
  });
});

describe('displayOffsetAccount', () => {
  test('Expense with one offset → returns the Asset account', () => {
    const splits = [sp('Assets:Bank', 'A', -500), sp('Expenses:Food', 'E', 500)];
    expect(displayOffsetAccount(splits, 'Expense', 'Expenses:Food')).toBe('Assets:Bank');
  });

  test('Income with one offset → returns the Asset account', () => {
    const splits = [sp('Assets:Bank', 'A', 3000), sp('Revenue:Salary', 'R', -3000)];
    expect(displayOffsetAccount(splits, 'Income', 'Revenue:Salary')).toBe('Assets:Bank');
  });

  test('Transfer with one offset → returns the negative account', () => {
    const splits = [sp('Assets:Checking', 'A', -1000), sp('Assets:Savings', 'A', 1000)];
    expect(displayOffsetAccount(splits, 'Transfer', 'Assets:Savings')).toBe('Assets:Checking');
  });

  test('Expense with multiple Expense splits collapses to the single non-Expense offset', () => {
    // Splits: Cash(A), Food(E), Household(E). Go excludes ALL E-type, leaving {Assets:Cash}.
    const splits = [
      sp('Assets:Cash', 'A', -100),
      sp('Expenses:Food', 'E', 60),
      sp('Expenses:Household', 'E', 40),
    ];
    expect(displayOffsetAccount(splits, 'Expense', 'Expenses:Food')).toBe('Assets:Cash');
  });

  test('Expense with multiple non-Expense offsets → returns "(multiple)"', () => {
    // Splits: Cash(A), Bank(A), Food(E). Go excludes E-type, leaving {Assets:Cash, Assets:Bank}.
    const splits = [
      sp('Assets:Cash', 'A', -60),
      sp('Assets:Bank', 'A', -40),
      sp('Expenses:Food', 'E', 100),
    ];
    expect(displayOffsetAccount(splits, 'Expense', 'Expenses:Food')).toBe('(multiple)');
  });

  test('Transfer with multiple non-primary accounts → returns "(multiple)"', () => {
    // Default branch: excludes only by name. Three distinct names → (multiple) after excluding one.
    const splits = [sp('Assets:A', 'A', 100), sp('Assets:B', 'A', -60), sp('Assets:C', 'A', -40)];
    expect(displayOffsetAccount(splits, 'Transfer', 'Assets:A')).toBe('(multiple)');
  });

  test('empty splits → "-"', () => {
    expect(displayOffsetAccount([], 'Expense', 'x')).toBe('-');
  });
});

describe('displayAmount', () => {
  test('Expense → negative signed amount', () => {
    const splits = [sp('Assets:Bank', 'A', -500), sp('Expenses:Food', 'E', 500)];
    expect(displayAmount(splits, 'Expense')).toEqual({ amount: -500, currency: 'USD' });
  });

  test('Income → positive signed amount', () => {
    const splits = [sp('Assets:Bank', 'A', 3000), sp('Revenue:Salary', 'R', -3000)];
    expect(displayAmount(splits, 'Income')).toEqual({ amount: 3000, currency: 'USD' });
  });

  test('Transfer → absolute positive amount', () => {
    const splits = [sp('Assets:Checking', 'A', -1000), sp('Assets:Savings', 'A', 1000)];
    expect(displayAmount(splits, 'Transfer')).toEqual({ amount: 1000, currency: 'USD' });
  });

  test('Other → max positive amount (matches CLI behavior)', () => {
    const splits = [sp('Assets:A', 'A', -200), sp('Assets:B', 'A', 200)];
    expect(displayAmount(splits, 'Other')).toEqual({ amount: 200, currency: 'USD' });
  });

  test('empty splits → 0 with empty currency', () => {
    expect(displayAmount([], 'Expense')).toEqual({ amount: 0, currency: '' });
  });
});
