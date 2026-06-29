import type { ComponentType } from 'react';
import {
  ACCOUNTS_MOVED_DEFAULT,
  AccountsMoved,
  AccountsMovedConfigForm,
} from '../../components/dashboard/widgets/AccountsMoved';
import {
  BIGGEST_EXPENSES_DEFAULT,
  BiggestExpenses,
  BiggestExpensesConfigForm,
} from '../../components/dashboard/widgets/BiggestExpenses';
import { CashFlowKpi, CashFlowKpiHeader } from '../../components/dashboard/widgets/CashFlowKpi';
import { NetWorthKpi, NetWorthKpiHeader } from '../../components/dashboard/widgets/NetWorthKpi';
import {
  NET_WORTH_TREND_DEFAULT,
  NetWorthTrend,
  NetWorthTrendConfigForm,
} from '../../components/dashboard/widgets/NetWorthTrend';
import { PerCurrencyTiles } from '../../components/dashboard/widgets/PerCurrencyTiles';
import {
  RECENT_TXN_DEFAULT,
  RecentTransactions,
  RecentTxnConfigForm,
} from '../../components/dashboard/widgets/RecentTransactions';
import {
  TOP_EXPENSE_DEFAULT,
  TopExpenseCategories,
  TopExpenseConfigForm,
} from '../../components/dashboard/widgets/TopExpenseCategories';
import type { WidgetId } from './types';

export interface WidgetMeta<Config = unknown> {
  id: WidgetId;
  title: string;
  defaultConfig: Config;
  component: ComponentType<{ config: Config }>;
  HeaderRight?: ComponentType<{ config: Config }>;
  ConfigForm?: ComponentType<{ config: Config; onChange: (c: Config) => void }>;
}

export const WIDGETS: Record<WidgetId, WidgetMeta> = {
  'net-worth-kpi': {
    id: 'net-worth-kpi',
    title: 'Net Worth',
    defaultConfig: {},
    component: NetWorthKpi,
    HeaderRight: NetWorthKpiHeader,
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
    HeaderRight: CashFlowKpiHeader,
  },
  'per-currency-tiles': {
    id: 'per-currency-tiles',
    title: 'Per-Currency Net Worth',
    defaultConfig: {},
    component: PerCurrencyTiles,
  },
  'top-expense-categories': {
    id: 'top-expense-categories',
    title: 'Top Expense Categories',
    defaultConfig: TOP_EXPENSE_DEFAULT,
    component: TopExpenseCategories,
    ConfigForm: TopExpenseConfigForm,
  } as WidgetMeta,
  'recent-transactions': {
    id: 'recent-transactions',
    title: 'Recent Transactions',
    defaultConfig: RECENT_TXN_DEFAULT,
    component: RecentTransactions,
    ConfigForm: RecentTxnConfigForm,
  } as WidgetMeta,
  'biggest-expenses': {
    id: 'biggest-expenses',
    title: 'Biggest Expenses',
    defaultConfig: BIGGEST_EXPENSES_DEFAULT,
    component: BiggestExpenses,
    ConfigForm: BiggestExpensesConfigForm,
  } as WidgetMeta,
  'accounts-moved': {
    id: 'accounts-moved',
    title: 'Accounts That Moved Most',
    defaultConfig: ACCOUNTS_MOVED_DEFAULT,
    component: AccountsMoved,
    ConfigForm: AccountsMovedConfigForm,
  } as WidgetMeta,
};

export const ALL_WIDGET_IDS = Object.keys(WIDGETS) as WidgetId[];
