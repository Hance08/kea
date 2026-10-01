# SPA Ledger Switcher Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add a sidebar dropdown to the Kea SPA that displays the active ledger and lets users switch between registered ledgers without leaving the browser.

**Architecture:** A single self-contained `LedgerSwitcher` component owns the ledger-list `useQuery`, the switch `useMutation`, and the dropdown UI. It replaces the static `"kea"` title in `Sidebar.tsx`. On successful switch, the component invalidates all TanStack Query caches so every panel refetches against the new ledger's data. Toasts (via `sonner`) surface success and error feedback.

**Tech Stack:** React 18 + TypeScript + Vite, TanStack Query v5, TanStack Router, shadcn/ui (Radix-backed) primitives, Tailwind, Vitest + React Testing Library.

**Reference spec:** `docs/superpowers/specs/2026-06-09-spa-ledger-switcher-design.md`.

---

## Pre-flight notes for the implementer

- All work happens in `spa/`. Commands assume `cd spa` unless stated otherwise.
- The repo uses Biome (`npm run check`) — no Prettier/ESLint. Run `npm run check:write` to autofix formatting before committing.
- The existing test file `spa/src/test/balances.test.tsx` mounts the full app via `makeTestApp`, which renders `Sidebar`. Once `Sidebar` calls `<LedgerSwitcher />`, that test's `fetch` stub must also answer `/api/ledgers` or the sidebar will be stuck in its pending state. Task 8 updates it.
- The repo uses conventional commits with scope `(spa)`. No `Co-Authored-By` footer. No SPDX header on TypeScript files.
- TanStack Query v5 uses `isPending` (not `isLoading`) for the initial-fetch state. The `useMutation` object exposes `isPending`, `variables` (the argument passed to `mutate`).
- Radix `DropdownMenu` portals its content to `document.body`. Tests must use `userEvent.click` (not `fireEvent.click`) on the trigger, then query menu items via `screen.findByRole('menuitem', { name: ... })`.

---

## Task 1: Install runtime dependencies

**Files:**
- Modify: `spa/package.json`
- Modify: `spa/package-lock.json`
- Create: `spa/src/components/ui/dropdown-menu.tsx` (generated)
- Create: `spa/src/components/ui/sonner.tsx` (generated)

- [ ] **Step 1: Add shadcn dropdown-menu and sonner primitives**

Run from the repo root:

```bash
cd spa && npx shadcn@latest add dropdown-menu sonner
```

Expected: shadcn writes `src/components/ui/dropdown-menu.tsx` and `src/components/ui/sonner.tsx`, installs `@radix-ui/react-dropdown-menu`, `sonner`, and `next-themes` (a sonner peer dep). If shadcn prompts about config, accept the defaults — `components.json` already exists from PR #185.

- [ ] **Step 2: Add @testing-library/user-event as a dev dep**

The existing tests don't drive interactions; the new tests do. Install:

```bash
cd spa && npm install --save-dev @testing-library/user-event
```

Expected: `@testing-library/user-event` appears under `devDependencies` in `spa/package.json`.

- [ ] **Step 3: Sanity-check the install**

```bash
cd spa && npm run check && npm run build
```

Expected: both commands exit 0. The two new `ui/*.tsx` files are part of the shadcn boilerplate and should be Biome-clean as generated.

- [ ] **Step 4: Commit**

```bash
git add spa/package.json spa/package-lock.json spa/src/components/ui/dropdown-menu.tsx spa/src/components/ui/sonner.tsx
git commit -m "build(spa): add dropdown-menu, sonner, user-event for ledger switcher"
```

---

## Task 2: Add API types and client functions

**Files:**
- Modify: `spa/src/lib/types.ts`
- Modify: `spa/src/lib/api.ts`

This task has no dedicated unit tests — `api.ts` is a thin wrapper over `apiFetch` and is exercised end-to-end through component tests in later tasks. This matches the existing pattern (`getConfig`, `getBalances` have no unit tests).

- [ ] **Step 1: Add types**

Append to `spa/src/lib/types.ts`:

