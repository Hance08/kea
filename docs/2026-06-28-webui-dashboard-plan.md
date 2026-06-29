# WebUI Dashboard Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Build a customizable dashboard at `/dashboard` that replaces the current `/` → `/balances` redirect, with 8 widgets (net worth KPI, net worth trend, cash flow, top expense categories, per-currency tiles, recent transactions, biggest expenses, accounts that moved), drag-and-drop reorder, resize, show/hide, and per-widget config.

**Architecture:** Single React SPA route. Widget registry maps id → component + meta. Layout/visibility/config persisted in `localStorage` per-ledger using the existing `filter-memory` pattern. `react-grid-layout` handles drag/resize/responsive grid. Each widget owns its own TanStack Query call. No backend changes.

**Tech Stack:** React 18, TypeScript, TanStack Router (file-based routing), TanStack Query, `react-grid-layout` (new dep), vitest + @testing-library/react, biome for formatting.

**Reference spec:** [docs/2026-06-28-webui-dashboard-design.md](2026-06-28-webui-dashboard-design.md)

**Working directory for all `npm`/`vitest` commands:** `spa/`

**Key existing helpers (do NOT reinvent):**
- Amount formatting: `formatCents(cents, currency, { hideDecimals? })` from `spa/src/lib/format.ts`. Convenience hook `useAmountFormat()` from `spa/src/lib/server-config.tsx` returns a `formatCents` bound to the server's `hide_decimals` setting.
- Server config: `useServerConfig()` returns `ServerConfig` (`{ defaults: { currency: string }, display: { hide_decimals: boolean } }`). Access the user's default currency as `useServerConfig().defaults.currency`.
- Transactions list fetch: `listTransactions(filter, opts)` from `spa/src/lib/transactions.ts` (already exists). Do NOT add a new `getTransactions` to `api.ts`.
- Net-worth-series fetch: `fetchNetWorthSeries()` from `spa/src/lib/api/reports.ts` (already exists).
- Balance history fetch: `getBalanceHistory()` from `spa/src/lib/api.ts` (already exists).
- Income/expense/breakdown hooks: `useIncomeStatement`, `useExpenseBreakdown`, `useIncomeBreakdown` from `spa/src/lib/hooks/useReport.ts`.
- `ReportResult` shape (`spa/src/lib/types.ts`): `total_income: Record<currency, cents>`, `total_expense: Record<currency, cents>`, `net_amount`, `net_worth`, `income_rows: ReportRow[]`, `expense_rows: ReportRow[]`. `ReportRow` is `{ account_name, offset_account, amount: cents, currency, tx_count }`.

---

## Phase 0 — Setup

### Task 0.1: Add react-grid-layout dependency

**Files:**
- Modify: `spa/package.json`

- [ ] **Step 1: Install the dep**

Run from `spa/`:
```bash
npm install react-grid-layout@^1.4.4
npm install -D @types/react-grid-layout@^1.3.5
```

- [ ] **Step 2: Verify build still passes**

Run from `spa/`:
```bash
npm run build
```
Expected: build completes without TS errors.

- [ ] **Step 3: Commit**

```bash
git add spa/package.json spa/package-lock.json
git commit -m "build(spa): add react-grid-layout for dashboard widgets"
```

---

## Phase 1 — Foundation: types, storage, registry skeleton

### Task 1.1: Define dashboard types

**Files:**
- Create: `spa/src/lib/dashboard/types.ts`

- [ ] **Step 1: Create the types file**

```ts
// spa/src/lib/dashboard/types.ts
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
```

- [ ] **Step 2: Verify it compiles**

Run from `spa/`:
```bash
npx tsc --noEmit
```
Expected: no errors.

- [ ] **Step 3: Commit**

```bash
git add spa/src/lib/dashboard/types.ts
git commit -m "feat(spa): add dashboard types"
```

---

### Task 1.2: Storage module (TDD)

**Files:**
- Create: `spa/src/lib/dashboard/storage.ts`
- Create: `spa/src/lib/dashboard/storage.test.ts`
- Reference: `spa/src/lib/filter-memory.ts` (existing per-ledger localStorage pattern)

- [ ] **Step 1: Write the failing test**

```ts
// spa/src/lib/dashboard/storage.test.ts
import { beforeEach, describe, expect, it, vi } from 'vitest';
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
    const saved: DashboardState = {
      version: 1,
      layout: [
        { i: 'net-worth-kpi', x: 0, y: 0, w: 1, h: 1 },
        // biome-ignore lint/suspicious/noExplicitAny: testing unknown id
        { i: 'removed-widget' as any, x: 1, y: 0, w: 1, h: 1 },
      ],
      // biome-ignore lint/suspicious/noExplicitAny: testing unknown id
      hidden: ['removed-widget' as any],
      // biome-ignore lint/suspicious/noExplicitAny: testing unknown id
      config: { 'removed-widget': { foo: 1 } as any },
    };
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
```

- [ ] **Step 2: Run the test to verify it fails**

Run from `spa/`:
```bash
npx vitest run src/lib/dashboard/storage.test.ts
```
Expected: FAIL — module `./storage` does not exist.

- [ ] **Step 3: Implement storage.ts**

```ts
// spa/src/lib/dashboard/storage.ts
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
```

- [ ] **Step 4: Run the test to verify it passes**

Run from `spa/`:
```bash
npx vitest run src/lib/dashboard/storage.test.ts
```
Expected: all tests PASS.

- [ ] **Step 5: Commit**

```bash
git add spa/src/lib/dashboard/storage.ts spa/src/lib/dashboard/storage.test.ts
git commit -m "feat(spa): add dashboard per-ledger localStorage with registry reconciliation"
```

---

### Task 1.3: Widget registry skeleton

**Files:**
- Create: `spa/src/lib/dashboard/registry.ts`
- Create: `spa/src/lib/dashboard/defaults.ts`

This task creates the registry **with all 8 widget metadata entries pointing to a placeholder component**. Each widget gets its real component in Phase 3 (Tasks 3.1–3.8). The placeholder lets us build the dashboard shell first without blocking on widget implementations.

- [ ] **Step 1: Create the placeholder component**

```tsx
// spa/src/components/dashboard/widgets/Placeholder.tsx
export function Placeholder({ id }: { id: string }) {
  return (
    <div className="flex h-full items-center justify-center text-sm text-muted-foreground">
      {id} — coming soon
    </div>
  );
}
```

- [ ] **Step 2: Create the defaults file**

`defaults.ts` defines the initial grid layout (used when a user has no saved state). Grid is 3 columns on desktop.

```ts
// spa/src/lib/dashboard/defaults.ts
import type { DashboardState, GridItem } from './types';

// Default desktop layout (3-col grid). Layout reads top-to-bottom, left-to-right:
//   row 0: [net-worth-kpi  ] [cash-flow-kpi    ] [per-currency-tiles]
//   row 1: [net-worth-trend       (2 wide, 2 tall)] [top-expense-categories]
//   row 3: [biggest-expenses ] [accounts-moved   ] [recent-transactions]
const DEFAULT_LAYOUT: GridItem[] = [
  { i: 'net-worth-kpi',          x: 0, y: 0, w: 1, h: 1 },
  { i: 'cash-flow-kpi',          x: 1, y: 0, w: 1, h: 1 },
  { i: 'per-currency-tiles',     x: 2, y: 0, w: 1, h: 1 },
  { i: 'net-worth-trend',        x: 0, y: 1, w: 2, h: 2 },
  { i: 'top-expense-categories', x: 2, y: 1, w: 1, h: 2 },
  { i: 'biggest-expenses',       x: 0, y: 3, w: 1, h: 2 },
  { i: 'accounts-moved',         x: 1, y: 3, w: 1, h: 2 },
  { i: 'recent-transactions',    x: 2, y: 3, w: 1, h: 2 },
];

export const DEFAULT_STATE: DashboardState = {
  version: 1,
  layout: DEFAULT_LAYOUT,
  hidden: [],
  config: {},
};
```

- [ ] **Step 3: Create the registry**

```tsx
// spa/src/lib/dashboard/registry.ts
import type { ComponentType } from 'react';
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
  'net-worth-kpi':          placeholder('net-worth-kpi', 'Net Worth'),
  'net-worth-trend':        placeholder('net-worth-trend', 'Net Worth Trend'),
  'cash-flow-kpi':          placeholder('cash-flow-kpi', 'This Month'),
  'per-currency-tiles':     placeholder('per-currency-tiles', 'Per-Currency'),
  'top-expense-categories': placeholder('top-expense-categories', 'Top Expense Categories'),
  'recent-transactions':    placeholder('recent-transactions', 'Recent Transactions'),
  'biggest-expenses':       placeholder('biggest-expenses', 'Biggest Expenses'),
  'accounts-moved':         placeholder('accounts-moved', 'Accounts That Moved'),
};

export const ALL_WIDGET_IDS = Object.keys(WIDGETS) as WidgetId[];
```

- [ ] **Step 4: Verify TS compiles**

Run from `spa/`:
```bash
npx tsc --noEmit
```
Expected: no errors.

- [ ] **Step 5: Commit**

