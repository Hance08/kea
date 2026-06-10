import { determineType } from '@/lib/determineType';
import type { SplitDetail } from '@/lib/types';
import { describe, expect, test } from 'vitest';

const sp = (
  account_name: string,
  account_type: SplitDetail['account_type'],
  amount: number,
  memo = '',
): SplitDetail => ({
  id: 0,
  account_id: 0,
  account_name,
  account_type,
  amount,
  currency: 'USD',
  memo,
});

describe('determineType', () => {
  test('empty splits → Other', () => {
    expect(determineType([])).toBe('Other');
  });

  test('Expense: E + A', () => {
    expect(determineType([sp('Expenses:Food', 'E', 500), sp('Assets:Bank', 'A', -500)])).toBe(
      'Expense',
    );
  });

  test('Income: R + A (positive total revenue dominates)', () => {
    expect(determineType([sp('Revenue:Salary', 'R', -3000), sp('Assets:Bank', 'A', 3000)])).toBe(
      'Income',
    );
  });

  test('Transfer: A + A', () => {
    expect(
      determineType([sp('Assets:Savings', 'A', 1000), sp('Assets:Checking', 'A', -1000)]),
    ).toBe('Transfer');
  });

  test('Transfer: A + L', () => {
    expect(determineType([sp('Assets:Bank', 'A', 1000), sp('Liabilities:Card', 'L', -1000)])).toBe(
      'Transfer',
    );
  });

  test('Income + Expense where revenue > expense → Income', () => {
    expect(
      determineType([
        sp('Revenue:Salary', 'R', -5000),
        sp('Expenses:Tax', 'E', 1000),
        sp('Assets:Bank', 'A', 4000),
      ]),
    ).toBe('Income');
  });

  test('Income + Expense where expense > revenue → Expense', () => {
    expect(
      determineType([
        sp('Revenue:Bonus', 'R', -500),
        sp('Expenses:Tax', 'E', 1000),
        sp('Assets:Bank', 'A', -500),
      ]),
    ).toBe('Expense');
  });

  test('Expense with 2 Asset/Liab and asset increase greater than expense → Transfer', () => {
    expect(
      determineType([
        sp('Assets:Bank', 'A', 1000),
        sp('Assets:Cash', 'A', -1500),
        sp('Expenses:Fees:Stocks', 'E', 500),
      ]),
    ).toBe('Transfer');
  });

  test('Expense with single Asset → Expense', () => {
    expect(determineType([sp('Assets:Cash', 'A', -50), sp('Expenses:Food', 'E', 50)])).toBe(
      'Expense',
    );
  });

  test('Equity + asset increase → Deposit', () => {
    expect(determineType([sp('Equity:Retained', 'C', -100), sp('Assets:Bank', 'A', 100)])).toBe(
      'Deposit',
    );
  });

  test('Equity + asset decrease → Withdrawal', () => {
    expect(determineType([sp('Equity:Retained', 'C', 100), sp('Assets:Bank', 'A', -100)])).toBe(
      'Withdrawal',
    );
  });

  test('Opening memo override → Opening', () => {
    expect(
      determineType([
        sp('Assets:Bank', 'A', 1000, 'Opening Balance'),
        sp('Equity:OpeningBalances_USD', 'C', -1000),
      ]),
    ).toBe('Opening');
  });
});
