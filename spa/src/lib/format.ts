export interface AmountFormatOptions {
  hideDecimals?: boolean;
}

export function formatCents(
  cents: number,
  currency: string,
  options: AmountFormatOptions = {},
): string {
  const value = cents / 100;
  const digits = options.hideDecimals ? 0 : 2;
  try {
    return new Intl.NumberFormat('en-US', {
      style: 'currency',
      currency,
      minimumFractionDigits: digits,
      maximumFractionDigits: digits,
    }).format(value);
  } catch {
    // Unknown currency code — fall back to plain number with the code.
    return `${value.toFixed(digits)} ${currency}`;
  }
}

// `$X,XXX.XX` with a generic `$` and no currency code. Used where the column
// or surrounding context already conveys which currency it is.
export function formatAmount(cents: number, options: AmountFormatOptions = {}): string {
  const value = cents / 100;
  const digits = options.hideDecimals ? 0 : 2;
  const sign = value < 0 ? '-' : '';
  return `${sign}$${Math.abs(value).toLocaleString('en-US', {
    minimumFractionDigits: digits,
    maximumFractionDigits: digits,
  })}`;
}

// `$X,XXX.XX` with no currency code and no minus sign. Used where the row's
// type/column already conveys account and the surrounding color conveys sign.
export function formatBalanceAbs(cents: number, options: AmountFormatOptions = {}): string {
  const abs = Math.abs(cents / 100);
  const digits = options.hideDecimals ? 0 : 2;
  return `$${abs.toLocaleString('en-US', {
    minimumFractionDigits: digits,
    maximumFractionDigits: digits,
  })}`;
}