```bash
git add spa/src/lib/dashboard/registry.ts spa/src/lib/dashboard/defaults.ts \
        spa/src/components/dashboard/widgets/Placeholder.tsx
git commit -m "feat(spa): add dashboard widget registry skeleton with placeholders"
```

---

## Phase 2 — Dashboard shell

### Task 2.1: Dashboard component (read-only render)

**Files:**
- Create: `spa/src/components/dashboard/Dashboard.tsx`
- Create: `spa/src/components/dashboard/Dashboard.test.tsx`
- Create: `spa/src/components/dashboard/grid.css` (imports react-grid-layout's CSS)

- [ ] **Step 1: Create the grid CSS shim**

```css
/* spa/src/components/dashboard/grid.css */
@import 'react-grid-layout/css/styles.css';
@import 'react-resizable/css/styles.css';

.dashboard-grid .react-grid-item.react-grid-placeholder {
  background: hsl(var(--primary) / 0.2);
  border: 1px dashed hsl(var(--primary));
  border-radius: 0.5rem;
}
```

- [ ] **Step 2: Write the failing test**

```tsx
// spa/src/components/dashboard/Dashboard.test.tsx
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { render, screen } from '@testing-library/react';
import { beforeEach, describe, expect, it } from 'vitest';
import { ServerConfigContext } from '../../lib/server-config';
import type { ServerConfig } from '../../lib/types';
import { Dashboard } from './Dashboard';

const TEST_CONFIG: ServerConfig = {
  defaults: { currency: 'USD' },
  display: { hide_decimals: false },
};

function renderDashboard() {
  const client = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  return render(
    <QueryClientProvider client={client}>
      <ServerConfigContext.Provider value={TEST_CONFIG}>
        <Dashboard />
      </ServerConfigContext.Provider>
    </QueryClientProvider>,
  );
}

beforeEach(() => {
  localStorage.clear();
  localStorage.setItem('kea.activeLedger', 'main');
});

describe('Dashboard', () => {
  it('renders 8 grid items by default (one per registered widget)', () => {
    const { container } = renderDashboard();
    // react-grid-layout assigns `react-grid-item` class to each item.
    expect(container.querySelectorAll('.react-grid-item').length).toBe(8);
  });
});
```

- [ ] **Step 3: Run the test to verify it fails**

Run from `spa/`:
```bash
npx vitest run src/components/dashboard/Dashboard.test.tsx
```
Expected: FAIL — Dashboard not exported.

- [ ] **Step 4: Implement Dashboard.tsx (read-only)**

```tsx
// spa/src/components/dashboard/Dashboard.tsx
import { useEffect, useMemo, useState } from 'react';
import GridLayout, { WidthProvider } from 'react-grid-layout';
import { DEFAULT_STATE } from '../../lib/dashboard/defaults';
import { ALL_WIDGET_IDS, WIDGETS } from '../../lib/dashboard/registry';
import {
  loadDashboardState,
  reconcileWithRegistry,
  saveDashboardState,
} from '../../lib/dashboard/storage';
import type { DashboardState, GridItem } from '../../lib/dashboard/types';
import './grid.css';

const ResponsiveGrid = WidthProvider(GridLayout);

const COLS = 3;
const ROW_HEIGHT = 80;

export function Dashboard() {
  const [state, setState] = useState<DashboardState>(() => {
    const saved = loadDashboardState();
    return saved ? reconcileWithRegistry(saved, ALL_WIDGET_IDS, DEFAULT_STATE) : DEFAULT_STATE;
  });

  // Debounced save on state change.
  useEffect(() => {
    const t = setTimeout(() => saveDashboardState(state), 500);
    return () => clearTimeout(t);
  }, [state]);

  const visibleItems = useMemo(
    () => state.layout.filter((g) => !state.hidden.includes(g.i)),
    [state.layout, state.hidden],
  );

  return (
    <div className="dashboard-grid p-4">
      <ResponsiveGrid
        cols={COLS}
        rowHeight={ROW_HEIGHT}
        layout={visibleItems}
        isDraggable={false}
        isResizable={false}
        margin={[16, 16]}
      >
        {visibleItems.map((item: GridItem) => {
          const meta = WIDGETS[item.i];
          const cfg = state.config[item.i] ?? meta.defaultConfig;
          const Comp = meta.component;
          return (
            <div
              key={item.i}
              className="rounded-lg border bg-card p-3 shadow-sm overflow-hidden"
            >
              <Comp config={cfg} />
            </div>
          );
        })}
      </ResponsiveGrid>
    </div>
  );
}
```

- [ ] **Step 5: Run the test to verify it passes**

Run from `spa/`:
```bash
npx vitest run src/components/dashboard/Dashboard.test.tsx
```
Expected: PASS.

- [ ] **Step 6: Commit**

```bash
git add spa/src/components/dashboard/Dashboard.tsx \
        spa/src/components/dashboard/Dashboard.test.tsx \
        spa/src/components/dashboard/grid.css
git commit -m "feat(spa): add read-only dashboard shell with grid layout"
```

---

### Task 2.2: Edit mode and WidgetFrame

**Files:**
- Create: `spa/src/components/dashboard/WidgetFrame.tsx`
- Create: `spa/src/components/dashboard/EditModeHeader.tsx`
- Modify: `spa/src/components/dashboard/Dashboard.tsx`
- Modify: `spa/src/components/dashboard/Dashboard.test.tsx`

- [ ] **Step 1: Write the failing tests**

Append to `Dashboard.test.tsx`:
```tsx
import { userEvent } from '@testing-library/user-event';

describe('Dashboard edit mode', () => {
  it('toggles edit mode and reveals hide buttons', async () => {
    const user = userEvent.setup();
    renderDashboard();
    expect(screen.queryByLabelText(/hide net-worth-kpi/i)).toBeNull();
    await user.click(screen.getByRole('button', { name: /^edit$/i }));
    expect(screen.getByLabelText(/hide net-worth-kpi/i)).toBeInTheDocument();
  });

  it('hides a widget when clicking its hide button', async () => {
    const user = userEvent.setup();
    const { container } = renderDashboard();
    expect(container.querySelectorAll('.react-grid-item').length).toBe(8);
    await user.click(screen.getByRole('button', { name: /^edit$/i }));
    await user.click(screen.getByLabelText(/hide net-worth-kpi/i));
    await user.click(screen.getByRole('button', { name: /^done$/i }));
    expect(container.querySelectorAll('.react-grid-item').length).toBe(7);
  });
});
```

- [ ] **Step 2: Run the tests to verify they fail**

Run from `spa/`:
```bash
npx vitest run src/components/dashboard/Dashboard.test.tsx
```
Expected: FAIL — Edit button not found.

- [ ] **Step 3: Create WidgetFrame**

```tsx
// spa/src/components/dashboard/WidgetFrame.tsx
import { X } from 'lucide-react';
import type { ReactNode } from 'react';
import type { WidgetMeta } from '../../lib/dashboard/registry';

interface Props {
  meta: WidgetMeta;
  editing: boolean;
  onHide: () => void;
  children: ReactNode;
}

export function WidgetFrame({ meta, editing, onHide, children }: Props) {
  if (!editing) return <>{children}</>;
  return (
    <div className="flex h-full flex-col">
      <div className="dashboard-drag-handle flex shrink-0 items-center justify-between border-b px-2 py-1 text-xs font-medium cursor-move bg-muted/30">
        <span className="truncate">{meta.title}</span>
        <button
          type="button"
          aria-label={`hide ${meta.id}`}
          onClick={(e) => {
            e.stopPropagation();
            onHide();
          }}
          className="rounded p-0.5 hover:bg-muted"
        >
          <X className="h-3.5 w-3.5" />
        </button>
      </div>
      <div className="flex-1 overflow-hidden p-2">{children}</div>
    </div>
  );
}
```

- [ ] **Step 4: Create EditModeHeader**

```tsx
// spa/src/components/dashboard/EditModeHeader.tsx
interface Props {
  editing: boolean;
  onToggle: () => void;
}

export function EditModeHeader({ editing, onToggle }: Props) {
  return (
    <div className="mb-4 flex items-center justify-between">
      <h1 className="text-2xl font-semibold">Dashboard</h1>
      <button
        type="button"
        onClick={onToggle}
        className="rounded-md border px-3 py-1.5 text-sm font-medium hover:bg-muted"
      >
        {editing ? 'Done' : 'Edit'}
      </button>
    </div>
  );
}
```

- [ ] **Step 5: Wire edit mode into Dashboard**

Replace `Dashboard.tsx` with:
```tsx
import { useEffect, useMemo, useState } from 'react';
import GridLayout, { WidthProvider, type Layout } from 'react-grid-layout';
import { DEFAULT_STATE } from '../../lib/dashboard/defaults';
import { ALL_WIDGET_IDS, WIDGETS } from '../../lib/dashboard/registry';
import {
  loadDashboardState,
  reconcileWithRegistry,
  saveDashboardState,
} from '../../lib/dashboard/storage';
import type { DashboardState, GridItem, WidgetId } from '../../lib/dashboard/types';
import { EditModeHeader } from './EditModeHeader';
import { WidgetFrame } from './WidgetFrame';
import './grid.css';

const ResponsiveGrid = WidthProvider(GridLayout);
const COLS = 3;
const ROW_HEIGHT = 80;

export function Dashboard() {
  const [editing, setEditing] = useState(false);
  const [state, setState] = useState<DashboardState>(() => {
    const saved = loadDashboardState();
    return saved
      ? reconcileWithRegistry(saved, ALL_WIDGET_IDS, DEFAULT_STATE)
      : DEFAULT_STATE;
  });

  useEffect(() => {
    const t = setTimeout(() => saveDashboardState(state), 500);
    return () => clearTimeout(t);
  }, [state]);

  const visibleItems = useMemo(
    () => state.layout.filter((g) => !state.hidden.includes(g.i)),
    [state.layout, state.hidden],
  );

  function hideWidget(id: WidgetId) {
    setState((s) => ({ ...s, hidden: [...s.hidden, id] }));
  }

  function onLayoutChange(next: Layout[]) {
    setState((s) => {
      const byId = new Map(next.map((n) => [n.i as WidgetId, n]));
      const updated = s.layout.map<GridItem>((g) => {
        const n = byId.get(g.i);
        return n ? { i: g.i, x: n.x, y: n.y, w: n.w, h: n.h } : g;
      });
      return { ...s, layout: updated };
    });
  }

  return (
    <div className="p-4">
      <EditModeHeader editing={editing} onToggle={() => setEditing((e) => !e)} />
      <div className="dashboard-grid">
        <ResponsiveGrid
          cols={COLS}
          rowHeight={ROW_HEIGHT}
          layout={visibleItems}
          isDraggable={editing}
          isResizable={editing}
          draggableHandle=".dashboard-drag-handle"
          onLayoutChange={onLayoutChange}
          margin={[16, 16]}
        >
          {visibleItems.map((item: GridItem) => {
            const meta = WIDGETS[item.i];
            const cfg = state.config[item.i] ?? meta.defaultConfig;
            const Comp = meta.component;
            return (
              <div
                key={item.i}
                className="rounded-lg border bg-card shadow-sm overflow-hidden"
              >
                <WidgetFrame meta={meta} editing={editing} onHide={() => hideWidget(item.i)}>
                  <Comp config={cfg} />
                </WidgetFrame>
              </div>
            );
          })}
        </ResponsiveGrid>
      </div>
    </div>
  );
}
```

- [ ] **Step 6: Run all dashboard tests to verify they pass**

Run from `spa/`:
```bash
npx vitest run src/components/dashboard/
```
Expected: PASS.

- [ ] **Step 7: Commit**

```bash
git add spa/src/components/dashboard/WidgetFrame.tsx \
        spa/src/components/dashboard/EditModeHeader.tsx \
        spa/src/components/dashboard/Dashboard.tsx \
        spa/src/components/dashboard/Dashboard.test.tsx
git commit -m "feat(spa): add dashboard edit mode with hide and drag/resize"
```

---

### Task 2.3: AddWidgetMenu for hidden widgets

**Files:**
- Create: `spa/src/components/dashboard/AddWidgetMenu.tsx`
- Modify: `spa/src/components/dashboard/EditModeHeader.tsx`
- Modify: `spa/src/components/dashboard/Dashboard.tsx`
- Modify: `spa/src/components/dashboard/Dashboard.test.tsx`

- [ ] **Step 1: Write the failing test**

Append to `Dashboard.test.tsx`:
```tsx
describe('Dashboard add widget', () => {
  it('re-adds a hidden widget via the Add menu', async () => {
    const user = userEvent.setup();
    const { container } = renderDashboard();
    await user.click(screen.getByRole('button', { name: /^edit$/i }));
    await user.click(screen.getByLabelText(/hide net-worth-kpi/i));
    expect(container.querySelectorAll('.react-grid-item').length).toBe(7);
    await user.click(screen.getByRole('button', { name: /^add widget$/i }));
    await user.click(screen.getByRole('menuitem', { name: /^net worth$/i }));
    expect(container.querySelectorAll('.react-grid-item').length).toBe(8);
  });
});
```

- [ ] **Step 2: Run the test to verify it fails**

Run from `spa/`:
```bash
npx vitest run src/components/dashboard/Dashboard.test.tsx -t "add"
```
Expected: FAIL — "Add widget" button not found.

- [ ] **Step 3: Implement AddWidgetMenu (uses existing `@radix-ui/react-dropdown-menu`)**

```tsx
// spa/src/components/dashboard/AddWidgetMenu.tsx
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuTrigger,
} from '@radix-ui/react-dropdown-menu';
import { Plus } from 'lucide-react';
import { WIDGETS } from '../../lib/dashboard/registry';
import type { WidgetId } from '../../lib/dashboard/types';

interface Props {
  hidden: WidgetId[];
  onAdd: (id: WidgetId) => void;
}

export function AddWidgetMenu({ hidden, onAdd }: Props) {
  if (hidden.length === 0) {
    return (
      <button
        type="button"
        disabled
        className="flex items-center gap-1 rounded-md border px-3 py-1.5 text-sm font-medium opacity-50"
      >
        <Plus className="h-4 w-4" /> Add widget
      </button>
    );
  }
  return (
    <DropdownMenu>
      <DropdownMenuTrigger asChild>
        <button
          type="button"
          className="flex items-center gap-1 rounded-md border px-3 py-1.5 text-sm font-medium hover:bg-muted"
        >
          <Plus className="h-4 w-4" /> Add widget
        </button>
      </DropdownMenuTrigger>
      <DropdownMenuContent
        align="end"
        className="z-50 min-w-[12rem] rounded-md border bg-popover p-1 shadow-md"
      >
        {hidden.map((id) => (
          <DropdownMenuItem
            key={id}
            onSelect={() => onAdd(id)}
            className="cursor-pointer rounded px-2 py-1.5 text-sm outline-none hover:bg-muted"
          >
            {WIDGETS[id].title}
          </DropdownMenuItem>
        ))}
      </DropdownMenuContent>
    </DropdownMenu>
  );
}
```

- [ ] **Step 4: Update EditModeHeader to host AddWidgetMenu and Reset**

```tsx
// spa/src/components/dashboard/EditModeHeader.tsx
import type { WidgetId } from '../../lib/dashboard/types';
import { AddWidgetMenu } from './AddWidgetMenu';

interface Props {
  editing: boolean;
  onToggle: () => void;
  hidden: WidgetId[];
  onAdd: (id: WidgetId) => void;
  onReset: () => void;
}

export function EditModeHeader({ editing, onToggle, hidden, onAdd, onReset }: Props) {
  return (
    <div className="mb-4 flex items-center justify-between">
      <h1 className="text-2xl font-semibold">Dashboard</h1>
      <div className="flex items-center gap-2">
        {editing && <AddWidgetMenu hidden={hidden} onAdd={onAdd} />}
        {editing && (
          <button
            type="button"
            onClick={() => {
              if (confirm('Reset dashboard to defaults?')) onReset();
            }}
            className="rounded-md border px-3 py-1.5 text-sm font-medium hover:bg-muted"
          >
            Reset
          </button>
        )}
        <button
          type="button"
          onClick={onToggle}
          className="rounded-md border px-3 py-1.5 text-sm font-medium hover:bg-muted"
        >
          {editing ? 'Done' : 'Edit'}
        </button>
      </div>
    </div>
  );
}
```

- [ ] **Step 5: Wire add/reset into Dashboard.tsx**

In `Dashboard.tsx`, add these handlers above the return:
```tsx
function addWidget(id: WidgetId) {
  setState((s) => {
    const next = { ...s, hidden: s.hidden.filter((h) => h !== id) };
    // If the widget has no layout entry (edge case), append it.
    if (!next.layout.find((g) => g.i === id)) {
      const def = DEFAULT_STATE.layout.find((g) => g.i === id);
      const maxY = next.layout.reduce((m, g) => Math.max(m, g.y + g.h), 0);
      next.layout = [
        ...next.layout,
        { i: id, x: def?.x ?? 0, y: maxY, w: def?.w ?? 1, h: def?.h ?? 1 },
      ];
    }
    return next;
  });
}

function resetToDefaults() {
  setState(DEFAULT_STATE);
}
```

Then pass them to the header:
```tsx
<EditModeHeader
  editing={editing}
  onToggle={() => setEditing((e) => !e)}
  hidden={state.hidden}
  onAdd={addWidget}
  onReset={resetToDefaults}
/>
```

- [ ] **Step 6: Resolve unused-import warning**

Run from `spa/`:
```bash
npm run check
```
Fix any biome warnings inline.

- [ ] **Step 7: Run tests**

Run from `spa/`:
```bash
npx vitest run src/components/dashboard/
```
Expected: all PASS.

- [ ] **Step 8: Commit**

```bash
git add spa/src/components/dashboard/AddWidgetMenu.tsx \
        spa/src/components/dashboard/EditModeHeader.tsx \
        spa/src/components/dashboard/Dashboard.tsx \
        spa/src/components/dashboard/Dashboard.test.tsx
git commit -m "feat(spa): add 'Add widget' menu and reset-to-defaults in dashboard edit mode"
```

---

### Task 2.4: Mobile breakpoint (edit mode disabled, single column)

**Files:**
- Modify: `spa/src/components/dashboard/Dashboard.tsx`

- [ ] **Step 1: Add a `useMediaQuery` hook**

```ts
// spa/src/lib/hooks/useMediaQuery.ts
import { useEffect, useState } from 'react';

export function useMediaQuery(query: string): boolean {
  const [matches, setMatches] = useState(() =>
    typeof window === 'undefined' ? false : window.matchMedia(query).matches,
  );
  useEffect(() => {
    const m = window.matchMedia(query);
    const handler = () => setMatches(m.matches);
    m.addEventListener('change', handler);
    return () => m.removeEventListener('change', handler);
  }, [query]);
  return matches;
}
```

- [ ] **Step 2: Use it in Dashboard.tsx**

Add inside `Dashboard`:
```tsx
const isMobile = useMediaQuery('(max-width: 640px)');
const cols = isMobile ? 1 : 3;
// Force editing off on mobile
useEffect(() => { if (isMobile) setEditing(false); }, [isMobile]);
```

And replace `cols={COLS}` with `cols={cols}`.

In the header, hide the Edit button on mobile:
```tsx
{!isMobile && (
  <button ...>{editing ? 'Done' : 'Edit'}</button>
)}
```

- [ ] **Step 3: Verify TS compiles and existing tests pass**

Run from `spa/`:
```bash
npx tsc --noEmit && npx vitest run src/components/dashboard/
```
Expected: no TS errors, all tests PASS.

- [ ] **Step 4: Commit**

```bash
git add spa/src/lib/hooks/useMediaQuery.ts spa/src/components/dashboard/Dashboard.tsx
git commit -m "feat(spa): collapse dashboard to single column and disable edit on mobile"
```

---

### Task 2.5: `/dashboard` route

**Files:**
- Create: `spa/src/routes/dashboard.tsx`

- [ ] **Step 1: Create the route**

```tsx
// spa/src/routes/dashboard.tsx
import { createFileRoute } from '@tanstack/react-router';
import { Dashboard } from '../components/dashboard/Dashboard';

export const Route = createFileRoute('/dashboard')({
  component: Dashboard,
});
```

- [ ] **Step 2: Regenerate the route tree**

The route tree is auto-generated by `@tanstack/router-vite-plugin` on dev/build. Trigger it:
```bash
npm run build
```
Expected: `spa/src/routeTree.gen.ts` now includes `/dashboard`. Build passes.

- [ ] **Step 3: Add Dashboard to the sidebar**

Edit `spa/src/components/Sidebar.tsx`. The existing `NAV` array is a plain list of `{ label, to, prefix? }`. Prepend a Dashboard entry:

```tsx
const NAV: NavItem[] = [
  { label: 'Dashboard', to: '/dashboard' },
  { label: 'Balances', to: '/balances' },
  { label: 'Accounts', to: '/accounts' },
  { label: 'Transactions', to: '/transactions' },
  { label: 'Reports', to: '/reports', prefix: true },
  { label: 'Reconcile' },
];
```

- [ ] **Step 4: Commit**

```bash
git add spa/src/routes/dashboard.tsx spa/src/routeTree.gen.ts spa/src/components/Sidebar.tsx
git commit -m "feat(spa): add /dashboard route and sidebar entry"
```

---

### Task 2.6: Redirect `/` to `/dashboard`

**Files:**
- Modify: `spa/src/routes/index.tsx`

- [ ] **Step 1: Update the index redirect**

```tsx
// spa/src/routes/index.tsx
import { createFileRoute, redirect } from '@tanstack/react-router';

export const Route = createFileRoute('/')({
  beforeLoad: () => {
    throw redirect({ to: '/dashboard' });
  },
});
```

- [ ] **Step 2: Verify build and tests**

Run from `spa/`:
```bash
npm run build && npx vitest run
```
Expected: build OK, all tests PASS.

- [ ] **Step 3: Commit**

```bash
git add spa/src/routes/index.tsx
git commit -m "feat(spa): redirect / to /dashboard"
```

---

## Phase 3 — Widgets (one per task)

For each widget below, the pattern is:
1. Write a TanStack Query hook (or reuse an existing one) and an API call (or reuse one).
2. Build the widget component with three explicit states: **loading** (skeleton), **error** (small retry tile), **empty** (placeholder message).
3. Register it in `WIDGETS` to replace the placeholder.
4. Verify the dashboard now renders the real widget.

### Task 3.1: NetWorthKpi

**Files:**
- Create: `spa/src/components/dashboard/widgets/NetWorthKpi.tsx`
- Modify: `spa/src/lib/api.ts` (add `getNetWorth`)
- Modify: `spa/src/lib/dashboard/registry.ts`

Period delta: Net Worth at `now` vs Net Worth at end of last month.

- [ ] **Step 1: Add `getNetWorth(at?: number)` to api.ts**

```ts
// Append to spa/src/lib/api.ts
export interface NetWorthResponse {
  at: number;
  net_worth: Record<string, number>; // currency -> cents
}

export function getNetWorth(at?: number): Promise<NetWorthResponse> {
  const q = at !== undefined ? `?at=${at}` : '';
  return apiFetch<NetWorthResponse>(`/api/reports/net-worth${q}`);
}
```

- [ ] **Step 2: Implement the widget**

```tsx
// spa/src/components/dashboard/widgets/NetWorthKpi.tsx
import { useQuery } from '@tanstack/react-query';
import { getNetWorth } from '../../../lib/api';
import { useAmountFormat, useServerConfig } from '../../../lib/server-config';

function endOfPrevMonthUnix(): number {
  const d = new Date();
  d.setUTCDate(1);
  d.setUTCHours(0, 0, 0, 0);
  return Math.floor(d.getTime() / 1000) - 1;
}

export function NetWorthKpi() {
  const currency = useServerConfig().defaults.currency;
  const { formatCents } = useAmountFormat();
  const cur = useQuery({
    queryKey: ['dashboard', 'net-worth', 'now'],
    queryFn: () => getNetWorth(),
    staleTime: 5 * 60_000,
  });
  const prev = useQuery({
    queryKey: ['dashboard', 'net-worth', 'prev-month-end'],
    queryFn: () => getNetWorth(endOfPrevMonthUnix()),
    staleTime: 5 * 60_000,
  });

  if (cur.isLoading || prev.isLoading) {
    return <Skeleton />;
  }
  if (cur.isError) return <ErrorTile onRetry={() => cur.refetch()} />;

  const now = cur.data?.net_worth[currency] ?? 0;
  const before = prev.data?.net_worth[currency] ?? 0;
  const delta = now - before;
  const pct = before === 0 ? null : (delta / Math.abs(before)) * 100;

  return (
    <div className="flex h-full flex-col justify-center">
      <div className="text-xs text-muted-foreground">Net Worth</div>
      <div className="text-2xl font-semibold tabular-nums">
        {formatCents(now, currency)}
      </div>
      <div
        className={`text-xs ${delta >= 0 ? 'text-emerald-600' : 'text-rose-600'}`}
      >
        {delta >= 0 ? '▲' : '▼'} {formatCents(Math.abs(delta), currency)}
        {pct !== null && ` (${pct.toFixed(1)}%)`} vs last month
      </div>
    </div>
  );
}

function Skeleton() {
  return (
    <div className="flex h-full flex-col justify-center gap-2">
      <div className="h-3 w-16 animate-pulse rounded bg-muted" />
      <div className="h-7 w-32 animate-pulse rounded bg-muted" />
      <div className="h-3 w-24 animate-pulse rounded bg-muted" />
    </div>
  );
}

function ErrorTile({ onRetry }: { onRetry: () => void }) {
  return (
    <div className="flex h-full flex-col items-center justify-center gap-1 text-xs">
      <span className="text-muted-foreground">Failed to load</span>
      <button
        type="button"
        onClick={onRetry}
        className="rounded border px-2 py-0.5 hover:bg-muted"
      >
        Retry
      </button>
    </div>
  );
}
```

> **Note:** All widgets in Phase 3 use `useAmountFormat()` (returns a `formatCents` bound to the user's `hide_decimals` setting) and `useServerConfig().defaults.currency` for the default currency. The exact names are listed in the "Key existing helpers" block at the top of this plan.

- [ ] **Step 3: Register the widget**

In `spa/src/lib/dashboard/registry.ts`, replace the `'net-worth-kpi'` placeholder:
```tsx
import { NetWorthKpi } from '../../components/dashboard/widgets/NetWorthKpi';
// ...
'net-worth-kpi': {
  id: 'net-worth-kpi',
  title: 'Net Worth',
  defaultConfig: {},
  component: NetWorthKpi,
},
```

- [ ] **Step 4: Run tests + check**

Run from `spa/`:
```bash
npm run check && npx vitest run
```
Expected: all PASS. The dashboard tests assert grid-item *count*, not widget text, so registering a real widget does not break them. (If you wrote any text assertions for placeholder text, swap them for grid-item count assertions.)

- [ ] **Step 5: Commit**

```bash
git add spa/src/components/dashboard/widgets/NetWorthKpi.tsx \
        spa/src/lib/api.ts spa/src/lib/dashboard/registry.ts \
        spa/src/components/dashboard/Dashboard.test.tsx
git commit -m "feat(spa): add NetWorthKpi dashboard widget"
```

---

### Task 3.2: NetWorthTrend (wraps existing NetWorthChart)

**Files:**
- Create: `spa/src/components/dashboard/widgets/NetWorthTrend.tsx`
- Modify: `spa/src/lib/dashboard/registry.ts`

Reuses `spa/src/components/reports/NetWorthChart.tsx`. Per-widget config: `{ range: '6m' | '12m' | '24m' | 'all' }`.

- [ ] **Step 1: Inspect NetWorthChart props**

Open `spa/src/components/reports/NetWorthChart.tsx` and confirm its prop shape. Adjust the wrapper below to match. Common shape: takes `series: CurrencyDailySeries[]` plus a `range` filter.

- [ ] **Step 2: Implement the widget**

```tsx
// spa/src/components/dashboard/widgets/NetWorthTrend.tsx
import { useQuery } from '@tanstack/react-query';
import { fetchNetWorthSeries } from '../../../lib/api/reports';
import { NetWorthChart } from '../../reports/NetWorthChart';

export type NetWorthTrendConfig = { range: '6m' | '12m' | '24m' | 'all' };

export const NET_WORTH_TREND_DEFAULT: NetWorthTrendConfig = { range: '12m' };

const RANGE_TO_DAYS: Record<NetWorthTrendConfig['range'], number | null> = {
  '6m': 183,
  '12m': 365,
  '24m': 730,
  all: null,
};

export function NetWorthTrend({ config }: { config: NetWorthTrendConfig }) {
  const q = useQuery({
    queryKey: ['dashboard', 'net-worth-series'],
    queryFn: fetchNetWorthSeries,
    staleTime: 5 * 60_000,
  });
  if (q.isLoading) return <div className="h-full animate-pulse rounded bg-muted" />;
  if (q.isError) return <div className="p-2 text-xs text-muted-foreground">Failed to load</div>;

  const days = RANGE_TO_DAYS[config.range];
  const series = (q.data?.items ?? []).map((s) => ({
    ...s,
    points: days === null ? s.points : s.points.slice(-days),
  }));

  return (
    <div className="flex h-full flex-col">
      <div className="mb-1 text-xs font-medium text-muted-foreground">Net Worth Trend</div>
      <div className="flex-1">
        <NetWorthChart series={series} />
      </div>
    </div>
  );
}
```

- [ ] **Step 3: Add ConfigForm**

```tsx
// Append to the same file
export function NetWorthTrendConfigForm({
  config,
  onChange,
}: {
  config: NetWorthTrendConfig;
  onChange: (c: NetWorthTrendConfig) => void;
}) {
  return (
    <div className="flex flex-col gap-1 text-sm">
      <label className="text-xs font-medium">Range</label>
      <select
        value={config.range}
        onChange={(e) => onChange({ range: e.target.value as NetWorthTrendConfig['range'] })}
        className="rounded border px-2 py-1 text-sm"
      >
        <option value="6m">6 months</option>
        <option value="12m">12 months</option>
        <option value="24m">24 months</option>
        <option value="all">All</option>
      </select>
    </div>
  );
}
```

- [ ] **Step 4: Register in registry.ts**

```tsx
import { NetWorthTrend, NET_WORTH_TREND_DEFAULT, NetWorthTrendConfigForm } from '../../components/dashboard/widgets/NetWorthTrend';

'net-worth-trend': {
  id: 'net-worth-trend',
  title: 'Net Worth Trend',
  defaultConfig: NET_WORTH_TREND_DEFAULT,
  component: NetWorthTrend,
  ConfigForm: NetWorthTrendConfigForm,
},
```

- [ ] **Step 5: Run check + tests**

```bash
npm run check && npx vitest run
```
Expected: PASS.

- [ ] **Step 6: Commit**

```bash
git add spa/src/components/dashboard/widgets/NetWorthTrend.tsx \
        spa/src/lib/dashboard/registry.ts
git commit -m "feat(spa): add NetWorthTrend dashboard widget"
```

---

### Task 3.3: CashFlowKpi

**Files:**
- Create: `spa/src/components/dashboard/widgets/CashFlowKpi.tsx`
- Modify: `spa/src/lib/dashboard/registry.ts`

Uses existing `useIncomeStatement` hook. Period: `month` (current YYYY-MM) for now, plus a query for last month for delta.

- [ ] **Step 1: Implement the widget**

`ReportResult` has `total_income: Record<currency, cents>` and `total_expense: Record<currency, cents>` per-currency maps (see types.ts).

```tsx
// spa/src/components/dashboard/widgets/CashFlowKpi.tsx
import { useIncomeStatement } from '../../../lib/hooks/useReport';
import { useAmountFormat, useServerConfig } from '../../../lib/server-config';
import type { ReportResult } from '../../../lib/types';

function thisMonthStr(): string {
  const d = new Date();
  return `${d.getUTCFullYear()}-${String(d.getUTCMonth() + 1).padStart(2, '0')}`;
}
function lastMonthStr(): string {
  const d = new Date();
  d.setUTCDate(1);
  d.setUTCMonth(d.getUTCMonth() - 1);
  return `${d.getUTCFullYear()}-${String(d.getUTCMonth() + 1).padStart(2, '0')}`;
}

function totals(report: ReportResult | undefined, currency: string) {
  return {
    income: report?.total_income[currency] ?? 0,
    expense: report?.total_expense[currency] ?? 0,
  };
}

export function CashFlowKpi() {
  const currency = useServerConfig().defaults.currency;
  const { formatCents } = useAmountFormat();
  const cur = useIncomeStatement({ month: thisMonthStr() });
  const prev = useIncomeStatement({ month: lastMonthStr() });

  if (cur.isLoading || prev.isLoading)
    return <div className="h-full animate-pulse rounded bg-muted" />;
  if (cur.isError)
    return <div className="p-2 text-xs text-muted-foreground">Failed to load</div>;

  const a = totals(cur.data, currency);
  const b = totals(prev.data, currency);
  const net = a.income - a.expense;
  const prevNet = b.income - b.expense;
  const netDelta = net - prevNet;

  return (
    <div className="flex h-full flex-col justify-center">
      <div className="text-xs text-muted-foreground">This Month</div>
      <div className="mt-1 grid grid-cols-3 gap-2 text-xs">
        <Cell label="Income" value={formatCents(a.income, currency)} tone="pos" />
        <Cell label="Expense" value={formatCents(a.expense, currency)} tone="neg" />
        <Cell
          label="Net"
          value={formatCents(net, currency)}
          tone={net >= 0 ? 'pos' : 'neg'}
        />
      </div>
      <div className={`mt-1 text-xs ${netDelta >= 0 ? 'text-emerald-600' : 'text-rose-600'}`}>
        {netDelta >= 0 ? '▲' : '▼'} {formatCents(Math.abs(netDelta), currency)} vs last month
      </div>
    </div>
  );
}

function Cell({
  label,
  value,
  tone,
}: { label: string; value: string; tone: 'pos' | 'neg' }) {
  return (
    <div>
      <div className="text-muted-foreground">{label}</div>
      <div className={`tabular-nums ${tone === 'pos' ? 'text-emerald-600' : 'text-rose-600'}`}>
        {value}
      </div>
    </div>
  );
}
```

- [ ] **Step 2: Register**

```tsx
import { CashFlowKpi } from '../../components/dashboard/widgets/CashFlowKpi';
'cash-flow-kpi': {
  id: 'cash-flow-kpi',
  title: 'This Month',
  defaultConfig: {},
  component: CashFlowKpi,
},
```

- [ ] **Step 3: Run tests + verify visually**

```bash
npm run check && npx vitest run
```

- [ ] **Step 4: Commit**

```bash
git add spa/src/components/dashboard/widgets/CashFlowKpi.tsx \
        spa/src/lib/dashboard/registry.ts
git commit -m "feat(spa): add CashFlowKpi dashboard widget"
```

---

### Task 3.4: PerCurrencyTiles

**Files:**
- Create: `spa/src/components/dashboard/widgets/PerCurrencyTiles.tsx`
- Modify: `spa/src/lib/dashboard/registry.ts`

Net worth per currency. Reuses `getNetWorth()` from Task 3.1.

- [ ] **Step 1: Implement the widget**

```tsx
// spa/src/components/dashboard/widgets/PerCurrencyTiles.tsx
import { useQuery } from '@tanstack/react-query';
import { getNetWorth } from '../../../lib/api';
import { useAmountFormat } from '../../../lib/server-config';

export function PerCurrencyTiles() {
  const { formatCents } = useAmountFormat();
  const q = useQuery({
    queryKey: ['dashboard', 'net-worth', 'now'],
    queryFn: () => getNetWorth(),
    staleTime: 5 * 60_000,
  });
  if (q.isLoading) return <div className="h-full animate-pulse rounded bg-muted" />;
  if (q.isError) return <div className="p-2 text-xs text-muted-foreground">Failed to load</div>;
  const entries = Object.entries(q.data?.net_worth ?? {});
  if (entries.length === 0) {
    return (
      <div className="flex h-full items-center justify-center text-xs text-muted-foreground">
        No balances yet
      </div>
    );
  }
  return (
    <div className="flex h-full flex-col">
      <div className="mb-1 text-xs text-muted-foreground">Per-Currency Net Worth</div>
      <ul className="flex flex-col gap-1 overflow-auto">
        {entries.map(([ccy, amount]) => (
          <li key={ccy} className="flex items-baseline justify-between text-sm">
            <span className="font-medium text-muted-foreground">{ccy}</span>
            <span className="tabular-nums">{formatCents(amount, ccy)}</span>
          </li>
        ))}
      </ul>
    </div>
  );
}
```

- [ ] **Step 2: Register**

```tsx
import { PerCurrencyTiles } from '../../components/dashboard/widgets/PerCurrencyTiles';
'per-currency-tiles': {
  id: 'per-currency-tiles',
  title: 'Per-Currency',
  defaultConfig: {},
  component: PerCurrencyTiles,
},
```

- [ ] **Step 3: Run tests + commit**

```bash
npm run check && npx vitest run
git add spa/src/components/dashboard/widgets/PerCurrencyTiles.tsx \
        spa/src/lib/dashboard/registry.ts
git commit -m "feat(spa): add PerCurrencyTiles dashboard widget"
```

---

### Task 3.5: TopExpenseCategories

**Files:**
- Create: `spa/src/components/dashboard/widgets/TopExpenseCategories.tsx`
- Modify: `spa/src/lib/dashboard/registry.ts`

Wraps `useExpenseBreakdown({ month })`. Show top N categories as horizontal bars (reuse `ProportionBar` if shape fits, otherwise just bars + amounts).

- [ ] **Step 1: Implement the widget**

`ReportResult.expense_rows` is `ReportRow[]` with `{ account_name, amount, currency, ... }` per row (amounts are already-aggregated line totals — one row per expense category). Filter by `currency === userDefaultCurrency`, sort by amount desc, take the top N.

```tsx
// spa/src/components/dashboard/widgets/TopExpenseCategories.tsx
import { useExpenseBreakdown } from '../../../lib/hooks/useReport';
import { useAmountFormat, useServerConfig } from '../../../lib/server-config';
import type { ReportResult } from '../../../lib/types';

export type TopExpenseConfig = { limit: 5 | 10 };
export const TOP_EXPENSE_DEFAULT: TopExpenseConfig = { limit: 5 };

function thisMonthStr(): string {
  const d = new Date();
  return `${d.getUTCFullYear()}-${String(d.getUTCMonth() + 1).padStart(2, '0')}`;
}

function topCategories(report: ReportResult | undefined, currency: string, limit: number) {
  return (report?.expense_rows ?? [])
    .filter((r) => r.currency === currency && r.amount > 0)
    .map((r) => ({ name: r.account_name, amount: r.amount }))
    .sort((a, b) => b.amount - a.amount)
    .slice(0, limit);
}

export function TopExpenseCategories({ config }: { config: TopExpenseConfig }) {
  const currency = useServerConfig().defaults.currency;
  const { formatCents } = useAmountFormat();
  const q = useExpenseBreakdown({ month: thisMonthStr() });
  if (q.isLoading) return <div className="h-full animate-pulse rounded bg-muted" />;
  if (q.isError) return <div className="p-2 text-xs text-muted-foreground">Failed to load</div>;
  const items = topCategories(q.data, currency, config.limit);
  if (items.length === 0) {
    return (
      <div className="flex h-full items-center justify-center text-xs text-muted-foreground">
        No expenses this month
      </div>
    );
  }
  const max = items[0].amount;
  return (
    <div className="flex h-full flex-col">
      <div className="mb-2 text-xs text-muted-foreground">Top Expenses This Month</div>
      <ul className="flex flex-col gap-1.5 overflow-auto">
        {items.map((it) => (
          <li key={it.name} className="text-xs">
            <div className="flex justify-between">
              <span className="truncate">{it.name}</span>
              <span className="tabular-nums">{formatCents(it.amount, currency)}</span>
            </div>
            <div className="mt-0.5 h-1.5 rounded bg-muted">
              <div
                className="h-full rounded bg-primary"
                style={{ width: `${(it.amount / max) * 100}%` }}
              />
            </div>
          </li>
        ))}
      </ul>
    </div>
  );
}

export function TopExpenseConfigForm({
  config,
  onChange,
}: { config: TopExpenseConfig; onChange: (c: TopExpenseConfig) => void }) {
  return (
    <label className="flex flex-col gap-1 text-sm">
      <span className="text-xs font-medium">Number of categories</span>
      <select
        value={config.limit}
        onChange={(e) =>
          onChange({ limit: Number(e.target.value) as TopExpenseConfig['limit'] })
        }
        className="rounded border px-2 py-1 text-sm"
      >
        <option value={5}>5</option>
        <option value={10}>10</option>
      </select>
    </label>
  );
}
```

- [ ] **Step 2: Register**

```tsx
import {
  TopExpenseCategories,
  TopExpenseConfigForm,
  TOP_EXPENSE_DEFAULT,
} from '../../components/dashboard/widgets/TopExpenseCategories';
'top-expense-categories': {
  id: 'top-expense-categories',
  title: 'Top Expense Categories',
  defaultConfig: TOP_EXPENSE_DEFAULT,
  component: TopExpenseCategories,
  ConfigForm: TopExpenseConfigForm,
},
```

- [ ] **Step 3: Run tests + commit**

```bash
npm run check && npx vitest run
git add spa/src/components/dashboard/widgets/TopExpenseCategories.tsx \
        spa/src/lib/dashboard/registry.ts
git commit -m "feat(spa): add TopExpenseCategories dashboard widget"
```

---

### Task 3.6: RecentTransactions

**Files:**
- Create: `spa/src/components/dashboard/widgets/RecentTransactions.tsx`
- Modify: `spa/src/lib/dashboard/registry.ts`

`listTransactions(filter, opts)` already exists in `spa/src/lib/transactions.ts` — reuse it.

- [ ] **Step 1: Implement the widget**

```tsx
// spa/src/components/dashboard/widgets/RecentTransactions.tsx
import { Link } from '@tanstack/react-router';
import { useQuery } from '@tanstack/react-query';
import { listTransactions } from '../../../lib/transactions';
import { useAmountFormat, useServerConfig } from '../../../lib/server-config';

export type RecentTxnConfig = { limit: 5 | 10 | 20 };
export const RECENT_TXN_DEFAULT: RecentTxnConfig = { limit: 10 };

export function RecentTransactions({ config }: { config: RecentTxnConfig }) {
  const currency = useServerConfig().defaults.currency;
  const { formatCents } = useAmountFormat();
  const q = useQuery({
    queryKey: ['dashboard', 'recent-transactions', config.limit],
    queryFn: () => listTransactions({}, { limit: config.limit, include_count: false }),
    staleTime: 30_000,
  });
  if (q.isLoading) return <div className="h-full animate-pulse rounded bg-muted" />;
  if (q.isError) return <div className="p-2 text-xs text-muted-foreground">Failed to load</div>;
  const items = q.data?.items ?? [];
  if (items.length === 0) {
    return (
      <div className="flex h-full items-center justify-center text-xs text-muted-foreground">
        No transactions yet
      </div>
    );
  }
  return (
    <div className="flex h-full flex-col">
      <div className="mb-1 text-xs text-muted-foreground">Recent Transactions</div>
      <ul className="flex flex-col gap-1 overflow-auto">
        {items.map((t) => {
          const expenseAmt = t.splits
            .filter((s) => s.account_type === 'E')
            .reduce((sum, s) => sum + s.amount, 0);
          return (
            <li key={t.id} className="text-xs">
              <Link
                to="/transactions/$id"
                params={{ id: String(t.id) }}
                className="flex items-baseline justify-between hover:underline"
              >
                <span className="truncate">{t.description || '(no description)'}</span>
                <span className="tabular-nums text-muted-foreground">
                  {formatCents(expenseAmt, currency)}
                </span>
              </Link>
            </li>
          );
        })}
      </ul>
    </div>
  );
}

export function RecentTxnConfigForm({
  config,
  onChange,
}: { config: RecentTxnConfig; onChange: (c: RecentTxnConfig) => void }) {
  return (
    <label className="flex flex-col gap-1 text-sm">
      <span className="text-xs font-medium">Number of transactions</span>
      <select
        value={config.limit}
        onChange={(e) => onChange({ limit: Number(e.target.value) as RecentTxnConfig['limit'] })}
        className="rounded border px-2 py-1 text-sm"
      >
        <option value={5}>5</option>
        <option value={10}>10</option>
        <option value={20}>20</option>
      </select>
    </label>
  );
}
```

> **Note:** The amount displayed is the sum of expense-type splits. If the transaction has no expense splits (e.g., a transfer), this shows 0. That's intentional for the dashboard — the user clicks through for detail. Adjust if the existing `transactions.index.tsx` displays a different "headline amount" — match that pattern.

- [ ] **Step 2: Register**

```tsx
import {
  RecentTransactions,
  RecentTxnConfigForm,
  RECENT_TXN_DEFAULT,
} from '../../components/dashboard/widgets/RecentTransactions';
'recent-transactions': {
  id: 'recent-transactions',
  title: 'Recent Transactions',
  defaultConfig: RECENT_TXN_DEFAULT,
  component: RecentTransactions,
  ConfigForm: RecentTxnConfigForm,
},
```

- [ ] **Step 3: Test + commit**

```bash
npm run check && npx vitest run
git add spa/src/components/dashboard/widgets/RecentTransactions.tsx \
        spa/src/lib/dashboard/registry.ts
git commit -m "feat(spa): add RecentTransactions dashboard widget"
```

---

### Task 3.7: BiggestExpenses

**Files:**
- Create: `spa/src/components/dashboard/widgets/BiggestExpenses.tsx`
- Modify: `spa/src/lib/dashboard/registry.ts`

Server has no `order=amount_desc` for transactions. Approach: fetch all `Expense` type transactions for this month and sort client-side by total expense-split amount.

- [ ] **Step 1: Implement the widget**

```tsx
// spa/src/components/dashboard/widgets/BiggestExpenses.tsx
import { Link } from '@tanstack/react-router';
import { useQuery } from '@tanstack/react-query';
import { listTransactions } from '../../../lib/transactions';
import { useAmountFormat, useServerConfig } from '../../../lib/server-config';

export type BiggestExpensesConfig = { limit: 5 | 10 };
export const BIGGEST_EXPENSES_DEFAULT: BiggestExpensesConfig = { limit: 5 };

function startOfMonthUnix(): number {
  const d = new Date();
  d.setUTCDate(1);
  d.setUTCHours(0, 0, 0, 0);
  return Math.floor(d.getTime() / 1000);
}
function endOfMonthUnix(): number {
  const d = new Date();
  d.setUTCMonth(d.getUTCMonth() + 1, 1);
  d.setUTCHours(0, 0, 0, 0);
  return Math.floor(d.getTime() / 1000) - 1;
}

export function BiggestExpenses({ config }: { config: BiggestExpensesConfig }) {
  const currency = useServerConfig().defaults.currency;
  const { formatCents } = useAmountFormat();
  const start = startOfMonthUnix();
  const end = endOfMonthUnix();
  const q = useQuery({
    queryKey: ['dashboard', 'biggest-expenses', start, end],
    queryFn: () =>
      listTransactions(
        { type: 'Expense', start_time: start, end_time: end },
        { limit: 500, include_count: false }, // safety cap; personal finance typically <100/month
      ),
    staleTime: 60_000,
  });
  if (q.isLoading) return <div className="h-full animate-pulse rounded bg-muted" />;
  if (q.isError) return <div className="p-2 text-xs text-muted-foreground">Failed to load</div>;

  const ranked = (q.data?.items ?? [])
    .map((t) => {
      const amount = t.splits
        .filter((s) => s.account_type === 'E' && s.currency === currency)
        .reduce((sum, s) => sum + s.amount, 0);
      return { id: t.id, description: t.description, amount };
    })
    .filter((r) => r.amount > 0)
    .sort((a, b) => b.amount - a.amount)
    .slice(0, config.limit);

  if (ranked.length === 0) {
    return (
      <div className="flex h-full items-center justify-center text-xs text-muted-foreground">
        No expenses this month
      </div>
    );
  }
  return (
    <div className="flex h-full flex-col">
      <div className="mb-1 text-xs text-muted-foreground">Biggest Expenses This Month</div>
      <ul className="flex flex-col gap-1 overflow-auto">
        {ranked.map((r) => (
          <li key={r.id} className="text-xs">
            <Link
              to="/transactions/$id"
              params={{ id: String(r.id) }}
              className="flex items-baseline justify-between hover:underline"
            >
              <span className="truncate">{r.description || '(no description)'}</span>
              <span className="tabular-nums">{formatCents(r.amount, currency)}</span>
            </Link>
          </li>
        ))}
      </ul>
    </div>
  );
}

export function BiggestExpensesConfigForm({
  config,
  onChange,
}: { config: BiggestExpensesConfig; onChange: (c: BiggestExpensesConfig) => void }) {
  return (
    <label className="flex flex-col gap-1 text-sm">
      <span className="text-xs font-medium">Number to show</span>
      <select
        value={config.limit}
        onChange={(e) =>
          onChange({ limit: Number(e.target.value) as BiggestExpensesConfig['limit'] })
        }
        className="rounded border px-2 py-1 text-sm"
      >
        <option value={5}>5</option>
        <option value={10}>10</option>
      </select>
    </label>
  );
}
```

- [ ] **Step 2: Register**

```tsx
import {
  BiggestExpenses,
  BiggestExpensesConfigForm,
  BIGGEST_EXPENSES_DEFAULT,
} from '../../components/dashboard/widgets/BiggestExpenses';
'biggest-expenses': {
  id: 'biggest-expenses',
  title: 'Biggest Expenses',
  defaultConfig: BIGGEST_EXPENSES_DEFAULT,
  component: BiggestExpenses,
  ConfigForm: BiggestExpensesConfigForm,
},
```

- [ ] **Step 3: Test + commit**

```bash
npm run check && npx vitest run
git add spa/src/components/dashboard/widgets/BiggestExpenses.tsx \
        spa/src/lib/dashboard/registry.ts
git commit -m "feat(spa): add BiggestExpenses dashboard widget"
```

---

### Task 3.8: AccountsMoved

**Files:**
- Create: `spa/src/components/dashboard/widgets/AccountsMoved.tsx`
- Modify: `spa/src/lib/dashboard/registry.ts`

Uses `getBalanceHistory()` (already in `lib/api.ts`) which returns `AccountMonthlySeries[]`. Compute Δ between this-month and prior-month for each account, sort by |Δ|.

- [ ] **Step 1: Implement the widget**

```tsx
// spa/src/components/dashboard/widgets/AccountsMoved.tsx
import { useQuery } from '@tanstack/react-query';
import { Link } from '@tanstack/react-router';
import { getBalanceHistory, getBalances } from '../../../lib/api';
import { useAmountFormat } from '../../../lib/server-config';

export type AccountsMovedConfig = { limit: 5 | 10 };
export const ACCOUNTS_MOVED_DEFAULT: AccountsMovedConfig = { limit: 5 };

function thisMonth(): string {
  const d = new Date();
  return `${d.getUTCFullYear()}-${String(d.getUTCMonth() + 1).padStart(2, '0')}`;
}
function lastMonth(): string {
  const d = new Date();
  d.setUTCDate(1);
  d.setUTCMonth(d.getUTCMonth() - 1);
  return `${d.getUTCFullYear()}-${String(d.getUTCMonth() + 1).padStart(2, '0')}`;
}

export function AccountsMoved({ config }: { config: AccountsMovedConfig }) {
  const { formatCents } = useAmountFormat();
  const history = useQuery({
    queryKey: ['dashboard', 'balance-history'],
    queryFn: getBalanceHistory,
    staleTime: 5 * 60_000,
  });
  const balances = useQuery({
    queryKey: ['dashboard', 'balances'],
    queryFn: getBalances,
    staleTime: 5 * 60_000,
  });

  if (history.isLoading || balances.isLoading)
    return <div className="h-full animate-pulse rounded bg-muted" />;
  if (history.isError || balances.isError)
    return <div className="p-2 text-xs text-muted-foreground">Failed to load</div>;

  const nameById = new Map(
    (balances.data?.items ?? []).map((b) => [b.account_id, { name: b.name, currency: b.currency }]),
  );
  const thisM = thisMonth();
  const lastM = lastMonth();
  const ranked = (history.data?.items ?? [])
    .map((s) => {
      const now = s.points.find((p) => p.month === thisM)?.balance ?? 0;
      const before = s.points.find((p) => p.month === lastM)?.balance ?? 0;
      const delta = now - before;
      const meta = nameById.get(s.account_id);
      return {
        id: s.account_id,
        name: meta?.name ?? `#${s.account_id}`,
        currency: meta?.currency ?? s.currency,
        delta,
      };
    })
    .filter((r) => r.delta !== 0)
    .sort((a, b) => Math.abs(b.delta) - Math.abs(a.delta))
    .slice(0, config.limit);

  if (ranked.length === 0) {
    return (
      <div className="flex h-full items-center justify-center text-xs text-muted-foreground">
        No movement this month
      </div>
    );
  }
  return (
    <div className="flex h-full flex-col">
      <div className="mb-1 text-xs text-muted-foreground">Accounts That Moved Most</div>
      <ul className="flex flex-col gap-1 overflow-auto">
        {ranked.map((r) => (
          <li key={r.id} className="text-xs">
            <Link
              to="/accounts/$id"
              params={{ id: String(r.id) }}
              className="flex items-baseline justify-between hover:underline"
            >
              <span className="truncate">{r.name}</span>
              <span
                className={`tabular-nums ${r.delta >= 0 ? 'text-emerald-600' : 'text-rose-600'}`}
              >
                {r.delta >= 0 ? '+' : '−'}
                {formatCents(Math.abs(r.delta), r.currency)}
              </span>
            </Link>
          </li>
        ))}
      </ul>
    </div>
  );
}

