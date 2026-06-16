import type {
  AccountBalance,
  BalanceHistoryResponse,
  LedgerInfo,
  LedgerListResponse,
  ListResult,
  ServerConfig,
} from './types';

export class ApiError extends Error {
  readonly status: number;
  readonly field?: string;
  constructor(status: number, message: string, field?: string) {
    super(message);
    this.name = 'ApiError';
    this.status = status;
    this.field = field;
  }
}

interface ApiErrorBody {
  message?: string;
  field?: string;
}

export async function apiFetch<T>(path: string, init?: RequestInit): Promise<T> {
  const resp = await fetch(path, init);
  if (!resp.ok) {
    let body: ApiErrorBody = {};
    try {
      body = (await resp.json()) as ApiErrorBody;
    } catch {
      // body wasn't JSON; fall through with empty body
    }
    throw new ApiError(resp.status, body.message ?? resp.statusText, body.field);
  }
  return (await resp.json()) as T;
}

export function getBalances(): Promise<ListResult<AccountBalance>> {
  return apiFetch<ListResult<AccountBalance>>('/api/balances');
}

export function getBalanceHistory(): Promise<BalanceHistoryResponse> {
  return apiFetch<BalanceHistoryResponse>('/api/balances/history');
}

export function getConfig(): Promise<ServerConfig> {
  return apiFetch<ServerConfig>('/api/config');
}

export function getLedgers(): Promise<LedgerListResponse> {
  return apiFetch<LedgerListResponse>('/api/ledgers');
}

export function switchLedger(name: string): Promise<LedgerInfo> {
  return apiFetch<LedgerInfo>('/api/ledgers/switch', {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ name }),
  });
}
