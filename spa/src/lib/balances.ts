import type { AccountBalance } from './types';

export interface BalancesSummary {
  assetsTotal: number;
  liabilitiesTotal: number;
  netWorth: number;
  included: AccountBalance[];
  excludedByCurrency: AccountBalance[]; // A/L rows whose currency differs from the target
  excludedByType: AccountBalance[]; // Non-A/L rows (Equity, Revenue, Expense)
}

export function summarizeBalances(
  rows: AccountBalance[],
  summaryCurrency: string,
): BalancesSummary {
  const included: AccountBalance[] = [];
  const excludedByCurrency: AccountBalance[] = [];
  const excludedByType: AccountBalance[] = [];
  let assetsTotal = 0;
  let liabilitiesTotal = 0;

  for (const row of rows) {
    const isAssetOrLiability = row.type === 'A' || row.type === 'L';
    if (!isAssetOrLiability) {
      // Income / Expense / Equity rows are never part of a Net Worth view,
      // regardless of currency. Bucket them separately so the UI can ignore them.
      excludedByType.push(row);
      continue;
    }
    if (row.currency !== summaryCurrency) {
      excludedByCurrency.push(row);
      continue;
    }
    included.push(row);
    if (row.type === 'A') {
      assetsTotal += row.amount;
    } else {
      liabilitiesTotal += row.amount;
    }
  }

  return {
    assetsTotal,
    liabilitiesTotal,
    netWorth: assetsTotal + liabilitiesTotal,
    included,
    excludedByCurrency,
    excludedByType,
  };
}
