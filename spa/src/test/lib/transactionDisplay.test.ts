import { describe, expect, test } from 'vitest';
import {
  displayAccount,
  displayAmount,
  displayOffsetAccount,
} from '@/lib/transactionDisplay';
import type { SplitDetail } from '@/lib/types';

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
    const splits = [
      sp('Equity:OpeningBalances_USD', 'C', -1000),
      sp('Assets:Bank', 'A', 1000),
    ];
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

  test('multiple offsets → returns "(multiple)"', () => {
    const splits = [
      sp('Assets:Cash', 'A', -100),
      sp('Expenses:Food', 'E', 60),
      sp('Expenses:Household', 'E', 40),
    ];
    expect(displayOffsetAccount(splits, 'Expense', 'Expenses:Food')).toBe('(multiple)');
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
