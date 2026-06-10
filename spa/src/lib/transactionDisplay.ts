import type { SplitDetail, TransactionType } from './types';

export function displayAccount(splits: SplitDetail[], type: TransactionType | string): string {
  if (splits.length === 0) return '-';

  switch (type) {
    case 'Expense':
      for (const s of splits) if (s.account_type === 'E') return s.account_name;
      break;
    case 'Income':
      for (const s of splits) if (s.account_type === 'R') return s.account_name;
      break;
    case 'Transfer':
      for (const s of splits) {
        if (s.amount > 0 && (s.account_type === 'A' || s.account_type === 'L')) {
          return s.account_name;
        }
      }
      break;
    case 'Opening':
      for (const s of splits) if (s.account_type !== 'C') return s.account_name;
      break;
    case 'Other':
      for (const s of splits) if (s.amount > 0) return s.account_name;
      break;
  }
  return splits[0]?.account_name ?? '-';
}

export function displayOffsetAccount(
  splits: SplitDetail[],
  type: TransactionType | string,
  primaryAccount: string,
): string {
  if (splits.length === 0) return '-';

  const seen = new Set<string>();
  const primaryType = type === 'Expense' ? 'E' : type === 'Income' ? 'R' : null;

  if (primaryType !== null) {
    for (const s of splits) {
      if (s.account_type !== primaryType) seen.add(s.account_name);
    }
  } else {
    for (const s of splits) {
      if (s.account_name !== primaryAccount) seen.add(s.account_name);
    }
  }

  if (seen.size === 0) return '-';
  if (seen.size === 1) return seen.values().next().value as string;
  return '(multiple)';
}

export function displayAmount(
  splits: SplitDetail[],
  type: TransactionType | string,
): { amount: number; currency: string } {
  if (splits.length === 0) return { amount: 0, currency: '' };

  const currency = splits[0].currency;

  switch (type) {
    case 'Expense': {
      const ex = splits.find((s) => s.account_type === 'E');
      if (ex) return { amount: -Math.abs(ex.amount), currency: ex.currency };
      break;
    }
    case 'Income': {
      const rv = splits.find((s) => s.account_type === 'R');
      if (rv) return { amount: Math.abs(rv.amount), currency: rv.currency };
      break;
    }
    case 'Transfer': {
      const positive = splits.find(
        (s) => s.amount > 0 && (s.account_type === 'A' || s.account_type === 'L'),
      );
      if (positive) return { amount: positive.amount, currency: positive.currency };
      break;
    }
  }

  // Fallback (Other / Opening / Deposit / Withdrawal): max positive amount + its currency
  let maxAmount = 0;
  let chosenCurrency = currency;
  for (const s of splits) {
    if (s.amount > maxAmount) {
      maxAmount = s.amount;
      chosenCurrency = s.currency;
    }
  }
  return { amount: maxAmount, currency: chosenCurrency };
}
