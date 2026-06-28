import type { ComponentType } from 'react';
import { NetWorthKpi } from '../../components/dashboard/widgets/NetWorthKpi';
import { Placeholder } from '../../components/dashboard/widgets/Placeholder';
import type { WidgetId } from './types';

export interface WidgetMeta<Config = unknown> {
  id: WidgetId;
  title: string;
  defaultConfig: Config;
  component: ComponentType<{ config: Config }>;
  ConfigForm?: ComponentType<{ config: Config; onChange: (c: Config) => void }>;
}

function placeholder(id: WidgetId, title: string): WidgetMeta {
  return {
    id,
    title,
    defaultConfig: {},
    component: () => <Placeholder id={id} />,
  };
}

export const WIDGETS: Record<WidgetId, WidgetMeta> = {
  'net-worth-kpi': {
    id: 'net-worth-kpi',
    title: 'Net Worth',
    defaultConfig: {},
    component: NetWorthKpi,
  },
  'net-worth-trend': placeholder('net-worth-trend', 'Net Worth Trend'),
  'cash-flow-kpi': placeholder('cash-flow-kpi', 'This Month'),
  'per-currency-tiles': placeholder('per-currency-tiles', 'Per-Currency'),
  'top-expense-categories': placeholder('top-expense-categories', 'Top Expense Categories'),
  'recent-transactions': placeholder('recent-transactions', 'Recent Transactions'),
  'biggest-expenses': placeholder('biggest-expenses', 'Biggest Expenses'),
  'accounts-moved': placeholder('accounts-moved', 'Accounts That Moved'),
};

export const ALL_WIDGET_IDS = Object.keys(WIDGETS) as WidgetId[];
