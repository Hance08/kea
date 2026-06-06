import type { AccountBalance } from './types';

export interface BalancesSummary {
  assetsTotal: number;
  liabilitiesTotal: number;
  netWorth: number;
  included: AccountBalance[];
  excluded: AccountBalance[];
}

export function summarizeBalances(
  rows: AccountBalance[],
  summaryCurrency: string,
): BalancesSummary {
  const included: AccountBalance[] = [];
  const excluded: AccountBalance[] = [];
  let assetsTotal = 0;
  let liabilitiesTotal = 0;

  for (const row of rows) {
    const isAssetOrLiability = row.type === 'A' || row.type === 'L';
    const matchesCurrency = row.currency === summaryCurrency;
    if (!isAssetOrLiability || !matchesCurrency) {
      excluded.push(row);
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
    excluded,
  };
}
