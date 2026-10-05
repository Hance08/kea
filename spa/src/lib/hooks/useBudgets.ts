import { useQuery } from '@tanstack/react-query';
import { fetchBudgetReport, fetchBudgets } from '../api/budgets';

export function useBudgets() {
  return useQuery({ queryKey: ['budgets', 'list'], queryFn: fetchBudgets });
}

export function useBudgetReport(month?: string) {
  return useQuery({
    queryKey: ['budgets', 'report', month ?? 'current'],
    queryFn: () => fetchBudgetReport(month),
  });
}