export function AccountsMovedConfigForm({
  config,
  onChange,
}: { config: AccountsMovedConfig; onChange: (c: AccountsMovedConfig) => void }) {
  return (
    <label className="flex flex-col gap-1 text-sm">
      <span className="text-xs font-medium">Number to show</span>
      <select
        value={config.limit}
        onChange={(e) =>
          onChange({ limit: Number(e.target.value) as AccountsMovedConfig['limit'] })
        }
        className="rounded border px-2 py-1 text-sm"
      >
        <option value={5}>5</option>
        <option value={10}>10</option>
      </select>
    </label>
  );
}
```

- [ ] **Step 2: Register**

```tsx
import {
  AccountsMoved,
  AccountsMovedConfigForm,
  ACCOUNTS_MOVED_DEFAULT,
} from '../../components/dashboard/widgets/AccountsMoved';
'accounts-moved': {
  id: 'accounts-moved',
  title: 'Accounts That Moved',
  defaultConfig: ACCOUNTS_MOVED_DEFAULT,
  component: AccountsMoved,
  ConfigForm: AccountsMovedConfigForm,
},
```

- [ ] **Step 3: Remove the now-unused Placeholder component**

```bash
git rm spa/src/components/dashboard/widgets/Placeholder.tsx
```

Also remove the `placeholder(...)` helper from `registry.ts` (no more callers).

- [ ] **Step 4: Test + commit**

```bash
npm run check && npx vitest run
git add spa/src/components/dashboard/widgets/AccountsMoved.tsx \
        spa/src/lib/dashboard/registry.ts
