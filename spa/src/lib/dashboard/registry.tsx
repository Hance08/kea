import type { ComponentType } from 'react';
import { CashFlowKpi } from '../../components/dashboard/widgets/CashFlowKpi';
import { NetWorthKpi } from '../../components/dashboard/widgets/NetWorthKpi';
import {
  NET_WORTH_TREND_DEFAULT,
  NetWorthTrend,
  NetWorthTrendConfigForm,
} from '../../components/dashboard/widgets/NetWorthTrend';
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
  'net-worth-trend': {
    id: 'net-worth-trend',
    title: 'Net Worth Trend',
    defaultConfig: NET_WORTH_TREND_DEFAULT,
    component: NetWorthTrend,
    ConfigForm: NetWorthTrendConfigForm,
  } as WidgetMeta,
  'cash-flow-kpi': {
    id: 'cash-flow-kpi',
    title: 'This Month',
    defaultConfig: {},
    component: CashFlowKpi,
  },
  'per-currency-tiles': placeholder('per-currency-tiles', 'Per-Currency'),
  'top-expense-categories': placeholder('top-expense-categories', 'Top Expense Categories'),
  'recent-transactions': placeholder('recent-transactions', 'Recent Transactions'),
  'biggest-expenses': placeholder('biggest-expenses', 'Biggest Expenses'),
  'accounts-moved': placeholder('accounts-moved', 'Accounts That Moved'),
};

export const ALL_WIDGET_IDS = Object.keys(WIDGETS) as WidgetId[];