```ts
export interface LedgerInfo {
  name: string;
  path: string;
  active: boolean;
}

export interface LedgerListResponse {
  active: string;
  items: LedgerInfo[];
}
```

- [ ] **Step 2: Add API client functions**

In `spa/src/lib/api.ts`, extend the existing imports and add the two functions.

Update the import line at the top:

```ts
import type {
  AccountBalance,
  LedgerInfo,
  LedgerListResponse,
  ListResult,
  ServerConfig,
} from './types';
```

Append after `getConfig`:

```ts
export function getLedgers(): Promise<LedgerListResponse> {
  return apiFetch<LedgerListResponse>('/api/ledgers');
}

export function switchLedger(name: string): Promise<LedgerInfo> {
  return apiFetch<LedgerInfo>('/api/ledgers/switch', {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ name }),
  });
}
```

- [ ] **Step 3: Type-check**

```bash
cd spa && npm run check && npm run build
```

Expected: exit 0. (No tests run yet — these functions are unused until Task 4.)

- [ ] **Step 4: Commit**

```bash
git add spa/src/lib/types.ts spa/src/lib/api.ts
git commit -m "feat(spa): add ledger list and switch API client"
```

---

## Task 3: Mount the Toaster

**Files:**
- Modify: `spa/src/main.tsx`

- [ ] **Step 1: Add Toaster to the app root**

Open `spa/src/main.tsx`. Add the import near the other library imports:

```ts
import { Toaster } from '@/components/ui/sonner';
```

Update the render call to include `<Toaster />` as a sibling of `<ServerConfigProvider>` inside `<QueryClientProvider>`:

```tsx
createRoot(rootEl).render(
  <StrictMode>
    <QueryClientProvider client={queryClient}>
      <ServerConfigProvider
        fallback={
          <div className="flex min-h-screen items-center justify-center text-sm text-muted-foreground">
            Loading…
          </div>
        }
      >
        {() => <RouterProvider router={router} />}
      </ServerConfigProvider>
      <Toaster />
    </QueryClientProvider>
  </StrictMode>,
);
```

- [ ] **Step 2: Type-check and build**

```bash
cd spa && npm run check && npm run build
```

Expected: exit 0.

- [ ] **Step 3: Commit**

```bash
git add spa/src/main.tsx
git commit -m "feat(spa): mount sonner Toaster at app root"
```

---

## Task 4: TDD LedgerSwitcher — trigger shows active ledger

This task starts the component using TDD. We add one capability per task, each driven by a failing test. Each task ends green and committable.

**Files:**
- Create: `spa/src/test/ledger-switcher.test.tsx`
- Create: `spa/src/components/LedgerSwitcher.tsx`

- [ ] **Step 1: Write the failing test**

Create `spa/src/test/ledger-switcher.test.tsx`:

```tsx
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { render, screen } from '@testing-library/react';
import { afterEach, beforeEach, expect, test, vi } from 'vitest';
import { LedgerSwitcher } from '../components/LedgerSwitcher';

const okResponse = (body: unknown) =>
  new Response(JSON.stringify(body), {
    status: 200,
    headers: { 'Content-Type': 'application/json' },
  });

const LEDGERS_OK = {
  active: 'personal',
  items: [
    { name: 'personal', path: '/p/personal.db', active: true },
    { name: 'business', path: '/p/business.db', active: false },
  ],
};

function renderSwitcher() {
  const queryClient = new QueryClient({
    defaultOptions: { queries: { retry: false, gcTime: 0 } },
  });
  return render(
    <QueryClientProvider client={queryClient}>
      <LedgerSwitcher />
    </QueryClientProvider>,
  );
}

beforeEach(() => {
  vi.stubGlobal(
    'fetch',
    vi.fn((url: string) => {
      if (url === '/api/ledgers') {
        return Promise.resolve(okResponse(LEDGERS_OK));
      }
      throw new Error(`unexpected fetch: ${url}`);
    }),
  );
});

afterEach(() => {
  vi.unstubAllGlobals();
});

test('trigger shows the active ledger name', async () => {
  renderSwitcher();
  expect(await screen.findByRole('button', { name: /personal/i })).toBeInTheDocument();
});
```

