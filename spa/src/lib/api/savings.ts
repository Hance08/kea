import { apiFetch } from '../api';
import type {
  SavingsReport,
  SavingsTarget,
  SavingsTargetListResponse,
  SetSavingsTargetInput,
  StopSavingsTargetInput,
} from '../types';

const jsonInit = (method: string, body: unknown): RequestInit => ({
  method,
  headers: { 'Content-Type': 'application/json' },
  body: JSON.stringify(body),
});

export function fetchSavingsTargets(): Promise<SavingsTargetListResponse> {
  return apiFetch<SavingsTargetListResponse>('/api/savings-targets');
}

export function setSavingsTarget(input: SetSavingsTargetInput): Promise<SavingsTarget> {
  return apiFetch<SavingsTarget>('/api/savings-targets', jsonInit('PUT', input));
}

export function stopSavingsTarget(input: StopSavingsTargetInput): Promise<SavingsTarget> {
  return apiFetch<SavingsTarget>('/api/savings-targets/stop', jsonInit('POST', input));
}

export function deleteSavingsTarget(id: number): Promise<{ deleted: boolean; id: number }> {
  return apiFetch<{ deleted: boolean; id: number }>(`/api/savings-targets/${id}`, {
    method: 'DELETE',
  });
}

// Omitting month lets the server use its current local month.
export function fetchSavingsReport(month?: string): Promise<SavingsReport> {
  const q = month ? `?month=${encodeURIComponent(month)}` : '';
  return apiFetch<SavingsReport>(`/api/reports/savings${q}`);
}
