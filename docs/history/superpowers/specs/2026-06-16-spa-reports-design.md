# SPA Reports — Design

**Date:** 2026-06-16
**Status:** Spec
**Scope:** Frontend (SPA) only. All required API endpoints and service methods already exist.

## Goal

Surface the existing five report endpoints (Income Statement, Income Breakdown, Expense Breakdown, Balance Sheet, Net Worth) in the SPA under a single `Reports` sidebar entry, matching the conventions of the existing Balances, Accounts, and Transactions pages.

## Non-Goals

- No new backend work. All endpoints under `/api/reports/*` already exist (`internal/api/reports.go`) and are tested. The SPA consumes them as-is.
- No new chart library — proportional bars are hand-rolled SVG, matching the existing `Sparkline.tsx` precedent.
- No multi-currency conversion / FX. Each currency is reported on its own.
- No time-series chart on the Net Worth page. Snapshot only.
- No E2E or visual-regression tests. The project doesn't have these today and this isn't the page to introduce them.

## Routes

Five sub-routes under a shared `/reports` parent, mirroring the `accounts.tsx` / `transactions.tsx` parent-pattern.

```
spa/src/routes/
  reports.tsx                            parent: layout shell + tab nav
  reports.index.tsx                      redirects to /reports/income-statement
  reports.income-statement.tsx           IS — period-based
  reports.income-breakdown.tsx           IB — period-based
  reports.expense-breakdown.tsx          EB — period-based
  reports.balance-sheet.tsx              BS — as-of snapshot
  reports.net-worth.tsx                  NW — at snapshot
```

Sidebar: `spa/src/components/Sidebar.tsx` `Reports` entry gets `to: '/reports'`, dropping its current `Coming soon` disabled state.

### Search params (validated per-route via TanStack Router's `validateSearch`)

**Period-based routes** (IS / IB / EB):

| Param  | Type                                                                      | Notes                                                          |
| ------ | ------------------------------------------------------------------------- | -------------------------------------------------------------- |
| range  | `'this-month' \| 'last-month' \| 'ytd' \| 'last-12mo' \| 'custom'`        | Default: `'this-month'`.                                       |
| month  | `string?` (`YYYY-MM`)                                                     | Only meaningful when `range === 'this-month'` or `'last-month'`; used to override the default-resolved month when deep-linking. |
| from   | `string?` (`YYYY-MM-DD`)                                                  | Required when `range === 'custom'`.                            |
| to     | `string?` (`YYYY-MM-DD`)                                                  | Required when `range === 'custom'`.                            |

**Balance Sheet** (`balance-sheet`): `as_of?: number` (Unix seconds; omit → server uses `now`).

**Net Worth** (`net-worth`): `at?: number` (Unix seconds).

Search-param state does **not** carry over when the user switches tabs. Each tab uses its own defaults. Carrying period across IS/IB/EB is a possible v2; not in scope.

## Data Loading

Use TanStack Query, matching existing pages.

```
spa/src/lib/api/reports.ts                ← new
  fetchIncomeStatement({ month?, from?, to? })  → ReportResult
  fetchIncomeBreakdown(...)                     → ReportResult
  fetchExpenseBreakdown(...)                    → ReportResult
  fetchBalanceSheet({ asOf? })                  → BalanceSheetResult
  fetchNetWorth({ at? })                        → NetWorthResponse

spa/src/lib/types.ts                      ← extend
  Add ReportRow, ReportResult, BalanceSheetResult, NetWorthResponse,
  mirroring the Go JSON shape from internal/model/report.go and
  internal/api/reports.go (netWorthResponse).

spa/src/lib/hooks/useReport.ts            ← new
  useIncomeStatement(params), useIncomeBreakdown(params),
  useExpenseBreakdown(params), useBalanceSheet(params),
  useNetWorth(params).
  Each wraps useQuery with queryKey ['reports', '<type>', params].
```

Period resolution is centralized so the picker, the API client, and the page title all agree:

```
spa/src/lib/period.ts                     ← new
  type ResolvedPeriod =
    | { mode: 'month'; month: string; label: string }            // 'YYYY-MM'
    | { mode: 'range'; from: string; to: string; label: string } // 'YYYY-MM-DD'

  resolvePeriod(search): ResolvedPeriod
  // 'this-month' → { mode:'month', month:'2026-06', label:'June 2026' }
  // 'last-month' → { mode:'month', month:'2026-05', label:'May 2026' }
  // 'ytd'        → { mode:'range', from:'2026-01-01', to:<today>, label:'YTD 2026' }
  // 'last-12mo'  → { mode:'range', from:<today-12mo>, to:<today>, label:'Last 12 months' }
  // 'custom'     → { mode:'range', from, to, label:'Jun 1 – Jun 30, 2026' }
```