- [ ] **Step 2: Run the test and watch it fail**

```bash
cd spa && npm run test -- ledger-switcher
```

Expected: FAIL — module `../components/LedgerSwitcher` not found.

- [ ] **Step 3: Create the minimal component**

Create `spa/src/components/LedgerSwitcher.tsx`:

```tsx
import { Button } from '@/components/ui/button';
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuTrigger,
} from '@/components/ui/dropdown-menu';
import { Skeleton } from '@/components/ui/skeleton';
import { getLedgers } from '@/lib/api';
import { useQuery } from '@tanstack/react-query';
import { ChevronDown } from 'lucide-react';

export function LedgerSwitcher() {
  const ledgersQuery = useQuery({ queryKey: ['ledgers'], queryFn: getLedgers });

  if (ledgersQuery.isPending) {
    return <Skeleton data-testid="ledger-switcher-skeleton" className="mb-6 h-7 w-24" />;
  }

  if (ledgersQuery.isError) {
    return <div className="mb-6 text-lg font-semibold tracking-tight">kea</div>;
  }

  const { active, items } = ledgersQuery.data;

  return (
    <div className="mb-6">
      <DropdownMenu>
        <DropdownMenuTrigger asChild>
          <Button
            variant="ghost"
            className="-mx-2 flex w-[calc(100%+1rem)] items-center justify-between px-2 text-lg font-semibold tracking-tight"
          >
            <span>{active}</span>
            <ChevronDown className="h-4 w-4 opacity-60" />
          </Button>
        </DropdownMenuTrigger>
        <DropdownMenuContent align="start" className="min-w-56">
          {items.map((item) => (
            <DropdownMenuItem key={item.name}>{item.name}</DropdownMenuItem>
          ))}
        </DropdownMenuContent>
      </DropdownMenu>
    </div>
  );
}
```

Note: `lucide-react` is already a transitive dep of shadcn primitives — confirm with `npm ls lucide-react` after Task 1 if uncertain. If it is not installed, add `npm install lucide-react`.

- [ ] **Step 4: Run the test and confirm it passes**

```bash
cd spa && npm run test -- ledger-switcher
```

Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add spa/src/components/LedgerSwitcher.tsx spa/src/test/ledger-switcher.test.tsx
git commit -m "feat(spa): show active ledger name in sidebar"
```

---

## Task 5: TDD LedgerSwitcher — menu lists all ledgers, active row is disabled with check-mark

**Files:**
- Modify: `spa/src/test/ledger-switcher.test.tsx`
- Modify: `spa/src/components/LedgerSwitcher.tsx`

- [ ] **Step 1: Add the failing test**

Append to `spa/src/test/ledger-switcher.test.tsx`:

```tsx
import userEvent from '@testing-library/user-event';

