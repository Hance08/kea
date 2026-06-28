export type WidgetId =
  | 'net-worth-kpi'
  | 'net-worth-trend'
  | 'cash-flow-kpi'
  | 'per-currency-tiles'
  | 'top-expense-categories'
  | 'recent-transactions'
  | 'biggest-expenses'
  | 'accounts-moved';

export interface GridItem {
  i: WidgetId;
  x: number;
  y: number;
  w: number;
  h: number;
}

export interface DashboardState {
  version: 1;
  layout: GridItem[];
  hidden: WidgetId[];
  config: Partial<Record<WidgetId, unknown>>;
}

export const CURRENT_VERSION = 1 as const;
