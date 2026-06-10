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
  const q = buildQuery({
    q: opts.q,
    type: opts.type,
    include_hidden: opts.include_hidden,
    limit: opts.limit,
    offset: opts.offset,
    include_count: true,
  });
  return apiFetch<ListResult<Account>>(`/api/accounts${q}`);
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

export function searchAccounts(query: string): Promise<ListResult<Account>> {
  return apiFetch<ListResult<Account>>(`/api/accounts${buildQuery({ q: query, limit: 20 })}`);
}