test('opens menu, lists ledgers, marks active row disabled with check', async () => {
  renderSwitcher();
  const trigger = await screen.findByRole('button', { name: /personal/i });
  await userEvent.click(trigger);

  const personal = await screen.findByRole('menuitem', { name: /personal/i });
  const business = await screen.findByRole('menuitem', { name: /business/i });

  // Active row carries an aria-disabled marker (Radix sets data-disabled).
  expect(personal).toHaveAttribute('data-disabled');
  expect(business).not.toHaveAttribute('data-disabled');

  // The active row contains the check-mark indicator we tag with a testid.
  expect(personal.querySelector('[data-testid="ledger-active-check"]')).not.toBeNull();
  expect(business.querySelector('[data-testid="ledger-active-check"]')).toBeNull();
});
```

Note: place the `import userEvent` line with the other imports at the top, not inside the test body.

- [ ] **Step 2: Run the test and watch it fail**

```bash
cd spa && npm run test -- ledger-switcher
```

Expected: FAIL — `data-disabled` attribute missing and `ledger-active-check` element not found.

- [ ] **Step 3: Update the component to mark the active row**

Edit `spa/src/components/LedgerSwitcher.tsx`. Update the imports:

```ts
import { Check, ChevronDown } from 'lucide-react';
```

Replace the `items.map(...)` block inside `<DropdownMenuContent>`:

```tsx
{items.map((item) => {
  const isActive = item.name === active;
  return (
    <DropdownMenuItem
      key={item.name}
      disabled={isActive}
      className="flex items-center justify-between"
    >
      <span>{item.name}</span>
      {isActive ? (
        <Check data-testid="ledger-active-check" className="h-4 w-4" />
      ) : null}
    </DropdownMenuItem>
  );
})}
```

- [ ] **Step 4: Run the test and confirm it passes**

```bash
cd spa && npm run test -- ledger-switcher
```

Expected: PASS for both tests in the file.

- [ ] **Step 5: Commit**

```bash
git add spa/src/components/LedgerSwitcher.tsx spa/src/test/ledger-switcher.test.tsx
git commit -m "feat(spa): mark active ledger row with check and disable"
```

---

## Task 6: TDD LedgerSwitcher — switching ledger triggers POST and success toast

**Files:**
- Modify: `spa/src/test/ledger-switcher.test.tsx`
- Modify: `spa/src/components/LedgerSwitcher.tsx`

- [ ] **Step 1: Replace the `beforeEach` stub with a switch-aware version**

At the top of `spa/src/test/ledger-switcher.test.tsx`, add helpers for capturing fetch calls and mocking `sonner`. Replace the existing `beforeEach` and add a mock for sonner. The full updated header (imports + helpers + mocks + beforeEach) becomes:

```tsx
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { render, screen } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { afterEach, beforeEach, expect, test, vi } from 'vitest';
import { LedgerSwitcher } from '../components/LedgerSwitcher';

const okResponse = (body: unknown) =>
  new Response(JSON.stringify(body), {
    status: 200,
    headers: { 'Content-Type': 'application/json' },
  });

const errorResponse = (status: number, body: unknown) =>
  new Response(JSON.stringify(body), {
    status,
    headers: { 'Content-Type': 'application/json' },
  });

const LEDGERS_OK = {
  active: 'personal',
  items: [
    { name: 'personal', path: '/p/personal.db', active: true },
    { name: 'business', path: '/p/business.db', active: false },
  ],
};

// vi.mock factories are hoisted above top-level `const`s, so referencing
// outer `vi.fn()` values directly throws. Use vi.hoisted to share state safely.
const { toastSuccess, toastError } = vi.hoisted(() => ({
  toastSuccess: vi.fn(),
  toastError: vi.fn(),
}));
vi.mock('sonner', () => ({
  toast: {
    success: (...args: unknown[]) => toastSuccess(...args),
    error: (...args: unknown[]) => toastError(...args),
  },
}));

function renderSwitcher() {
  const queryClient = new QueryClient({
    defaultOptions: { queries: { retry: false, gcTime: 0 } },
  });
  return render(
    <QueryClientProvider client={queryClient}>
      <LedgerSwitcher />
    </QueryClientProvider>,
  );
}

let fetchCalls: Array<{ url: string; init?: RequestInit }>;

beforeEach(() => {
  fetchCalls = [];
  toastSuccess.mockReset();
  toastError.mockReset();
  vi.stubGlobal(
    'fetch',
    vi.fn((url: string, init?: RequestInit) => {
      fetchCalls.push({ url, init });
      if (url === '/api/ledgers') {
        return Promise.resolve(okResponse(LEDGERS_OK));
      }
      if (url === '/api/ledgers/switch') {
        return Promise.resolve(
          okResponse({ name: 'business', path: '/p/business.db', active: true }),
        );
      }
      throw new Error(`unexpected fetch: ${url}`);
    }),
  );
});

