import { useQuery } from '@tanstack/react-query';
import {
  type PeriodApiParams,
  fetchBalanceSheet,
  fetchExpenseBreakdown,
  fetchIncomeBreakdown,
  fetchIncomeStatement,
  fetchNetWorthSeries,
} from '../api/reports';

export function useIncomeStatement(params: PeriodApiParams) {
  return useQuery({
    queryKey: ['reports', 'income-statement', params],
    queryFn: () => fetchIncomeStatement(params),
  });
}

export function useIncomeBreakdown(params: PeriodApiParams) {
  return useQuery({
    queryKey: ['reports', 'income-breakdown', params],
    queryFn: () => fetchIncomeBreakdown(params),
  });
}

export function useExpenseBreakdown(params: PeriodApiParams) {
  return useQuery({
    queryKey: ['reports', 'expense-breakdown', params],
    queryFn: () => fetchExpenseBreakdown(params),
  });
}

export function useBalanceSheet(params: { as_of?: number }) {
  return useQuery({
    queryKey: ['reports', 'balance-sheet', params],
    queryFn: () => fetchBalanceSheet(params),
  });
}

export function useNetWorthSeries() {
  return useQuery({
    queryKey: ['reports', 'net-worth-series'],
    queryFn: fetchNetWorthSeries,
  });
}
