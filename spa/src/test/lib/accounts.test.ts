import { describe, expect, it } from 'vitest';
import { isOpeningBalancesAccount } from '@/lib/accounts';

describe('isOpeningBalancesAccount', () => {
  it('matches per-currency system accounts', () => {
    expect(isOpeningBalancesAccount('Equity:OpeningBalances_USD')).toBe(true);
    expect(isOpeningBalancesAccount('Equity:OpeningBalances_EUR')).toBe(true);
  });

  it('matches legacy single-name system account', () => {
    expect(isOpeningBalancesAccount('Equity:OpeningBalances')).toBe(true);
  });

  it('does not match unrelated accounts', () => {
    expect(isOpeningBalancesAccount('Equity:RetainedEarnings')).toBe(false);
    expect(isOpeningBalancesAccount('Assets:Bank:Checking')).toBe(false);
    expect(isOpeningBalancesAccount('')).toBe(false);
  });
});
