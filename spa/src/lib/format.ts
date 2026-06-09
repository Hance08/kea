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
