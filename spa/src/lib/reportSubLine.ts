/**
 * Build the muted sub-line shown under Total Income / Total Expense KPI cards.
 * Returns undefined when both subtotals are zero, so the caller can skip
 * passing a sub-line altogether (KpiCard renders nothing for an undefined
 * sub-line).
 */
export function regularSubLine(
  regular: number,
  irregular: number,
  currency: string,
  formatCents: (cents: number, currency: string) => string,
): string | undefined {
  if (regular === 0 && irregular === 0) return undefined;
  return `Regular ${formatCents(regular, currency)} · Irregular ${formatCents(irregular, currency)}`;
}
