# WebUI Dashboard — Design

**Date:** 2026-06-28
**Status:** Approved (brainstorming)
**Scope:** Add a customizable dashboard as the new home page of the KEA SPA.

## Goal

Replace the current `/` → `/balances` redirect with a dashboard at `/dashboard` that gives the user, at a glance, two things:

1. **Financial health snapshot** — net worth, cash flow, where money is going.
2. **Recent activity recap** — last transactions, biggest expenses, accounts that moved.

The dashboard is **customizable**: users can show/hide, reorder, resize, and configure individual widgets.

## Non-goals (v1)

- Cross-device sync of dashboard settings.
- User-defined widgets (e.g., "balance of account X" with arbitrary parameters).
- Click-through from widgets to pre-filtered views (deferred polish).
- Per-widget refresh intervals exposed in UI.

## Widgets

Eight widgets ship in v1.

**Health snapshot:**

| ID | Title | Source |
|---|---|---|
| `net-worth-kpi` | Net Worth KPI | `GET /api/reports/net-worth` (current + delta endpoint) |
| `net-worth-trend` | Net Worth trend chart | `GET /api/reports/net-worth-series?range=12m` (wraps existing `NetWorthChart`) |
| `cash-flow-kpi` | This month's Income / Expense / Net | `GET /api/reports/income-statement?period=this-month` (+ last-month for delta) |
| `top-expense-categories` | Top spending categories this month | `GET /api/reports/expense-breakdown?period=this-month` (wraps existing `ProportionBar`) |
| `per-currency-tiles` | Per-currency net worth tiles | `GET /api/reports/net-worth` per-currency breakdown |

**Recent activity:**

| ID | Title | Source |
|---|---|---|
| `recent-transactions` | Last 10 transactions | `GET /api/transactions?limit=10&order=date_desc` |
| `biggest-expenses` | Biggest expenses this month | `GET /api/transactions?period=this-month&kind=expense&order=amount_desc&limit=5` |
| `accounts-moved` | Accounts with biggest balance Δ this month | `GET /api/balances/history?range=this-month`, sort client-side |

**Two assumptions to verify during planning** (not in this spec):

1. `GET /api/transactions` accepts a filter for expense-only transactions. If not, `biggest-expenses` either gets a new server-side filter or fetches more and filters client-side.
2. `GET /api/reports/net-worth` returns per-currency breakdown. If not, `per-currency-tiles` derives from `GET /api/balances`.

## Routing

- `routes/index.tsx`: redirect changes from `/balances` → `/dashboard`.
- New route `routes/dashboard.tsx` renders the dashboard.
- `/balances` and all other existing routes are unchanged.

## Architecture

### Widget registry

A single registry (`spa/src/lib/dashboard/registry.ts`) is the source of truth for which widgets exist, their default config, and their default grid placement:

```ts
type WidgetId =
  | 'net-worth-kpi' | 'net-worth-trend' | 'cash-flow-kpi'
  | 'top-expense-categories' | 'per-currency-tiles'
  | 'recent-transactions' | 'biggest-expenses' | 'accounts-moved';

type WidgetMeta<Config> = {
  id: WidgetId;
  title: string;
  defaultConfig: Config;
  defaultLayout: { x: number; y: number; w: number; h: number };
  component: ComponentType<{ config: Config }>;
  ConfigForm?: ComponentType<{ config: Config; onChange: (c: Config) => void }>;
};

export const WIDGETS: Record<WidgetId, WidgetMeta<any>>;
```

Adding a future widget = one file edit (register it) plus the widget component itself. Reconciliation logic in storage handles users whose saved layout predates the new widget.

### Layout engine

`react-grid-layout` (~14KB gzipped, accessible, includes drag + resize + responsive breakpoints):

- Desktop (≥1024px): 3-column grid.
- Tablet (≥640px, <1024px): 2-column grid.
- Mobile (<640px): single column. Edit mode disabled (tooltip: "Edit dashboard on a larger screen").

Each widget declares its `defaultLayout` (`w`/`h` in grid units). The layout is mutable in edit mode.

### Edit mode

- Header has an **Edit** toggle.
- When on:
  - Each widget renders inside `WidgetFrame`, which adds a title bar (drag handle), ✕ (hide), and ⚙ (open config popover).
  - Header gains **Add widget** (popover listing hidden widgets) and **Reset to defaults** (destructive, confirm).
  - Grid items become draggable and resizable.
- **Done** exits edit mode and flushes pending writes.

When off, no chrome; widgets render bare.

Per-widget config (⚙) opens an inline popover anchored to the widget. The widget's `ConfigForm` renders inside. Changes apply live.