git commit -m "feat(spa): add AccountsMoved widget and remove dashboard placeholder"
```

---

## Phase 4 — Polish: per-widget config UI, integration test, manual verify

### Task 4.1: Per-widget config popover (⚙ button)

**Files:**
- Modify: `spa/src/components/dashboard/WidgetFrame.tsx`
- Modify: `spa/src/components/dashboard/Dashboard.tsx`

Each widget with a `ConfigForm` exposes a ⚙ button next to its hide button. Clicking it opens an inline popover.

- [ ] **Step 1: Update WidgetFrame to accept a config-form prop and render the gear**

```tsx
// spa/src/components/dashboard/WidgetFrame.tsx
import { Settings, X } from 'lucide-react';
import {
  Popover,
  PopoverContent,
  PopoverTrigger,
} from '@radix-ui/react-popover';
import type { ReactNode } from 'react';
import type { WidgetMeta } from '../../lib/dashboard/registry';

interface Props {
  meta: WidgetMeta<unknown>;
  editing: boolean;
  config: unknown;
  onConfigChange: (c: unknown) => void;
  onHide: () => void;
  children: ReactNode;
}

export function WidgetFrame({
  meta, editing, config, onConfigChange, onHide, children,
}: Props) {
  if (!editing) return <>{children}</>;
  const ConfigForm = meta.ConfigForm;
  return (
    <div className="flex h-full flex-col">
      <div className="dashboard-drag-handle flex shrink-0 items-center justify-between gap-1 border-b px-2 py-1 text-xs font-medium cursor-move bg-muted/30">
        <span className="truncate">{meta.title}</span>
        <div className="flex items-center gap-1">
          {ConfigForm && (
            <Popover>
              <PopoverTrigger asChild>
                <button
                  type="button"
                  aria-label={`configure ${meta.id}`}
                  onClick={(e) => e.stopPropagation()}
                  onMouseDown={(e) => e.stopPropagation()}
                  className="rounded p-0.5 hover:bg-muted"
                >
                  <Settings className="h-3.5 w-3.5" />
                </button>
              </PopoverTrigger>
              <PopoverContent
                align="end"
                sideOffset={4}
                className="z-50 min-w-[12rem] rounded-md border bg-popover p-3 shadow-md"
                onClick={(e) => e.stopPropagation()}
                onMouseDown={(e) => e.stopPropagation()}
              >
                <ConfigForm config={config} onChange={onConfigChange} />
              </PopoverContent>
            </Popover>
          )}
          <button
            type="button"
            aria-label={`hide ${meta.id}`}
            onClick={(e) => {
              e.stopPropagation();
              onHide();
            }}
            className="rounded p-0.5 hover:bg-muted"
          >
            <X className="h-3.5 w-3.5" />
          </button>
        </div>
      </div>
      <div className="flex-1 overflow-hidden p-2">{children}</div>
    </div>
  );
}
```

Note: `@radix-ui/react-popover` is not in `package.json`. Add it:
```bash
npm install @radix-ui/react-popover
```

- [ ] **Step 2: Wire `onConfigChange` in Dashboard.tsx**

Add a handler:
```tsx
function setConfig(id: WidgetId, cfg: unknown) {
  setState((s) => ({ ...s, config: { ...s.config, [id]: cfg } }));
}
```

Pass `config` and `onConfigChange` into `<WidgetFrame>`:
```tsx
<WidgetFrame
  meta={meta}
  editing={editing}
  config={cfg}
  onConfigChange={(c) => setConfig(item.i, c)}
  onHide={() => hideWidget(item.i)}
