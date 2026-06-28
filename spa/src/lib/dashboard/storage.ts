import { getActiveLedger } from '../filter-memory';
import { CURRENT_VERSION, type DashboardState, type GridItem, type WidgetId } from './types';

function safeStorage(): Storage | null {
  try {
    return typeof localStorage !== 'undefined' ? localStorage : null;
  } catch {
    return null;
  }
}

function key(ledger: string): string {
  return `kea.dashboard.${ledger}`;
}

export function loadDashboardState(): DashboardState | null {
  const s = safeStorage();
  if (!s) return null;
  const ledger = getActiveLedger();
  if (!ledger) return null;
  try {
    const raw = s.getItem(key(ledger));
    if (raw === null) return null;
    const parsed = JSON.parse(raw) as DashboardState;
    if (parsed.version !== CURRENT_VERSION) return null;
    return parsed;
  } catch {
    return null;
  }
}

export function saveDashboardState(state: DashboardState): void {
  const s = safeStorage();
  if (!s) return;
  const ledger = getActiveLedger();
  if (!ledger) return;
  try {
    s.setItem(key(ledger), JSON.stringify(state));
  } catch {
    // quota or unavailable — silently no-op
  }
}

export function resetDashboardState(): void {
  const s = safeStorage();
  if (!s) return;
  const ledger = getActiveLedger();
  if (!ledger) return;
  try {
    s.removeItem(key(ledger));
  } catch {
    // unavailable — silently no-op
  }
}

export function reconcileWithRegistry(
  saved: DashboardState,
  known: WidgetId[],
  defaults: DashboardState,
): DashboardState {
  const knownSet = new Set<WidgetId>(known);
  const layout = saved.layout.filter((g): g is GridItem => knownSet.has(g.i));
  const hidden = saved.hidden.filter((id) => knownSet.has(id));
  const config: Partial<Record<WidgetId, unknown>> = {};
  for (const [id, cfg] of Object.entries(saved.config)) {
    if (knownSet.has(id as WidgetId)) config[id as WidgetId] = cfg;
  }
  const visibleIds = new Set([...layout.map((g) => g.i), ...hidden]);
  const defaultsById = new Map(defaults.layout.map((g) => [g.i, g]));
  let nextY = layout.reduce((m, g) => Math.max(m, g.y + g.h), 0);
  for (const id of known) {
    if (!visibleIds.has(id)) {
      const def = defaultsById.get(id);
      layout.push({
        i: id,
        x: def?.x ?? 0,
        y: nextY,
        w: def?.w ?? 1,
        h: def?.h ?? 1,
      });
      nextY += def?.h ?? 1;
    }
  }
  return { version: CURRENT_VERSION, layout, hidden, config };
}
