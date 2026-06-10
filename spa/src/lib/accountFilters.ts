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
