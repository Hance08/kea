export type SavingsStatus = 'achieved' | 'on-track' | 'behind' | 'negative';

/**
 * Saving is on track when the saved share of the target keeps up with the
 * elapsed share of the month. `elapsed` is null for any month other than the
 * current one, which is treated as complete.
 */
export function savingsStatus(
  target: number,
  saved: number,
  elapsed: number | null,
): SavingsStatus {
  if (saved < 0) return 'negative';
  if (saved >= target) return 'achieved';
  const expected = elapsed ?? 1;
  return saved / target < expected ? 'behind' : 'on-track';
}

/** "X to go" while short, "X over target" when exceeded, "Target met" at exactly zero. */
export function remainingLabel(remaining: number, format: (cents: number) => string): string {
  if (remaining > 0) return `${format(remaining)} to go`;
  if (remaining < 0) return `${format(-remaining)} over target`;
  return 'Target met';
}