afterEach(() => {
  vi.unstubAllGlobals();
});
```

Keep the two previously written tests below unchanged.

- [ ] **Step 2: Add the failing test**

Append:

```tsx
test('clicking non-active row POSTs /api/ledgers/switch and shows success toast', async () => {
  renderSwitcher();
  const trigger = await screen.findByRole('button', { name: /personal/i });
  await userEvent.click(trigger);

  const business = await screen.findByRole('menuitem', { name: /business/i });
  await userEvent.click(business);

  const switchCall = fetchCalls.find((c) => c.url === '/api/ledgers/switch');
  expect(switchCall).toBeDefined();
  expect(switchCall?.init?.method).toBe('POST');
  expect(switchCall?.init?.body).toBe(JSON.stringify({ name: 'business' }));

  await vi.waitFor(() => {
    expect(toastSuccess).toHaveBeenCalledWith('Switched to business');
  });
});
```

- [ ] **Step 3: Run the test and watch it fail**

```bash
cd spa && npm run test -- ledger-switcher
```

Expected: FAIL — clicking the menu item does nothing; no POST is made.

- [ ] **Step 4: Wire the mutation into the component**

Edit `spa/src/components/LedgerSwitcher.tsx`. Update imports:

```ts
import { switchLedger, getLedgers } from '@/lib/api';
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { toast } from 'sonner';
```

Add the mutation hook at the top of the component:

```ts
const queryClient = useQueryClient();
const ledgersQuery = useQuery({ queryKey: ['ledgers'], queryFn: getLedgers });
const mutation = useMutation({
  mutationFn: (name: string) => switchLedger(name),
  onSuccess: (info) => {
    queryClient.invalidateQueries();
    toast.success(`Switched to ${info.name}`);
  },
  onError: (err) => {
    toast.error(err instanceof Error ? err.message : 'Switch failed');
  },
});
```

Add an `onSelect` handler to each non-active menu item:

```tsx
{items.map((item) => {
  const isActive = item.name === active;
  return (
    <DropdownMenuItem
      key={item.name}
      disabled={isActive}
      onSelect={(e) => {
        if (isActive) {
          e.preventDefault();
          return;
        }
        mutation.mutate(item.name);
      }}
      className="flex items-center justify-between"
    >
      <span>{item.name}</span>
      {isActive ? (
        <Check data-testid="ledger-active-check" className="h-4 w-4" />
      ) : null}
    </DropdownMenuItem>
  );
})}
```

- [ ] **Step 5: Run the test and confirm it passes**

```bash
cd spa && npm run test -- ledger-switcher
```

Expected: all three tests PASS.

- [ ] **Step 6: Commit**

```bash
git add spa/src/components/LedgerSwitcher.tsx spa/src/test/ledger-switcher.test.tsx
git commit -m "feat(spa): switch ledger via POST and toast on success"
```

---

## Task 7: TDD LedgerSwitcher — failure toast and in-flight disabling

**Files:**
- Modify: `spa/src/test/ledger-switcher.test.tsx`
- Modify: `spa/src/components/LedgerSwitcher.tsx`

- [ ] **Step 1: Add the failure-toast test**

Append to `spa/src/test/ledger-switcher.test.tsx`:

```tsx
test('failed switch shows error toast with API message', async () => {
  // Replace the default fetch stub for this test: make /switch fail.
  vi.stubGlobal(
    'fetch',
    vi.fn((url: string) => {
      if (url === '/api/ledgers') {
        return Promise.resolve(okResponse(LEDGERS_OK));
      }
      if (url === '/api/ledgers/switch') {
        return Promise.resolve(
          errorResponse(404, { message: 'ledger not found: "business"' }),
        );
      }
      throw new Error(`unexpected fetch: ${url}`);
    }),
  );

  renderSwitcher();
  await userEvent.click(await screen.findByRole('button', { name: /personal/i }));
  await userEvent.click(await screen.findByRole('menuitem', { name: /business/i }));

  await vi.waitFor(() => {
    expect(toastError).toHaveBeenCalledWith('ledger not found: "business"');
  });
});
```

- [ ] **Step 2: Add the pending-disables-siblings test**

Append:

```tsx
test('while switch pending, other menu items are disabled', async () => {
  // Build a fetch stub where /switch returns a Promise we can resolve manually.
  let resolveSwitch: (value: Response) => void = () => {
    /* assigned below */
  };
  const switchPromise = new Promise<Response>((res) => {
    resolveSwitch = res;
  });

  const threeLedgers = {
    active: 'personal',
    items: [
      { name: 'personal', path: '/p/personal.db', active: true },
      { name: 'business', path: '/p/business.db', active: false },
      { name: 'savings', path: '/p/savings.db', active: false },
    ],
  };

  vi.stubGlobal(
    'fetch',
    vi.fn((url: string) => {
      if (url === '/api/ledgers') {
        return Promise.resolve(okResponse(threeLedgers));
      }
      if (url === '/api/ledgers/switch') {
        return switchPromise;
      }
      throw new Error(`unexpected fetch: ${url}`);
    }),
  );

  renderSwitcher();
  await userEvent.click(await screen.findByRole('button', { name: /personal/i }));
  await userEvent.click(await screen.findByRole('menuitem', { name: /business/i }));

  // While the switch is in-flight, the third (other) row should be disabled.
  const savings = await screen.findByRole('menuitem', { name: /savings/i });
  expect(savings).toHaveAttribute('data-disabled');

  // Resolve to let the test tear down cleanly.
  resolveSwitch(okResponse({ name: 'business', path: '/p/business.db', active: true }));
});
```

- [ ] **Step 3: Run the tests and watch them fail**

```bash
cd spa && npm run test -- ledger-switcher
```

Expected:
- "failed switch" FAILs — `toast.error` is wired but the existing test passes; actually this should already pass because the mutation already calls `toast.error` on failure. Run it; if it passes, that is fine — the test still locks in behavior. If it fails, inspect the error message format.
- "while switch pending" FAILs — non-active rows are not disabled during pending.

- [ ] **Step 4: Disable non-active rows while mutation is pending**

Edit `spa/src/components/LedgerSwitcher.tsx`. Update the menu-item disabling logic and the trigger:

```tsx
const { active, items } = ledgersQuery.data;