The hooks pass `month` *or* `from`+`to` to the backend depending on `mode`. The CLI accepts both shapes already (`DateRangeParams` in `internal/service/report_service.go`).

Multi-currency handling: the page renders KPI cards in the default currency (`config.Currency`, read from the existing `useServerConfig()` hook in `spa/src/lib/server-config.tsx`). Non-default currencies render as a small `<CurrencyFooter>` line below the KPI cards. No conversion.

## Components

```
spa/src/components/reports/
  TabNav.tsx                             5-tab segmented control
  PeriodPicker.tsx                       chip row + custom-range popover
  AsOfPicker.tsx                         single-date picker (BS, NW)
  KpiCard.tsx                            label + amount + optional sub-line; variant: green | red | neutral
  ProportionBar.tsx                      hand-rolled SVG horizontal bar; given rows + total
  ReportRowTable.tsx                     drill-down rows; accepts onRowLink(row) → `<Link>` target
  CurrencyFooter.tsx                     non-default currency totals
```

All components are presentational; they take fully-resolved data as props. Routes own the data fetching.

Amount formatting reuses the existing `utils/format.ts` (`formatAmount`) used by Balances. Error display reuses the existing `<ErrorBanner>` used in Accounts / Transactions.

## Per-Report Composition

Each route mounts the same scaffold (parent provides `<TabNav>` + per-route `<PeriodControls>` slot + `<Outlet />`):

### Income Statement (`reports.income-statement.tsx`)

- 3× `KpiCard`: Income (green), Expense (red), Net (neutral).
- Net card sub-line: `▲ 6.2% net worth` from `NetWorthGrowthPct[defaultCurrency]` (omit if absent).
- 2× `ProportionBar` block: "Income mix" / "Expense mix" — top-N (N=8) rows; `+ k more` link expands to show the rest.
- 2× `ReportRowTable`: full Income / Expense lists. Row click → `<Link to="/transactions" search={{ account, from, to }}>`. `from`/`to` come from `resolvePeriod()` (always absolute dates).
- `<CurrencyFooter>` if `Object.keys(TotalIncome).length > 1`.

### Income Breakdown (`reports.income-breakdown.tsx`)

- 1× `KpiCard`: Total Income.
- 1× `ProportionBar` block — **full** list (the breakdown *is* the ranking; not top-N).
- 1× `ReportRowTable`: full income rows.
- No expense side.

### Expense Breakdown (`reports.expense-breakdown.tsx`)

Mirror of Income Breakdown on the expense side.

### Balance Sheet (`reports.balance-sheet.tsx`)

- 3× `KpiCard`: Total Assets, Total Liabilities, Total Equity.
- Net Worth as a sub-line on the Equity card (`Net worth: $X`). No separate KPI card for net worth here — the sheet's net worth is just `Assets − Liabilities`.
- 1× `ProportionBar` block: "Asset mix" only. Liability and equity mixes add visual noise without insight.
- 3× `ReportRowTable`, stacked: Assets, Liabilities, Equity. Row click → `<Link to="/accounts/{id}">` (no period to scope by). Empty sections are hidden rather than rendered as "no rows".

### Net Worth (`reports.net-worth.tsx`)

- 1 large number: default-currency net worth as of `at`.
- `<CurrencyFooter>` for other currencies.
- `<AsOfPicker>` only — no tables, no charts. This page is the answer to "what's my net worth right now?". Anything more belongs on IS or BS.

### Parent (`reports.tsx`)

- Renders `<TabNav>` + slot for per-route `<PeriodControls>` + `<Outlet />`.
- `<TabNav>` highlights the active route by matching `useRouterState().location.pathname`.

### Index (`reports.index.tsx`)

- `beforeLoad` redirects to `/reports/income-statement`, mirroring `index.tsx` → `/balances`.

## States

**Loading.** Skeleton matching Balances/Accounts: grey blocks for KPI cards, grey lines for tables. No spinners.

**Empty:**

