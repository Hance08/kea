import type { Budget } from './types';

export type BudgetStatus = 'ok' | 'warning' | 'over';

export const WARN_PCT = 80;

/** Percent of budget used, or null when the budget is 0 (no meaningful ratio). */
export function usedPct(budget: number, actual: number): number | null {
  if (budget === 0) return null;
  return (actual / budget) * 100;
}

export function budgetStatus(budget: number, actual: number): BudgetStatus {
  if (budget === 0) return actual > 0 ? 'over' : 'ok';
  const pct = (actual / budget) * 100;
  if (pct > 100) return 'over';
  if (pct >= WARN_PCT) return 'warning';
  return 'ok';
}

const pad = (n: number) => String(n).padStart(2, '0');

/** Current month in LOCAL time; the server interprets months in local time too. */
export function currentMonth(now: Date = new Date()): string {
  return `${now.getFullYear()}-${pad(now.getMonth() + 1)}`;
}

export function shiftMonth(month: string, delta: number): string {
  const [y, m] = month.split('-').map(Number);
  const d = new Date(y, m - 1 + delta, 1);
  return `${d.getFullYear()}-${pad(d.getMonth() + 1)}`;
}

/** Fraction of the month elapsed (completed days / days in month), only for the current month. */
export function elapsedFraction(month: string, now: Date = new Date()): number | null {
  if (month !== currentMonth(now)) return null;
  const daysInMonth = new Date(now.getFullYear(), now.getMonth() + 1, 0).getDate();
  return (now.getDate() - 1) / daysInMonth;
}

export interface VersionedRow {
  account_id: number;
  account_name: string;
  effective_month: string;
  stopped: boolean;
}

/** Mirrors the server: latest version with effective_month <= month per account, minus stopped ones. */
export function activeVersions<T extends VersionedRow>(items: T[], month: string): T[] {
  const latest = new Map<number, T>();
  for (const v of items) {
    if (v.effective_month > month) continue;
    const cur = latest.get(v.account_id);
    if (!cur || v.effective_month > cur.effective_month) latest.set(v.account_id, v);
  }
  return [...latest.values()]
    .filter((v) => !v.stopped)
    .sort((a, b) => a.account_name.localeCompare(b.account_name));
}

export function activeBudgets(items: Budget[], month: string): Budget[] {
  return activeVersions(items, month);
}

/** Number of other names in `names` that are ancestors of `name` in the account tree. */
export function depthIn(name: string, names: string[]): number {
  return names.filter((n) => n !== name && name.startsWith(`${n}:`)).length;
}

export function parseCents(s: string): number {
  if (s.trim() === '') return Number.NaN;
  const n = Number(s);
  if (!Number.isFinite(n)) return Number.NaN;
  return Math.round(n * 100);
}