>
  <Comp config={cfg} />
</WidgetFrame>
```

- [ ] **Step 3: Add a config persistence test**

Append to `Dashboard.test.tsx`:
```tsx
describe('Dashboard widget config', () => {
  it('saves per-widget config changes to localStorage', async () => {
    const user = userEvent.setup();
    renderDashboard();
    await user.click(screen.getByRole('button', { name: /^edit$/i }));
    await user.click(screen.getByLabelText(/configure recent-transactions/i));
    const select = await screen.findByRole('combobox', { name: /number of transactions/i });
    await user.selectOptions(select, '20');
    // Storage save is debounced 500ms.
    await new Promise((r) => setTimeout(r, 600));
    const raw = localStorage.getItem('kea.dashboard.main');
    expect(raw).toBeTruthy();
    const parsed = JSON.parse(raw as string);
    expect(parsed.config['recent-transactions']).toEqual({ limit: 20 });
  });
});
```

- [ ] **Step 4: Run tests + commit**

```bash
npm run check && npx vitest run
git add spa/src/components/dashboard/WidgetFrame.tsx \
        spa/src/components/dashboard/Dashboard.tsx \
        spa/src/components/dashboard/Dashboard.test.tsx \
        spa/package.json spa/package-lock.json
git commit -m "feat(spa): add per-widget config popover with persistence"
```

---

### Task 4.2: Manual verification in browser

This task uses the `preview_*` tools (or the user's local dev server) to confirm the dashboard works end-to-end.

- [ ] **Step 1: Start the dev server**

Run from the repo root:
```bash
make run
# OR: cd spa && npm run dev (if SPA is served separately in dev)
```

If the project's `verify` skill is available, prefer that — it has the standardized launch flow.

- [ ] **Step 2: Verify each scenario**

For each of the following, observe the dashboard in the browser:

1. **First load on an empty ledger** — dashboard renders the 8 widgets in the default layout. Widgets with no data show their empty states.
2. **Edit mode** — clicking Edit reveals widget chrome (drag handle, ✕, ⚙). Dragging a widget reorders it. Resizing changes its grid span. Clicking Done exits edit mode; the dashboard reads clean.
3. **Hide + Add** — hide a widget in edit mode, click Done. Reload page → widget still hidden. Re-open Edit → Add Widget → click the widget → it reappears.
4. **Config** — open ⚙ on NetWorthTrend, change range from 12m to 6m. Chart redraws. Reload page → range is still 6m.
5. **Reset** — click Reset in edit mode, confirm. All widgets visible, default order.
6. **Mobile** — resize browser below 640px. Grid collapses to one column. Edit button is hidden.
7. **Ledger switch** — if multiple ledgers exist, switch ledgers using the existing LedgerSwitcher. The dashboard layout is per-ledger; changes to one don't affect the other.

- [ ] **Step 3: Take a screenshot for the PR**

If using `preview_screenshot`, capture default view + edit mode.

- [ ] **Step 4: No commit unless adjustments needed**

If any step fails, fix inline and create a follow-up commit.

---

## Self-Review Checklist (run after writing tasks above)

Run these one-time checks at the end before handing off:

- [ ] **Final TS + tests + biome:**
  ```bash
  cd spa && npx tsc --noEmit && npm run check && npx vitest run
  cd .. && go build ./... && go test ./...
  ```
- [ ] **Spec coverage:** every section/decision in the design spec has a corresponding task above. Specifically:
  - Net-worth-kpi → Task 3.1
  - Net-worth-trend → Task 3.2
  - Cash-flow-kpi → Task 3.3
  - Per-currency-tiles → Task 3.4
  - Top-expense-categories → Task 3.5
  - Recent-transactions → Task 3.6
  - Biggest-expenses → Task 3.7
  - Accounts-moved → Task 3.8
  - Storage + reconciliation → Task 1.2
  - Widget registry → Task 1.3
  - Grid + edit mode → Tasks 2.1, 2.2
  - Add menu + reset → Task 2.3
  - Mobile breakpoint → Task 2.4
  - Routing → Tasks 2.5, 2.6
  - Per-widget config UI → Task 4.1
  - Manual verify → Task 4.2

---

## Out of scope (per spec)

- Cross-device sync (would require new backend endpoint)
- User-defined widgets
- Drill-down click-through from widgets to filtered views
- Per-widget refresh intervals exposed in UI
- Backend changes of any kind