return (
  <div className="mb-6">
    <DropdownMenu>
      <DropdownMenuTrigger asChild disabled={mutation.isPending}>
        <Button
          variant="ghost"
          className="-mx-2 flex w-[calc(100%+1rem)] items-center justify-between px-2 text-lg font-semibold tracking-tight"
        >
          <span>{active}</span>
          <ChevronDown className="h-4 w-4 opacity-60" />
        </Button>
      </DropdownMenuTrigger>
      <DropdownMenuContent align="start" className="min-w-56">
        {items.map((item) => {
          const isActive = item.name === active;
          const isSwitching = mutation.isPending && mutation.variables === item.name;
          return (
            <DropdownMenuItem
              key={item.name}
              disabled={isActive || mutation.isPending}
              onSelect={(e) => {
                if (isActive || mutation.isPending) {
                  e.preventDefault();
                  return;
                }
                mutation.mutate(item.name);
              }}
              className="flex items-center justify-between"
            >
              <span>{item.name}</span>
              {isSwitching ? (
                <Loader2 data-testid="ledger-switching-spinner" className="h-4 w-4 animate-spin" />
              ) : isActive ? (
                <Check data-testid="ledger-active-check" className="h-4 w-4" />
              ) : null}
            </DropdownMenuItem>
          );
        })}
      </DropdownMenuContent>
    </DropdownMenu>
  </div>
);
```

Update the icon import:

```ts
import { Check, ChevronDown, Loader2 } from 'lucide-react';
```

- [ ] **Step 5: Run all tests in the file and confirm green**

```bash
cd spa && npm run test -- ledger-switcher
```

Expected: all five tests PASS.

- [ ] **Step 6: Commit**

```bash
git add spa/src/components/LedgerSwitcher.tsx spa/src/test/ledger-switcher.test.tsx
git commit -m "feat(spa): toast on switch failure, disable rows while switch pending"
```

---

## Task 8: Wire LedgerSwitcher into Sidebar

**Files:**
- Modify: `spa/src/components/Sidebar.tsx`
- Modify: `spa/src/test/balances.test.tsx`

- [ ] **Step 1: Update `Sidebar.tsx`**

Replace the static header line with the new component. The whole file becomes:

```tsx
import { LedgerSwitcher } from '@/components/LedgerSwitcher';
import { cn } from '@/lib/cn';
import { Link, useRouterState } from '@tanstack/react-router';

interface NavItem {
  label: string;
  to?: string;
}

