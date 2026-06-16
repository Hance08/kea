import { apiFetch } from '../api';
import type { BalanceSheetResult, NetWorthResponse, ReportResult } from '../types';

function buildQuery(params: { [k: string]: string | number | undefined }): string {
  const usp = new URLSearchParams();
  for (const [k, v] of Object.entries(params)) {
    if (v === undefined || v === '') continue;
    usp.set(k, String(v));
  }
  const s = usp.toString();
  return s ? `?${s}` : '';
}

export interface PeriodApiParams {
  [k: string]: string | undefined;
  month?: string;
  from?: string;
  to?: string;
}

export function fetchIncomeStatement(params: PeriodApiParams): Promise<ReportResult> {
  return apiFetch<ReportResult>(`/api/reports/income-statement${buildQuery(params)}`);
}

export function fetchIncomeBreakdown(params: PeriodApiParams): Promise<ReportResult> {
  return apiFetch<ReportResult>(`/api/reports/income-breakdown${buildQuery(params)}`);
}

export function fetchExpenseBreakdown(params: PeriodApiParams): Promise<ReportResult> {
  return apiFetch<ReportResult>(`/api/reports/expense-breakdown${buildQuery(params)}`);
}

export function fetchBalanceSheet(params: { [k: string]: number | undefined; as_of?: number }): Promise<BalanceSheetResult> {
  return apiFetch<BalanceSheetResult>(`/api/reports/balance-sheet${buildQuery(params)}`);
}

export function fetchNetWorth(params: { [k: string]: number | undefined; at?: number }): Promise<NetWorthResponse> {
  return apiFetch<NetWorthResponse>(`/api/reports/net-worth${buildQuery(params)}`);
}
