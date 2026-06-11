export function formatCents(cents: number, currency: string): string {
  const value = cents / 100;
  try {
    return new Intl.NumberFormat('en-US', {
      style: 'currency',
      currency,
    }).format(value);
  } catch {
    // Unknown currency code — fall back to plain number with the code.
    return `${value.toFixed(2)} ${currency}`;
  }
}

// `$X,XXX.XX` with no currency code and no minus sign. Used where the row's
// type/column already conveys account and the surrounding color conveys sign.
export function formatBalanceAbs(cents: number): string {
  const abs = Math.abs(cents / 100);
  return `$${abs.toLocaleString('en-US', { minimumFractionDigits: 2, maximumFractionDigits: 2 })}`;
}
