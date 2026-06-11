import { apiFetch } from './api';
import type {
  Account,
  AccountNode,
  AccountType,
  BalanceResponse,
  CreateAccountInput,
  ListResult,
  UpdateAccountInput,
} from './types';

interface ListAccountsOpts {
  q?: string;
  type?: AccountType;
  include_hidden?: boolean;
  limit?: number;
  offset?: number;
}

function buildQuery(params: Record<string, string | number | boolean | undefined>): string {
  const usp = new URLSearchParams();
  for (const [k, v] of Object.entries(params)) {
    if (v === undefined || v === '' || v === null) continue;
    usp.set(k, String(v));
  }
  const s = usp.toString();
  return s ? `?${s}` : '';
}

export function listAccounts(opts: ListAccountsOpts = {}): Promise<ListResult<Account>> {
  // When q is set we use the SearchAccounts backend path (fuzzy name match,
  // paginated, but excludes per-currency system accounts).
  // When q is empty we route to ListAccounts so system accounts are included
  // — important for the Type filter on /accounts.
  const hasQ = Boolean(opts.q && opts.q.length > 0);
  const params: Record<string, string | number | boolean | undefined> = {
    q: opts.q,
    type: opts.type,
    include_hidden: opts.include_hidden,
  };
  if (hasQ) {
    params.limit = opts.limit;
    params.offset = opts.offset;
    params.include_count = true;
  }
  return apiFetch<ListResult<Account>>(`/api/accounts${buildQuery(params)}`);
}

function normalizeNode(node: AccountNode): AccountNode {
  return {
    account: node.account,
    children: (node.children ?? []).map(normalizeNode),
  };
}

export function getAccountTree(opts: { include_hidden?: boolean } = {}): Promise<AccountNode[]> {
  const q = buildQuery({ include_hidden: opts.include_hidden });
  return apiFetch<AccountNode[]>(`/api/accounts/tree${q}`).then((roots) =>
    (roots ?? []).map(normalizeNode),
  );
}

export function getAccount(id: number): Promise<Account> {
  return apiFetch<Account>(`/api/accounts/${id}`);
}

export function getAccountBalance(id: number): Promise<BalanceResponse> {
  return apiFetch<BalanceResponse>(`/api/accounts/${id}/balance`);
}

export function createAccount(input: CreateAccountInput): Promise<Account> {
  return apiFetch<Account>('/api/accounts', {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify(input),
  });
}

export function updateAccount(id: number, patch: UpdateAccountInput): Promise<Account> {
  return apiFetch<Account>(`/api/accounts/${id}`, {
    method: 'PATCH',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify(patch),
  });
}

export function deleteAccount(id: number): Promise<{ deleted: boolean; id: number }> {
  return apiFetch<{ deleted: boolean; id: number }>(`/api/accounts/${id}`, {
    method: 'DELETE',
  });
}

// Mirrors model.IsOpeningBalancesAccount: matches the per-currency form
// (Equity:OpeningBalances_<CCY>). The legacy bare name is auto-renamed at
// startup, so it shouldn't appear at runtime.
export function isOpeningBalancesAccount(name: string): boolean {
  return /^Equity:OpeningBalances_[A-Z]{3}$/.test(name);
}

// Returns the balance in the account's "natural direction" — what the
// user means by "the size of this account" regardless of the stored sign
// convention. Used for sorting so descending = biggest of this thing.
//
// Liability and Revenue are stored negative when they grow (more debt /
// more income earned), so we negate them to get a positive scale that
// sort_desc can rank intuitively against Asset / Expense / Equity.
export function naturalAmount(type: AccountType, stored: number): number {
  return type === 'L' || type === 'R' ? -stored : stored;
}

// Colors a balance amount per accounting display convention:
//   Asset / Liability / Equity → positive green, negative red (raw sign).
//   Revenue / Expense          → inverted: positive red (refunds for Rev,
//                                spending for Exp), negative green
//                                (income for Rev, expense refunds for Exp).
// Returns a Tailwind class string ('' for zero).
export function balanceColor(type: AccountType, amount: number): string {
  const inverted = type === 'R' || type === 'E';
  if (amount > 0) return inverted ? 'text-red-600' : 'text-green-600';
  if (amount < 0) return inverted ? 'text-green-600' : 'text-red-600';
  return '';
}

export function searchAccounts(query: string): Promise<ListResult<Account>> {
  return apiFetch<ListResult<Account>>(`/api/accounts${buildQuery({ q: query, limit: 20 })}`);
}
