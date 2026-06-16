import type { SplitDetail, TransactionType } from './types';

const OPENING_MEMO = 'Opening Balance'; // mirrors model.OpeningAccountMemo

export function determineType(splits: SplitDetail[]): TransactionType {
  if (splits.length === 0) return 'Other';

  let totalRevenueAmount = 0;
  let totalExpenseAmount = 0;
  let totalPositiveAssetLiabAmount = 0;

  let hasExpense = false;
  let hasRevenue = false;
  let hasEquity = false;
  let assetOrLiabCnt = 0;
  let isOpening = false;
  let isAssetIncrease = false;
  let hasInvestmentAccount = false;
  let nonInvestmentAssetOrLiabCnt = 0;

  for (const s of splits) {
    if (s.memo === OPENING_MEMO) isOpening = true;
    switch (s.account_type) {
      case 'E':
        hasExpense = true;
        totalExpenseAmount += Math.abs(s.amount);
        break;
      case 'R':
        hasRevenue = true;
        totalRevenueAmount += Math.abs(s.amount);
        break;
      case 'A':
        assetOrLiabCnt++;
        if (s.account_name.startsWith('Assets:Investments:')) {
          hasInvestmentAccount = true;
        } else {
          nonInvestmentAssetOrLiabCnt++;
        }
        if (s.amount > 0) {
          isAssetIncrease = true;
          totalPositiveAssetLiabAmount += s.amount;
        }
        break;
      case 'L':
        assetOrLiabCnt++;
        nonInvestmentAssetOrLiabCnt++;
        if (s.amount > 0) totalPositiveAssetLiabAmount += s.amount;
        break;
      case 'C':
        hasEquity = true;
        break;
    }
  }

  if (isOpening) return 'Opening';

  if (hasInvestmentAccount && nonInvestmentAssetOrLiabCnt >= 1) return 'Investment';

  if (hasExpense && hasRevenue) {
    return totalRevenueAmount >= totalExpenseAmount ? 'Income' : 'Expense';
  }

  if (hasExpense && assetOrLiabCnt >= 2) {
    return totalPositiveAssetLiabAmount > totalExpenseAmount ? 'Transfer' : 'Expense';
  }
  if (hasExpense && assetOrLiabCnt === 1) return 'Expense';

  if (hasRevenue && assetOrLiabCnt >= 1) return 'Income';

  if (assetOrLiabCnt >= 2) return 'Transfer';

  if (hasEquity && assetOrLiabCnt >= 1) {
    return isAssetIncrease ? 'Deposit' : 'Withdrawal';
  }

  return 'Other';
}
