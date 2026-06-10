import type { Account } from './types';

/**
 * Build a leaf-only predicate from a list of accounts. An account is a
 * leaf iff no other account in the list has its id as parent_id. Only
 * leaf accounts can hold transactions, so transaction-entry inputs
 * should restrict the combobox to leaves.
 *
 * Returns undefined while the list is loading so the caller can leave
 * the filter unset (showing all results) rather than block input.
 */
export function buildLeafFilter(
  accounts: Account[] | undefined,
): ((acc: Account) => boolean) | undefined {
  if (!accounts) return undefined;
  const parentIds = new Set<number>();
  for (const a of accounts) {
    if (a.parent_id !== undefined && a.parent_id !== null) parentIds.add(a.parent_id);
  }
  return (acc) => !parentIds.has(acc.id);
}

/**
 * Build a single-currency predicate. Used to lock other comboboxes in
 * a transaction to the currency of the first picked account (the
 * service rejects mixed-currency splits in one transaction).
 */
export function buildCurrencyFilter(
  currency: string | undefined,
): ((acc: Account) => boolean) | undefined {
  if (!currency) return undefined;
  return (acc) => acc.currency === currency;
}

/**
 * Compose multiple optional predicates with logical AND. Predicates
 * that are undefined are skipped. Returns undefined if every input is
 * undefined so callers can leave the combined filter unset.
 */
export function combineFilters<T>(
  ...filters: (((x: T) => boolean) | undefined)[]
): ((x: T) => boolean) | undefined {
  const active = filters.filter((f): f is (x: T) => boolean => !!f);
  if (active.length === 0) return undefined;
  if (active.length === 1) return active[0];
  return (x) => active.every((f) => f(x));
}
