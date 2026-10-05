import { apiFetch } from '../api';
import type {
  Budget,
  BudgetListResponse,
  BudgetReport,
  SetBudgetInput,
  StopBudgetInput,
} from '../types';

const jsonInit = (method: string, body: unknown): RequestInit => ({
  method,
  headers: { 'Content-Type': 'application/json' },
  body: JSON.stringify(body),
});

export function fetchBudgets(): Promise<BudgetListResponse> {
  return apiFetch<BudgetListResponse>('/api/budgets');
}

export function setBudget(input: SetBudgetInput): Promise<Budget> {
  return apiFetch<Budget>('/api/budgets', jsonInit('PUT', input));
}

export function stopBudget(input: StopBudgetInput): Promise<Budget> {
  return apiFetch<Budget>('/api/budgets/stop', jsonInit('POST', input));
}

export function deleteBudget(id: number): Promise<{ deleted: boolean; id: number }> {
  return apiFetch<{ deleted: boolean; id: number }>(`/api/budgets/${id}`, { method: 'DELETE' });
}

// Omitting month lets the server use its current local month.
export function fetchBudgetReport(month?: string): Promise<BudgetReport> {
  const q = month ? `?month=${encodeURIComponent(month)}` : '';
  return apiFetch<BudgetReport>(`/api/reports/budget${q}`);
}
