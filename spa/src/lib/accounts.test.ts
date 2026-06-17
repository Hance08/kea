import { describe, expect, test } from 'vitest';
import { stripAccountTypePrefix } from './accounts';

describe('stripAccountTypePrefix', () => {
  test('strips each recognized type prefix and keeps the rest of the path', () => {
    expect(stripAccountTypePrefix('Assets:Bank:Checking')).toBe('Bank:Checking');
    expect(stripAccountTypePrefix('Liabilities:CreditCard')).toBe('CreditCard');
    expect(stripAccountTypePrefix('Equity:OpeningBalances_USD')).toBe('OpeningBalances_USD');
    expect(stripAccountTypePrefix('Revenue:Salary')).toBe('Salary');
    expect(stripAccountTypePrefix('Expenses:Food:Dinner')).toBe('Food:Dinner');
  });

  test('only strips the first segment when it matches a known type', () => {
    expect(stripAccountTypePrefix('Expenses')).toBe('Expenses');
    expect(stripAccountTypePrefix('Investments:Stocks')).toBe('Investments:Stocks');
    expect(stripAccountTypePrefix('')).toBe('');
  });

  test('does not strip later segments named like a type', () => {
    expect(stripAccountTypePrefix('Assets:Expenses:Foo')).toBe('Expenses:Foo');
  });
});
