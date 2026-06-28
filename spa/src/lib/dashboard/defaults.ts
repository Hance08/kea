import type { DashboardState, GridItem } from './types';

// Default desktop layout (3-col grid). Layout reads top-to-bottom, left-to-right:
//   row 0: [net-worth-kpi  ] [cash-flow-kpi    ] [per-currency-tiles]
//   row 1: [net-worth-trend       (2 wide, 2 tall)] [top-expense-categories]
//   row 3: [biggest-expenses ] [accounts-moved   ] [recent-transactions]
const DEFAULT_LAYOUT: GridItem[] = [
  { i: 'net-worth-kpi', x: 0, y: 0, w: 1, h: 1 },
  { i: 'cash-flow-kpi', x: 1, y: 0, w: 1, h: 1 },
  { i: 'per-currency-tiles', x: 2, y: 0, w: 1, h: 1 },
  { i: 'net-worth-trend', x: 0, y: 1, w: 2, h: 2 },
  { i: 'top-expense-categories', x: 2, y: 1, w: 1, h: 2 },
  { i: 'biggest-expenses', x: 0, y: 3, w: 1, h: 2 },
  { i: 'accounts-moved', x: 1, y: 3, w: 1, h: 2 },
  { i: 'recent-transactions', x: 2, y: 3, w: 1, h: 2 },
];

export const DEFAULT_STATE: DashboardState = {
  version: 1,
  layout: DEFAULT_LAYOUT,
  hidden: [],
  config: {},
};