### File layout

```
spa/src/
  routes/
    dashboard.tsx
    index.tsx                      # redirect updated
  components/dashboard/
    Dashboard.tsx                  # grid + edit-mode shell
    WidgetFrame.tsx                # edit-mode chrome (title bar, ✕, ⚙)
    AddWidgetMenu.tsx
    EditModeHeader.tsx
    widgets/
      NetWorthKpi.tsx
      NetWorthTrend.tsx            # wraps NetWorthChart
      CashFlowKpi.tsx
      PerCurrencyTiles.tsx
      TopExpenseCategories.tsx     # wraps ProportionBar
      BiggestExpenses.tsx
      AccountsMoved.tsx
      RecentTransactions.tsx
  lib/dashboard/
    registry.ts
    storage.ts                     # per-ledger localStorage
    types.ts
```

## Data flow

Each widget owns its own data fetching via TanStack Query. There is no shared dashboard-level query.

Rationale:

- Widgets stay independently composable; adding a new widget doesn't require touching a dashboard-wide query orchestrator.
- Each widget can choose its own staleness (KPIs ~5 min, recent transactions ~30s) without coupling to others.
- A failing endpoint affects only its own widget; the dashboard frame remains.

## Storage

**Key:** `kea:dashboard:<ledger-name>` — matches the existing `filter-memory` per-ledger convention.

**Schema:**

```ts
type DashboardState = {
  version: 1;
  layout: Array<{ i: WidgetId; x: number; y: number; w: number; h: number }>;
  hidden: WidgetId[];
  config: Partial<Record<WidgetId, unknown>>;
};
```

**Load:**

1. Read key. If missing or `version` mismatch → use registry defaults.
2. Reconcile against registry:
   - Drop unknown ids from `layout`, `hidden`, and `config` (widget removed in a later release).
   - Append registry widgets not present in saved `layout` or `hidden` (widget added in a later release), using their `defaultLayout`.
3. Merge each widget's saved `config` with its `defaultConfig` so newly added config fields get sensible defaults.

**Save:** debounced 500ms; exiting edit mode flushes immediately.

## Edge cases

- **Empty ledger / no transactions:** each widget renders its own empty state ("No transactions this month yet"). The dashboard frame still renders.
- **All widgets hidden:** dashboard shows an "Add widgets" CTA pointing to the Add menu.
- **Multi-currency KPIs:** values that can't reduce to a single number (e.g., net worth across USD + TWD) follow the existing `NetWorthCard` pattern — primary currency shown big with a "+ N other currencies" hint. `per-currency-tiles` is the dedicated answer for serious multi-currency users.
- **API errors:** per-widget error boundary renders an inline error tile with a retry button. A 5xx never blanks the dashboard.
- **Storage corruption:** invalid JSON or a version newer than known → console warning + fall back to defaults. Never throws.

## Testing

| Layer | Tool | Target |
|---|---|---|
| Unit | vitest | `lib/dashboard/storage.ts` — round-trip, version-mismatch fallback, registry reconciliation (drop unknown, append new), default-config merge |
| Component | vitest + testing-library | Each widget in isolation against a mocked API — empty, populated, error states |
| Integration | vitest | `Dashboard.tsx` — edit-mode toggle, hide/show, drag reorder (via react-grid-layout API), config persistence across reload |

No backend tests. No backend changes (subject to the two assumptions in the Widgets section).

## Dependencies

- Add: `react-grid-layout` (and its CSS) as a runtime dependency in `spa/`.
- No backend `go.mod` changes.

## Decisions log

| Decision | Choice | Why |
|---|---|---|
| Job-to-be-done | Health snapshot + recent activity | User explicitly chose A+C; rejected an action-queue (B) |
| Customization scope | Show/hide + reorder/resize + lightweight per-widget config | Drag/resize is natural in a grid; full custom widgets deferred |
| Customization UX | In-place edit mode on the dashboard | Matches Linear/Notion patterns; no extra navigation hop |
| Persistence | localStorage per-ledger | Reuses existing `filter-memory` pattern; no backend needed; KEA is single-user local-first |
| Navigation | `/` → `/dashboard`; `/balances` stays | Dashboard becomes the landing page; balances retains its deep-dive role |
| Layout engine | `react-grid-layout` | Drag + resize + responsive in one dep; alternative was hand-rolled flexbox + `@dnd-kit` |
| Layout shape | 3-column grid (desktop), KPIs top, varied widget widths | "Most dashboard-y"; gives the trend chart space; selected from layout mockups |
| Data fetching | Per-widget TanStack Query | Independent composability; per-widget staleness; no shared orchestrator |
