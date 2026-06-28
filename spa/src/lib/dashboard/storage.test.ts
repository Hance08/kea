import { beforeEach, describe, expect, it } from 'vitest';
import {
  loadDashboardState,
  reconcileWithRegistry,
  saveDashboardState,
  resetDashboardState,
} from './storage';
import type { DashboardState, WidgetId } from './types';

const ALL_IDS: WidgetId[] = [
  'net-worth-kpi',
  'net-worth-trend',
  'cash-flow-kpi',
  'per-currency-tiles',
  'top-expense-categories',
  'recent-transactions',
  'biggest-expenses',
  'accounts-moved',
];

const defaults: DashboardState = {
  version: 1,
  layout: ALL_IDS.map((i, idx) => ({ i, x: 0, y: idx, w: 1, h: 1 })),
  hidden: [],
  config: {},
};

beforeEach(() => {
  localStorage.clear();
  localStorage.setItem('kea.activeLedger', 'main');
});

describe('storage', () => {
  it('returns null when no value is saved', () => {
    expect(loadDashboardState()).toBeNull();
  });

  it('round-trips a saved state', () => {
    saveDashboardState(defaults);
    expect(loadDashboardState()).toEqual(defaults);
  });

  it('returns null when no active ledger is set', () => {
    localStorage.removeItem('kea.activeLedger');
    saveDashboardState(defaults);
    expect(loadDashboardState()).toBeNull();
  });

  it('returns null on version mismatch', () => {
    localStorage.setItem(
      'kea.dashboard.main',
      JSON.stringify({ ...defaults, version: 999 }),
    );
    expect(loadDashboardState()).toBeNull();
  });

  it('returns null on corrupt JSON without throwing', () => {
    localStorage.setItem('kea.dashboard.main', '{not json');
    expect(loadDashboardState()).toBeNull();
  });

  it('resetDashboardState removes the saved state', () => {
    saveDashboardState(defaults);
    resetDashboardState();
    expect(loadDashboardState()).toBeNull();
  });
});

describe('reconcileWithRegistry', () => {
  it('drops unknown ids from layout, hidden, and config', () => {
    const known: WidgetId[] = ['net-worth-kpi'];
    // Cast the whole literal to DashboardState because TS rejects unknown
    // string keys in Partial<Record<WidgetId, unknown>>, even with `as any` on
    // individual values. We're intentionally simulating a stale saved state.
    const saved = {
      version: 1,
      layout: [
        { i: 'net-worth-kpi', x: 0, y: 0, w: 1, h: 1 },
        { i: 'removed-widget', x: 1, y: 0, w: 1, h: 1 },
      ],
      hidden: ['removed-widget'],
      config: { 'removed-widget': { foo: 1 } },
    } as unknown as DashboardState;
    const result = reconcileWithRegistry(saved, known, defaults);
    expect(result.layout.map((i) => i.i)).toEqual(['net-worth-kpi']);
    expect(result.hidden).toEqual([]);
    expect(result.config).toEqual({});
  });

  it('appends registry widgets missing from layout to the bottom', () => {
    const saved: DashboardState = {
      version: 1,
      layout: [{ i: 'net-worth-kpi', x: 0, y: 0, w: 1, h: 1 }],
      hidden: [],
      config: {},
    };
    const result = reconcileWithRegistry(saved, ALL_IDS, defaults);
    expect(result.layout.map((i) => i.i)).toEqual(ALL_IDS);
    // first item keeps its saved position
    expect(result.layout[0]).toEqual({ i: 'net-worth-kpi', x: 0, y: 0, w: 1, h: 1 });
  });

  it('does not append widgets that are explicitly hidden', () => {
    const saved: DashboardState = {
      version: 1,
      layout: [{ i: 'net-worth-kpi', x: 0, y: 0, w: 1, h: 1 }],
      hidden: ['net-worth-trend'],
      config: {},
    };
    const result = reconcileWithRegistry(saved, ALL_IDS, defaults);
    expect(result.layout.map((i) => i.i)).not.toContain('net-worth-trend');
    expect(result.hidden).toContain('net-worth-trend');
  });
});
