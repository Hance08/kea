import { apiFetch } from './api';
import type {
  Account,
  CreateTransactionInput,
  ListResult,
  TransactionDetail,
  TransactionFilter,
  TransactionStatus,
  UpdateTransactionInput,
} from './types';

interface ListOptions {
  limit?: number;
  offset?: number;
  include_count?: boolean;
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

export function listTransactions(
  filter: TransactionFilter,
  opts: ListOptions = {},
): Promise<ListResult<TransactionDetail>> {
  const q = buildQuery({
    account_id: filter.account_id,
    type: filter.type,
    status: filter.status,
    start_time: filter.start_time,
    end_time: filter.end_time,
    description: filter.description,
    limit: opts.limit,
    offset: opts.offset,
    include_count: opts.include_count ?? true,
  });
  return apiFetch<ListResult<TransactionDetail>>(`/api/transactions${q}`);
}

export function getTransaction(id: number): Promise<TransactionDetail> {
  return apiFetch<TransactionDetail>(`/api/transactions/${id}`);
}

export function createTransaction(input: CreateTransactionInput): Promise<TransactionDetail> {
  return apiFetch<TransactionDetail>('/api/transactions', {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify(input),
  });
}

export function updateTransaction(input: UpdateTransactionInput): Promise<TransactionDetail> {
  return apiFetch<TransactionDetail>(`/api/transactions/${input.id}`, {
    method: 'PATCH',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify(input),
  });
}

export function updateTransactionStatus(
  id: number,
  status: TransactionStatus,
): Promise<TransactionDetail> {
  return apiFetch<TransactionDetail>(`/api/transactions/${id}/status`, {
    method: 'PATCH',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ status }),
  });
}

export function deleteTransaction(id: number): Promise<{ deleted: boolean; id: number }> {
  return apiFetch<{ deleted: boolean; id: number }>(`/api/transactions/${id}`, {
    method: 'DELETE',
  });
}

export function listAccounts(): Promise<ListResult<Account>> {
  return apiFetch<ListResult<Account>>('/api/accounts?include_hidden=false');
}

export function searchAccounts(query: string): Promise<ListResult<Account>> {
  const q = buildQuery({ q: query, limit: 20 });
  return apiFetch<ListResult<Account>>(`/api/accounts${q}`);
}
