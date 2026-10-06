import { useQuery } from '@tanstack/react-query';
import { fetchSavingsReport, fetchSavingsTargets } from '../api/savings';

export function useSavingsTargets() {
  return useQuery({ queryKey: ['savings', 'list'], queryFn: fetchSavingsTargets });
}

export function useSavingsReport(month?: string) {
  return useQuery({
    queryKey: ['savings', 'report', month ?? 'current'],
    queryFn: () => fetchSavingsReport(month),
  });
}
