import { redirect } from '@tanstack/react-router';

export type PageId =
  | 'transactions'
  | 'accounts'
  | 'balances'
  | 'reports/balance-sheet'
  | 'reports/income-statement'
  | 'reports/expense-breakdown'
  | 'reports/income-breakdown';

const ACTIVE_LEDGER_KEY = 'kea.activeLedger';

function filterKey(ledger: string, pageId: PageId): string {
  return `kea.filters.${ledger}.${pageId}`;
}

function safeStorage(): Storage | null {
  try {
    return typeof localStorage !== 'undefined' ? localStorage : null;
  } catch {
    return null;
  }
}

export function getActiveLedger(): string | null {
  const s = safeStorage();
  if (!s) return null;
  try {
    return s.getItem(ACTIVE_LEDGER_KEY);
  } catch {
    return null;
  }
}

export function setActiveLedger(name: string): void {
  const s = safeStorage();
  if (!s) return;
  try {
    s.setItem(ACTIVE_LEDGER_KEY, name);
  } catch {
    // quota or unavailable — silently no-op
  }
}

export function loadFilters<T>(pageId: PageId): T | null {
  const s = safeStorage();
  if (!s) return null;
  const ledger = getActiveLedger();
  if (!ledger) return null;
  try {
    const raw = s.getItem(filterKey(ledger, pageId));
    if (raw === null) return null;
    return JSON.parse(raw) as T;
  } catch {
    return null;
  }
}

export function saveFilters(pageId: PageId, search: object): void {
  const s = safeStorage();
  if (!s) return;
  const ledger = getActiveLedger();
  if (!ledger) return;
  try {
    s.setItem(filterKey(ledger, pageId), JSON.stringify(search));
  } catch {
    // quota or unavailable — silently no-op
  }
}

export function clearFilters(pageId: PageId): void {
  const s = safeStorage();
  if (!s) return;
  const ledger = getActiveLedger();
  if (!ledger) return;
  try {
    s.removeItem(filterKey(ledger, pageId));
  } catch {
    // unavailable — silently no-op
  }
}

export function searchEquals(a: object, b: object): boolean {
  const ak = Object.keys(a).filter((k) => (a as Record<string, unknown>)[k] !== undefined);
  const bk = Object.keys(b).filter((k) => (b as Record<string, unknown>)[k] !== undefined);
  if (ak.length !== bk.length) return false;
  for (const k of ak) {
    if ((a as Record<string, unknown>)[k] !== (b as Record<string, unknown>)[k]) {
      return false;
    }
  }
  return true;
}

export function makeFilterMemoryLoader<T extends object>(opts: {
  pageId: PageId;
  defaults: T;
  redirectTo: string;
}): (ctx: { deps: T }) => void {
  return ({ deps: search }) => {
    const ledger = getActiveLedger();
    if (!ledger) return;

    if (searchEquals(search, opts.defaults)) {
      const remembered = loadFilters<T>(opts.pageId);
      if (remembered && !searchEquals(remembered, opts.defaults)) {
        // biome-ignore lint/suspicious/noExplicitAny: redirect.to is route-typed, but this factory is generic over routes
        throw redirect({ to: opts.redirectTo as any, search: remembered as any });
      }
      return;
    }

    saveFilters(opts.pageId, search);
  };
}