const NAV: NavItem[] = [
  { label: 'Balances', to: '/balances' },
  { label: 'Accounts' },
  { label: 'Transactions' },
  { label: 'Reports' },
  { label: 'Reconcile' },
];

export function Sidebar() {
  const { location } = useRouterState();
  return (
    <nav aria-label="Main navigation" className="w-56 shrink-0 border-r bg-muted/30 p-4">
      <LedgerSwitcher />
      <ul className="space-y-1">
        {NAV.map((item) => {
          if (!item.to) {
            return (
              <li key={item.label}>
                <span
                  aria-disabled="true"
                  className="block cursor-not-allowed rounded px-3 py-2 text-sm text-muted-foreground"
                  title="Coming soon"
                >
                  {item.label}
                </span>
              </li>
            );
          }
          const isActive = location.pathname === item.to;
          return (
            <li key={item.label}>
              <Link
                to={item.to}
                className={cn(
                  'block rounded px-3 py-2 text-sm transition-colors',
                  isActive ? 'bg-primary text-primary-foreground font-medium' : 'hover:bg-muted',
                )}
              >
                {item.label}
              </Link>
            </li>
          );
        })}
      </ul>
    </nav>
  );
}
```

- [ ] **Step 2: Update `balances.test.tsx` fetch stub**

Now that `Sidebar` always renders `LedgerSwitcher`, the balances test must answer `/api/ledgers` or React Query will keep the switcher in the pending state and may impact stability of related assertions.

In `spa/src/test/balances.test.tsx`, extend the `beforeEach` fetch stub. Add a branch for `/api/ledgers` before the final `throw`:

```ts
if (url === '/api/ledgers') {
  return Promise.resolve(
    okResponse({
      active: 'personal',
      items: [{ name: 'personal', path: '/p/personal.db', active: true }],
    }),
  );
}
```

- [ ] **Step 3: Run the full SPA test suite**

```bash
cd spa && npm run test
```

Expected: all tests PASS — both the new `ledger-switcher.test.tsx` suite and the updated `balances.test.tsx`.

- [ ] **Step 4: Commit**

```bash
git add spa/src/components/Sidebar.tsx spa/src/test/balances.test.tsx
git commit -m "feat(spa): replace sidebar title with ledger switcher"
```

---

## Task 9: Final verification

**Files:** None (verification only).

- [ ] **Step 1: Run SPA verification suite**

```bash
cd spa && npm run check && npm run test && npm run build
```

Expected: all three commands exit 0. If `npm run check` flags formatting, run `npm run check:write` and amend the relevant commit (or create a small `style(spa)` follow-up commit).

- [ ] **Step 2: Run Go verification**

```bash
go test ./... && go build ./...
```

Expected: exit 0. No Go changes were made, so this is just a smoke check.

- [ ] **Step 3: Manual smoke test (recommended, not strictly required)**

Skip if running unattended. Otherwise:

1. From repo root: `make build && ./kea_test ledger add personal && ./kea_test ledger add business`.
2. Run `./kea_test serve` in one terminal.
3. Run `cd spa && npm run dev` in another.
4. Open `http://localhost:5173/balances`.
5. Confirm the sidebar header shows `personal` with a chevron.
6. Click it, switch to `business`. Confirm: toast appears, sidebar updates to show `business`, balances panel refetches.

- [ ] **Step 4: Branch hygiene check**

```bash
git status   # should be clean
git log --oneline master..HEAD   # confirm commits look right
```

Expected: a small, ordered series of `build(spa)` and `feat(spa)` commits.

---

## Self-review notes (for the implementer's reference)

- The spec calls for five test cases — all five are present (Tasks 4–7).
- The spec calls for a self-contained `LedgerSwitcher.tsx` — Task 4 creates it; Tasks 5–7 grow it.
- The spec calls for the `Toaster` to mount once at the root — Task 3 does that.
- The spec calls for `getLedgers` / `switchLedger` / new types — Task 2 covers them.
- The spec leaves `useActiveLedger` deferred — this plan does not introduce it.
- The spec keeps the sidebar's nav structure unchanged — Task 8 preserves the existing `NAV` list verbatim.