- *No data in period* (IS/IB/EB): `No income or expenses in <period>. Try a wider range.` The period picker stays active so the user can fix it without leaving.
- *No accounts of this type* (BS section): hide the empty section entirely, matching the CLI behavior.
- *Net worth = 0 and no accounts*: `No accounts yet.` with a `<Link to="/accounts/new">`.

**Error.** `<ErrorBanner>` rendered above the report body so the period picker stays usable. TanStack Query default retry behavior; no custom retry logic.

## Edge Cases

- **Drill-down period encoding.** `<ReportRowTable>` always receives absolute `from`/`to` from `resolvePeriod()`, never the `range` enum. The Transactions filter expects absolute timestamps.
- **Balance Sheet row click without a period.** Links to `/accounts/{id}` rather than `/transactions`. No meaningful date scope.
- **Long account names** (e.g., `Expenses:Food:Restaurants:Sushi`). `ReportRowTable` uses CSS `truncate` with the full name in a `title` tooltip — same convention as the Accounts page.
- **`NetWorthGrowthPct` absent or `null` for the default currency** (small histories). The Net card sub-line is omitted; no `NaN` rendering.

## Testing

Vitest + Testing Library, under `spa/src/test/` to match the existing structure.

**Unit (helpers):**

- `lib/period.ts` — `resolvePeriod` for each chip preset, custom-range pass-through, month-only fallback, label formatting, edge cases (Dec → Jan rollover, leap-year Feb).
- `components/reports/ProportionBar.tsx` — proportion math (all-zero total handled, single-row 100%, negative amounts on expense side don't break the bar).

**Component:**

- `KpiCard` — renders amount with default currency, applies `green | red | neutral` variant, renders sub-line when provided.
- `ReportRowTable` — renders rows; clicking a row produces a `<Link>` with the correct `account` / `from` / `to` search params. Empty array → empty-state message.
- `PeriodPicker` — clicking a chip updates the URL search param (`createMemoryHistory`); custom-range mode reveals from/to inputs; invalid range shows inline validation.
- `TabNav` — active tab matches current pathname; clicking a tab navigates.

**Route (happy-path integration):**

- One test per route file. Mock the API client returning a fixture; assert KPI cards / mix bars / tables render with expected numbers. Mirrors how Balances is tested today.

**Out of scope:**

- Backend tests — `/api/reports/*` already have coverage in `internal/api/reports_test.go`.
- Visual-regression / E2E.

**Manual sanity check before merge:**

- Open each tab; period picker works on IS/IB/EB; `Cmd+Click` a row to verify the drill-down URL; verify empty state by setting a far-past range; verify multi-currency footer by adding a second currency in a scratch ledger.

## Files Changed

**Added:**

- `spa/src/routes/reports.tsx`
- `spa/src/routes/reports.index.tsx`
- `spa/src/routes/reports.income-statement.tsx`
- `spa/src/routes/reports.income-breakdown.tsx`
- `spa/src/routes/reports.expense-breakdown.tsx`
- `spa/src/routes/reports.balance-sheet.tsx`
- `spa/src/routes/reports.net-worth.tsx`
- `spa/src/components/reports/TabNav.tsx`
- `spa/src/components/reports/PeriodPicker.tsx`
- `spa/src/components/reports/AsOfPicker.tsx`
- `spa/src/components/reports/KpiCard.tsx`
- `spa/src/components/reports/ProportionBar.tsx`
- `spa/src/components/reports/ReportRowTable.tsx`
- `spa/src/components/reports/CurrencyFooter.tsx`
- `spa/src/lib/api/reports.ts`
- `spa/src/lib/hooks/useReport.ts`
- `spa/src/lib/period.ts`
- Tests under `spa/src/test/` matching the structure above.

**Modified:**

- `spa/src/components/Sidebar.tsx` — `Reports` nav item becomes a link to `/reports`.
- `spa/src/lib/types.ts` — add `ReportRow`, `ReportResult`, `BalanceSheetResult`, `NetWorthResponse`.
- `spa/src/routeTree.gen.ts` — regenerated by TanStack Router.

## Risks & Open Questions

- **`NetWorthGrowthPct` semantics across "custom" ranges.** The backend computes previous-period via `previousPeriodRange`. Confirm during implementation that arbitrary custom ranges yield sensible "previous" windows; if not, omit the growth sub-line for custom ranges.
- **Period sharing across IS/IB/EB tabs.** Currently each tab uses its own search params. If users frequently flip between the three on the same month, lift period state into the parent loader as a v2 polish.
