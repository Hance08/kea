import { apiFetch } from '../api';
import type { BalanceSheetResult, NetWorthResponse, ReportResult } from '../types';

function buildQuery(params: Record<string, string | number | undefined>): string {
  const usp = new URLSearchParams();
  for (const [k, v] of Object.entries(params)) {
    if (v === undefined || v === '') continue;
    usp.set(k, String(v));
  }
  const s = usp.toString();
  return s ? `?${s}` : '';
}

export interface PeriodApiParams {
  month?: string;
  from?: string;
  to?: string;
}

export function fetchIncomeStatement(params: PeriodApiParams): Promise<ReportResult> {
  return apiFetch<ReportResult>(`/api/reports/income-statement${buildQuery(params as Record<string, string | undefined>)}`);
}

export function fetchIncomeBreakdown(params: PeriodApiParams): Promise<ReportResult> {
  return apiFetch<ReportResult>(`/api/reports/income-breakdown${buildQuery(params as Record<string, string | undefined>)}`);
}

export function fetchExpenseBreakdown(params: PeriodApiParams): Promise<ReportResult> {
  return apiFetch<ReportResult>(`/api/reports/expense-breakdown${buildQuery(params as Record<string, string | undefined>)}`);
}

export function fetchBalanceSheet(params: { as_of?: number }): Promise<BalanceSheetResult> {
  return apiFetch<BalanceSheetResult>(`/api/reports/balance-sheet${buildQuery(params as Record<string, number | undefined>)}`);
}

export function fetchNetWorth(params: { at?: number }): Promise<NetWorthResponse> {
  return apiFetch<NetWorthResponse>(`/api/reports/net-worth${buildQuery(params as Record<string, number | undefined>)}`);
}
