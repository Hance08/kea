# kea SPA

React SPA served by `kea serve` (embedded into the Go binary) and developed with the Vite dev server.

> Patterns for adding pages: [../docs/recipes/add-spa-page.md](../docs/recipes/add-spa-page.md). Toolchain details: [../docs/development.md](../docs/development.md#spa-toolchain).

## Stack

- Vite, React 18, TypeScript
- TanStack Router (file-based routes in `src/routes`) and TanStack Query
- Tailwind CSS, shadcn/ui-style components (Radix primitives, class-variance-authority, lucide-react), next-themes, sonner toasts
- Forms use controlled `useState` (react-hook-form is installed but unused); zod validates search params in `spa/src/lib/*-search-params.ts`; react-grid-layout for the dashboard
- Biome for lint and format; Vitest with Testing Library and jsdom for tests

## Routes

| URL | Route file | Purpose |
| --- | --- | --- |
| `/` | `spa/src/routes/index.tsx` | Redirects to `/dashboard` |
| `/dashboard` | `spa/src/routes/dashboard.tsx` | Customizable dashboard |
| `/balances` | `spa/src/routes/balances.tsx` | Account balances |
| `/accounts` | `spa/src/routes/accounts.index.tsx` | Account list (layout: `accounts.tsx`) |
| `/accounts/new` | `spa/src/routes/accounts.new.tsx` | Create an account |
| `/accounts/$id` | `spa/src/routes/accounts.$id.index.tsx` | Account detail (layout: `accounts.$id.tsx`) |
| `/accounts/$id/edit` | `spa/src/routes/accounts.$id.edit.tsx` | Edit an account |
| `/transactions` | `spa/src/routes/transactions.index.tsx` | Transaction list (layout: `transactions.tsx`) |
| `/transactions/new` | `spa/src/routes/transactions.new.tsx` | Create a transaction |
| `/transactions/$id` | `spa/src/routes/transactions.$id.index.tsx` | Transaction detail (layout: `transactions.$id.tsx`) |
| `/transactions/$id/edit` | `spa/src/routes/transactions.$id.edit.tsx` | Edit a transaction |
| `/reconcile` | `spa/src/routes/reconcile.index.tsx` | Choose an account to reconcile (layout: `reconcile.tsx`) |
| `/reconcile/$id` | `spa/src/routes/reconcile.$id.tsx` | Reconcile one account |
| `/reports` | `spa/src/routes/reports.index.tsx` | Redirects to `/reports/income-statement` (layout: `reports.tsx`) |
| `/reports/balance-sheet` | `spa/src/routes/reports.balance-sheet.tsx` | Balance sheet report |
| `/reports/income-statement` | `spa/src/routes/reports.income-statement.tsx` | Income statement report |
| `/reports/income-breakdown` | `spa/src/routes/reports.income-breakdown.tsx` | Income breakdown report |
| `/reports/expense-breakdown` | `spa/src/routes/reports.expense-breakdown.tsx` | Expense breakdown report |
| `/reports/net-worth` | `spa/src/routes/reports.net-worth.tsx` | Redirects to `/reports/balance-sheet` |
| `/reports/budget` | `spa/src/routes/reports.budget.tsx` | Budget vs actual report |
| `/budgets` | `spa/src/routes/budgets.tsx` | Budget settings (set, stop, delete; currency resolved from the account, hidden accounts included) |
| `/reports/savings` | `spa/src/routes/reports.savings.tsx` | Savings target vs saved, month and year to date |
| `/savings` | `spa/src/routes/savings.tsx` | Savings target settings (set, stop, delete; Asset accounts only) |
| `/settings` | `spa/src/routes/settings.tsx` | Settings |

The root layout is `spa/src/routes/__root.tsx`.

## Commands

Run from `spa/` (Makefile equivalents run from the repo root):

```bash
npm install           # make spa-install
npm run dev           # make spa-dev; Vite on :5173, proxies /api to http://localhost:8080
npm test              # Vitest, single run (npm run test:watch to watch)
npm run check         # Biome lint and format check (npm run check:write to fix)
npm run build         # make spa-build; type-checks, then writes internal/web/dist
```

`npm run build` empties `internal/web/dist` and rewrites its placeholder `index.html`. Do not commit that change; `make spa-clean` restores it.
