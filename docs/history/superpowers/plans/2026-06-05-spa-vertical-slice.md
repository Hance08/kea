# SPA Vertical Slice — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Stand up a React SPA at `spa/` paired with `kea serve`, shipping one working route — `/balances` — backed by `GET /api/balances`, with the sidebar shell the rest of the app will grow into.

**Architecture:** Vite + React + TypeScript workspace at `spa/`. TanStack Router for routing (file-based, persistent sidebar in `__root.tsx`). TanStack Query for fetching the bulk balances endpoint. Tailwind + shadcn/ui for styling and accessible primitives. Biome for lint/format. Vitest for tests (one smoke test, plus unit tests for pure helpers). Single commit on `feat/spa-vertical-slice`.

**Tech Stack:** Vite 5, React 18, TypeScript 5, @tanstack/react-router, @tanstack/react-query v5, Tailwind 3, shadcn/ui, Biome, Vitest, @testing-library/react, jsdom.

**Spec:** [`docs/superpowers/specs/2026-06-05-spa-vertical-slice-design.md`](../specs/2026-06-05-spa-vertical-slice-design.md)

---

## Prerequisites

Before Task 1: verify Node and npm are installed. Run `node --version && npm --version`. Node 20+ recommended (Vite 5 requires Node 18+; latest LTS is preferred). If missing, STOP and report — the implementer cannot proceed without these.

The working branch is `feat/spa-vertical-slice`. The controller will create it before dispatching.

---

## File Map

```
spa/                                      # all new
├── .gitignore
├── .env.example
├── README.md
├── package.json
├── tsconfig.json
├── tsconfig.node.json
├── vite.config.ts                        # dev server + proxy + Vitest config
├── biome.json
├── postcss.config.js
├── tailwind.config.js
├── components.json                       # shadcn/ui config
├── index.html
├── public/
│   └── favicon.svg
└── src/
    ├── main.tsx
    ├── styles/
    │   └── globals.css
    ├── lib/
    │   ├── types.ts
    │   ├── api.ts
    │   ├── format.ts
    │   ├── format.test.ts
    │   ├── balances.ts
    │   └── balances.test.ts
    ├── components/
    │   ├── Sidebar.tsx
    │   ├── NetWorthCard.tsx
    │   ├── TypeTotalCard.tsx
    │   ├── AccountListRow.tsx
    │   └── ui/                           # shadcn-generated
    │       ├── button.tsx
    │       ├── card.tsx
    │       ├── skeleton.tsx
    │       └── alert.tsx
    ├── routes/
    │   ├── __root.tsx
    │   ├── index.tsx
    │   └── balances.tsx
    └── test/
        ├── setup.ts
        └── balances.test.tsx

# Modified at repo root (not in spa/)
.gitignore                                # add spa/node_modules, spa/dist
Makefile                                  # add spa-install, spa-dev, spa-build
```

---

### Task 1: Workspace scaffolding

**Files:**
- Create: `spa/package.json`, `spa/tsconfig.json`, `spa/tsconfig.node.json`, `spa/vite.config.ts`, `spa/index.html`, `spa/.gitignore`, `spa/.env.example`, `spa/public/favicon.svg`

- [ ] **Step 1: Create `spa/` directory and `package.json`**

Run from repo root: `mkdir -p spa/public spa/src/{lib,components/ui,routes,styles,test}`.

Write `spa/package.json`:

```json
{
  "name": "kea-spa",
  "private": true,
  "type": "module",
  "version": "0.0.0",
  "scripts": {
    "dev": "vite",
    "build": "tsc -b && vite build",
    "preview": "vite preview",
    "test": "vitest run",
    "test:watch": "vitest",
    "check": "biome check src",
    "check:write": "biome check --write src"
  },
  "dependencies": {
    "@tanstack/react-query": "^5.51.0",
    "@tanstack/react-router": "^1.46.0",
    "class-variance-authority": "^0.7.0",
    "clsx": "^2.1.0",
    "lucide-react": "^0.400.0",
    "react": "^18.3.0",
    "react-dom": "^18.3.0",
    "tailwind-merge": "^2.4.0",
    "tailwindcss-animate": "^1.0.7"
  },
  "devDependencies": {
    "@biomejs/biome": "^1.8.0",
    "@tanstack/router-vite-plugin": "^1.46.0",
    "@testing-library/jest-dom": "^6.4.0",
    "@testing-library/react": "^16.0.0",
    "@types/node": "^20.14.0",
    "@types/react": "^18.3.0",
    "@types/react-dom": "^18.3.0",
    "@vitejs/plugin-react": "^4.3.0",
    "autoprefixer": "^10.4.19",
    "jsdom": "^24.1.0",
    "postcss": "^8.4.38",
    "tailwindcss": "^3.4.0",
    "typescript": "^5.4.0",
    "vite": "^5.3.0",
    "vitest": "^2.0.0"
  }
}
```

Version pins use `^` so npm fetches a recent compatible. If a version no longer exists on npm at install time, the implementer should bump to the latest minor in the same major and note it.

- [ ] **Step 2: Create `spa/tsconfig.json`**

```json
{
  "compilerOptions": {
    "target": "ES2022",
    "useDefineForClassFields": true,
    "lib": ["ES2022", "DOM", "DOM.Iterable"],
    "module": "ESNext",
    "skipLibCheck": true,
    "moduleResolution": "bundler",
    "allowImportingTsExtensions": true,
    "resolveJsonModule": true,
    "isolatedModules": true,
    "moduleDetection": "force",
    "noEmit": true,
    "jsx": "react-jsx",
    "strict": true,
    "noUnusedLocals": true,
    "noUnusedParameters": true,
    "noFallthroughCasesInSwitch": true,
    "baseUrl": ".",
    "paths": { "@/*": ["./src/*"] },
    "types": ["vitest/globals", "@testing-library/jest-dom"]
  },
  "include": ["src"],
  "references": [{ "path": "./tsconfig.node.json" }]
}
```

- [ ] **Step 3: Create `spa/tsconfig.node.json`**

```json
{
  "compilerOptions": {
    "composite": true,
    "skipLibCheck": true,
    "module": "ESNext",
    "moduleResolution": "bundler",
    "allowSyntheticDefaultImports": true,
    "strict": true
  },
  "include": ["vite.config.ts"]
}
```

- [ ] **Step 4: Create `spa/vite.config.ts`**

Includes the `/api/**` proxy to `localhost:8080` and the Vitest config (kept in the same file for one source of truth):

```ts
import { defineConfig } from 'vite';
import react from '@vitejs/plugin-react';
import { TanStackRouterVite } from '@tanstack/router-vite-plugin';
import path from 'node:path';

export default defineConfig({
  plugins: [TanStackRouterVite(), react()],
  resolve: {
    alias: { '@': path.resolve(__dirname, './src') },
  },
  server: {
    port: 5173,
    proxy: {
      '/api': {
        target: 'http://localhost:8080',
        changeOrigin: false,
      },
    },
  },
  test: {
    environment: 'jsdom',
    globals: true,
    setupFiles: ['./src/test/setup.ts'],
    css: false,
  },
});
```

- [ ] **Step 5: Create `spa/index.html`**

```html
<!doctype html>
<html lang="en">
  <head>
    <meta charset="UTF-8" />
    <link rel="icon" type="image/svg+xml" href="/favicon.svg" />
    <meta name="viewport" content="width=device-width, initial-scale=1.0" />
    <title>kea</title>
  </head>
  <body>
    <div id="root"></div>
    <script type="module" src="/src/main.tsx"></script>
  </body>
</html>
```

- [ ] **Step 6: Create `spa/.gitignore`**

```
node_modules
dist
.DS_Store
*.local
.env
.env.*
!.env.example
coverage
```

- [ ] **Step 7: Create `spa/.env.example`**

```
# Currency used for the Balances headline. Falls back to USD if unset.
VITE_DEFAULT_CURRENCY=USD
```

- [ ] **Step 8: Create `spa/public/favicon.svg`**

A minimal placeholder. Real branding is out of scope.

```xml
<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 32 32"><rect width="32" height="32" rx="6" fill="#2563eb"/><text x="16" y="22" text-anchor="middle" font-family="sans-serif" font-size="18" font-weight="700" fill="white">k</text></svg>
```

- [ ] **Step 9: Install dependencies**

Run: `cd spa && npm install`

Expected: completes without errors. Generates `spa/package-lock.json`. If any package version 404s, replace `^X.Y.0` with the latest available in the same major.

- [ ] **Step 10: Smoke-check the dev server starts**

Run: `cd spa && timeout 10 npm run dev || true`

Expected: Vite prints `Local: http://localhost:5173/` within a couple seconds. (`timeout 10` kills it; we're just confirming startup. `|| true` swallows the `timeout` exit code.) No need for an interactive check yet — `main.tsx` doesn't exist, so the page will 500, but the server starts.

If Vite fails to start, STOP and fix before moving on.

---

### Task 2: Tooling configs — Tailwind, Biome, TypeScript paths

**Files:**
- Create: `spa/tailwind.config.js`, `spa/postcss.config.js`, `spa/biome.json`, `spa/src/styles/globals.css`, `spa/src/lib/cn.ts`

- [ ] **Step 1: `spa/postcss.config.js`**

```js
export default {
  plugins: {
    tailwindcss: {},
    autoprefixer: {},
  },
};
```

- [ ] **Step 2: `spa/tailwind.config.js`**

Standard shadcn-compatible config (the `npx shadcn init` in Task 3 expects this shape):

```js
/** @type {import('tailwindcss').Config} */
export default {
  darkMode: ['class'],
  content: ['./index.html', './src/**/*.{ts,tsx}'],
  theme: {
    container: {
      center: true,
      padding: '2rem',
      screens: { '2xl': '1400px' },
    },
    extend: {
      colors: {
        border: 'hsl(var(--border))',
        input: 'hsl(var(--input))',
        ring: 'hsl(var(--ring))',
        background: 'hsl(var(--background))',
        foreground: 'hsl(var(--foreground))',
        primary: {
          DEFAULT: 'hsl(var(--primary))',
          foreground: 'hsl(var(--primary-foreground))',
        },
        secondary: {
          DEFAULT: 'hsl(var(--secondary))',
          foreground: 'hsl(var(--secondary-foreground))',
        },
        destructive: {
          DEFAULT: 'hsl(var(--destructive))',
          foreground: 'hsl(var(--destructive-foreground))',
        },
        muted: {
          DEFAULT: 'hsl(var(--muted))',
          foreground: 'hsl(var(--muted-foreground))',
        },
        accent: {
          DEFAULT: 'hsl(var(--accent))',
          foreground: 'hsl(var(--accent-foreground))',
        },
        card: {
          DEFAULT: 'hsl(var(--card))',
          foreground: 'hsl(var(--card-foreground))',
        },
      },
      borderRadius: {
        lg: 'var(--radius)',
        md: 'calc(var(--radius) - 2px)',
        sm: 'calc(var(--radius) - 4px)',
      },
    },
  },
  plugins: [require('tailwindcss-animate')],
};
```

- [ ] **Step 3: `spa/src/styles/globals.css`**

Standard shadcn theme tokens + Tailwind directives:

```css
@tailwind base;
@tailwind components;
@tailwind utilities;

@layer base {
  :root {
    --background: 0 0% 100%;
    --foreground: 222.2 84% 4.9%;
    --card: 0 0% 100%;
    --card-foreground: 222.2 84% 4.9%;
    --primary: 221.2 83.2% 53.3%;
    --primary-foreground: 210 40% 98%;
    --secondary: 210 40% 96.1%;
    --secondary-foreground: 222.2 47.4% 11.2%;
    --muted: 210 40% 96.1%;
    --muted-foreground: 215.4 16.3% 46.9%;
    --accent: 210 40% 96.1%;
    --accent-foreground: 222.2 47.4% 11.2%;
    --destructive: 0 84.2% 60.2%;
    --destructive-foreground: 210 40% 98%;
    --border: 214.3 31.8% 91.4%;
    --input: 214.3 31.8% 91.4%;
    --ring: 221.2 83.2% 53.3%;
    --radius: 0.5rem;
  }

  * { @apply border-border; }
  body { @apply bg-background text-foreground; }
}
```

- [ ] **Step 4: `spa/src/lib/cn.ts`** — shadcn's standard class-merging helper

```ts
import { type ClassValue, clsx } from 'clsx';
import { twMerge } from 'tailwind-merge';

export function cn(...inputs: ClassValue[]) {
  return twMerge(clsx(inputs));
}
```

- [ ] **Step 5: `spa/biome.json`**

```json
{
  "$schema": "https://biomejs.dev/schemas/1.8.0/schema.json",
  "organizeImports": { "enabled": true },
  "formatter": {
    "enabled": true,
    "indentStyle": "space",
    "indentWidth": 2,
    "lineWidth": 100
  },
  "linter": {
    "enabled": true,
    "rules": {
      "recommended": true,
      "style": {
        "useImportType": "warn"
      }
    }
  },
  "javascript": {
    "formatter": {
      "quoteStyle": "single",
      "trailingCommas": "all",
      "semicolons": "always"
    }
  },
  "files": {
    "ignore": ["dist", "node_modules", "src/components/ui"]
  }
}
```

`src/components/ui` is ignored because shadcn's scaffolded components have their own conventions; ignoring them keeps churn out of the lint reports.

- [ ] **Step 6: Verify Biome runs**

Run: `cd spa && npm run check`

Expected: passes (no source files to lint yet beyond `cn.ts`, which should be clean). If it fails on formatting, run `npm run check:write` and re-run.

---

### Task 3: shadcn/ui init + add components

**Files:**
- Create: `spa/components.json` (via `npx shadcn init`)
- Create: `spa/src/components/ui/{button,card,skeleton,alert}.tsx` (via `npx shadcn add`)

- [ ] **Step 1: Run shadcn init**

Run: `cd spa && npx --yes shadcn@latest init --yes`

This is interactive in some versions. If the CLI prompts, accept defaults: TypeScript yes, style "default", base color "slate", CSS variables yes, components path `@/components`, utils path `@/lib/cn`, RSC no, alias paths matching the tsconfig.

The CLI may insist on `tailwind.config.ts` instead of `.js` — if it errors on the existing `.js`, rename it to `.ts` and convert the `module.exports`/`export default` accordingly (the contents are identical, just adjust the file extension).

Expected: creates `spa/components.json`. May rewrite `spa/tailwind.config.{js,ts}` and `spa/src/styles/globals.css`. If the rewrites conflict materially with what Task 2 wrote, re-apply the Task 2 versions (shadcn's init is opinionated but the configs here are already shadcn-compatible).

- [ ] **Step 2: Add the four primitives**

Run: `cd spa && npx --yes shadcn@latest add button card skeleton alert --yes`

Expected: creates `spa/src/components/ui/{button,card,skeleton,alert}.tsx`. These are standard shadcn copy-paste components — do NOT edit them; treat them as vendored.

If `npx shadcn` is unavailable or fails, the implementer can hand-write minimal versions following the shadcn source. The four components are small (~30-80 lines each). Document the deviation in the commit message.

- [ ] **Step 3: Verify imports resolve**

Run: `cd spa && npx tsc -b --dry`

Expected: no type errors. If imports fail (e.g. shadcn used a different `@/lib/utils` path), align the import in the shadcn files to `@/lib/cn` to match what Task 2 created.

---

### Task 4: Pure helpers with TDD — `format.ts` and `balances.ts`

**Files:**
- Create: `spa/src/lib/format.ts`, `spa/src/lib/format.test.ts`, `spa/src/lib/balances.ts`, `spa/src/lib/balances.test.ts`
- Modify: `spa/src/lib/types.ts` (create types referenced by the tests)

- [ ] **Step 1: Create `spa/src/lib/types.ts` first** (referenced by tests below)

```ts
export type AccountType = 'A' | 'L' | 'C' | 'R' | 'E';

export interface AccountBalance {
  account_id: number;
  name: string;
  type: AccountType;
  parent_id?: number;
  currency: string;
  amount: number; // int64 cents
  is_hidden: boolean;
}

export interface ListResult<T> {
  items: T[];
  total_count: number;
  limit: number;
  offset: number;
}
```

- [ ] **Step 2: Write failing tests for `format.ts`**

Create `spa/src/lib/format.test.ts`:

```ts
import { describe, expect, test } from 'vitest';
import { formatCents } from './format';

describe('formatCents', () => {
  test('formats positive cents as USD', () => {
    expect(formatCents(125000, 'USD')).toBe('$1,250.00');
  });

  test('formats zero as $0.00 in USD', () => {
    expect(formatCents(0, 'USD')).toBe('$0.00');
  });

  test('formats negative cents with minus prefix', () => {
    expect(formatCents(-42000, 'USD')).toBe('-$420.00');
  });

  test('formats TWD without decimals', () => {
    // TWD uses 0 fraction digits in Intl
    expect(formatCents(125000, 'TWD')).toBe('NT$1,250');
  });

  test('falls back gracefully on unknown currency code', () => {
    // Intl will throw on bogus codes; helper must handle.
    expect(() => formatCents(100, 'ZZZ')).not.toThrow();
  });
});
```

- [ ] **Step 3: Run the failing tests**

Run: `cd spa && npm run test -- format`

Expected: FAIL — `Cannot find module './format'`.

- [ ] **Step 4: Implement `spa/src/lib/format.ts`**

```ts
export function formatCents(cents: number, currency: string): string {
  const value = cents / 100;
  try {
    return new Intl.NumberFormat('en-US', {
      style: 'currency',
      currency,
    }).format(value);
  } catch {
    // Unknown currency code — fall back to plain number with the code.
    return `${value.toFixed(2)} ${currency}`;
  }
}
```

- [ ] **Step 5: Verify the format tests pass**

Run: `cd spa && npm run test -- format`

Expected: all 5 tests PASS.

- [ ] **Step 6: Write failing tests for `balances.ts`**

Create `spa/src/lib/balances.test.ts`:

```ts
import { describe, expect, test } from 'vitest';
import { summarizeBalances } from './balances';
import type { AccountBalance } from './types';

const row = (overrides: Partial<AccountBalance>): AccountBalance => ({
  account_id: 0,
  name: 'X',
  type: 'A',
  currency: 'USD',
  amount: 0,
  is_hidden: false,
  ...overrides,
});

describe('summarizeBalances', () => {
  test('includes Asset and Liability rows in the target currency', () => {
    const rows: AccountBalance[] = [
      row({ account_id: 1, name: 'Assets:Bank', type: 'A', amount: 125000 }),
      row({ account_id: 2, name: 'Assets:Cash', type: 'A', amount: 3500 }),
      row({ account_id: 3, name: 'Liab:Card',  type: 'L', amount: -42000 }),
    ];
    const s = summarizeBalances(rows, 'USD');
    expect(s.assetsTotal).toBe(128500);
    expect(s.liabilitiesTotal).toBe(-42000);
    expect(s.netWorth).toBe(86500);
    expect(s.included).toHaveLength(3);
    expect(s.excluded).toHaveLength(0);
  });

  test('excludes non-asset/liability accounts', () => {
    const rows: AccountBalance[] = [
      row({ account_id: 1, name: 'Assets:Bank', type: 'A', amount: 100 }),
      row({ account_id: 2, name: 'Income:Salary', type: 'R', amount: -50000 }),
      row({ account_id: 3, name: 'Expenses:Food', type: 'E', amount: 25000 }),
      row({ account_id: 4, name: 'Equity:Opening', type: 'C', amount: 1000 }),
    ];
    const s = summarizeBalances(rows, 'USD');
    expect(s.included).toHaveLength(1);
    expect(s.excluded).toHaveLength(3);
    expect(s.assetsTotal).toBe(100);
    expect(s.liabilitiesTotal).toBe(0);
    expect(s.netWorth).toBe(100);
  });

  test('excludes accounts in a different currency', () => {
    const rows: AccountBalance[] = [
      row({ account_id: 1, name: 'Assets:USDBank', type: 'A', currency: 'USD', amount: 1000 }),
      row({ account_id: 2, name: 'Assets:TWDBank', type: 'A', currency: 'TWD', amount: 999999 }),
    ];
    const s = summarizeBalances(rows, 'USD');
    expect(s.included).toHaveLength(1);
    expect(s.excluded).toHaveLength(1);
    expect(s.excluded[0].account_id).toBe(2);
    expect(s.assetsTotal).toBe(1000);
  });

  test('returns zeros for empty input', () => {
    const s = summarizeBalances([], 'USD');
    expect(s.assetsTotal).toBe(0);
    expect(s.liabilitiesTotal).toBe(0);
    expect(s.netWorth).toBe(0);
    expect(s.included).toEqual([]);
    expect(s.excluded).toEqual([]);
  });
});
```

- [ ] **Step 7: Run the failing balances tests**

Run: `cd spa && npm run test -- balances`

Expected: FAIL — `Cannot find module './balances'`.

- [ ] **Step 8: Implement `spa/src/lib/balances.ts`**

```ts
import type { AccountBalance } from './types';

export interface BalancesSummary {
  assetsTotal: number;
  liabilitiesTotal: number;
  netWorth: number;
  included: AccountBalance[];
  excluded: AccountBalance[];
}

export function summarizeBalances(
  rows: AccountBalance[],
  summaryCurrency: string,
): BalancesSummary {
  const included: AccountBalance[] = [];
  const excluded: AccountBalance[] = [];
  let assetsTotal = 0;
  let liabilitiesTotal = 0;

  for (const row of rows) {
    const isAssetOrLiability = row.type === 'A' || row.type === 'L';
    const matchesCurrency = row.currency === summaryCurrency;
    if (!isAssetOrLiability || !matchesCurrency) {
      excluded.push(row);
      continue;
    }
    included.push(row);
    if (row.type === 'A') {
      assetsTotal += row.amount;
    } else {
      liabilitiesTotal += row.amount;
    }
  }

  return {
    assetsTotal,
    liabilitiesTotal,
    netWorth: assetsTotal + liabilitiesTotal,
    included,
    excluded,
  };
}
```

- [ ] **Step 9: Verify all unit tests pass**

Run: `cd spa && npm run test`

Expected: 9 tests PASS (5 format + 4 balances).

---

### Task 5: API client

**Files:**
- Create: `spa/src/lib/api.ts`

- [ ] **Step 1: Create `spa/src/lib/api.ts`**

```ts
import type { AccountBalance, ListResult } from './types';

export class ApiError extends Error {
  readonly status: number;
  readonly field?: string;
  constructor(status: number, message: string, field?: string) {
    super(message);
    this.name = 'ApiError';
    this.status = status;
    this.field = field;
  }
}

interface ApiErrorBody {
  error?: string;
  message?: string;
  field?: string;
}

async function apiFetch<T>(path: string, init?: RequestInit): Promise<T> {
  const resp = await fetch(path, init);
  if (!resp.ok) {
    let body: ApiErrorBody = {};
    try {
      body = (await resp.json()) as ApiErrorBody;
    } catch {
      // body wasn't JSON; fall through with empty body
    }
    throw new ApiError(resp.status, body.message ?? resp.statusText, body.field);
  }
  return (await resp.json()) as T;
}

export function getBalances(): Promise<ListResult<AccountBalance>> {
  return apiFetch<ListResult<AccountBalance>>('/api/balances');
}
```

- [ ] **Step 2: Verify it compiles**

Run: `cd spa && npx tsc -b --dry`

Expected: no errors. (No tests for this file — exercised end-to-end by the smoke test in Task 9.)

---

### Task 6: Routing scaffold — root layout, redirect, balances skeleton

**Files:**
- Create: `spa/src/main.tsx`, `spa/src/routes/__root.tsx`, `spa/src/routes/index.tsx`, `spa/src/routes/balances.tsx` (skeleton)

The TanStack Router Vite plugin (added in Task 1's vite.config) auto-generates `spa/src/routeTree.gen.ts` from the files in `routes/` on dev/build. The implementer must NOT hand-edit that file.

- [ ] **Step 1: Create `spa/src/routes/__root.tsx`** — persistent layout shell

```tsx
import { Outlet, createRootRoute } from '@tanstack/react-router';
import { Sidebar } from '@/components/Sidebar';

export const Route = createRootRoute({
  component: RootLayout,
});

function RootLayout() {
  return (
    <div className="flex min-h-screen">
      <Sidebar />
      <main className="flex-1 p-8">
        <Outlet />
      </main>
    </div>
  );
}
```

`@/components/Sidebar` doesn't exist yet — Task 7 creates it. TypeScript will error until then; that's expected.

- [ ] **Step 2: Create `spa/src/routes/index.tsx`** — redirect

```tsx
import { createFileRoute, redirect } from '@tanstack/react-router';

export const Route = createFileRoute('/')({
  beforeLoad: () => {
    throw redirect({ to: '/balances' });
  },
});
```

- [ ] **Step 3: Create `spa/src/routes/balances.tsx`** — skeleton (Task 8 fully wires it)

```tsx
import { createFileRoute } from '@tanstack/react-router';

export const Route = createFileRoute('/balances')({
  component: BalancesPage,
});

function BalancesPage() {
  return <div>Balances (placeholder)</div>;
}
```

- [ ] **Step 4: Create `spa/src/main.tsx`** — React root + providers

```tsx
import './styles/globals.css';
import { StrictMode } from 'react';
import { createRoot } from 'react-dom/client';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { RouterProvider, createRouter } from '@tanstack/react-router';
import { routeTree } from './routeTree.gen';

const queryClient = new QueryClient({
  defaultOptions: {
    queries: { staleTime: 30_000, refetchOnWindowFocus: false },
  },
});

const router = createRouter({ routeTree });

declare module '@tanstack/react-router' {
  interface Register {
    router: typeof router;
  }
}

const rootEl = document.getElementById('root');
if (!rootEl) throw new Error('root element not found');

createRoot(rootEl).render(
  <StrictMode>
    <QueryClientProvider client={queryClient}>
      <RouterProvider router={router} />
    </QueryClientProvider>
  </StrictMode>,
);
```

`routeTree.gen.ts` is generated by the TanStack Router Vite plugin on first dev/build. If the import is unresolved at typecheck time, run a quick `npm run dev` (then Ctrl-C) to trigger generation.

- [ ] **Step 5: Build the SPA** — generates routeTree, verifies typecheck

Run: `cd spa && npm run build`

Expected: build fails because `@/components/Sidebar` doesn't exist yet. That's fine for now; Tasks 7/8 will fix it. Confirm the failure is specifically about `Sidebar` import, NOT about routing config — if there's a routing-side error, fix that first.

---

### Task 7: Sidebar component

**Files:**
- Create: `spa/src/components/Sidebar.tsx`

- [ ] **Step 1: Create `spa/src/components/Sidebar.tsx`**

```tsx
import { Link, useRouterState } from '@tanstack/react-router';
import { cn } from '@/lib/cn';

interface NavItem {
  label: string;
  to?: string; // undefined = disabled stub
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
    <nav
      aria-label="Main navigation"
      className="w-56 shrink-0 border-r bg-muted/30 p-4"
    >
      <div className="mb-6 text-lg font-semibold tracking-tight">kea</div>
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
                  isActive
                    ? 'bg-primary text-primary-foreground font-medium'
                    : 'hover:bg-muted',
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

- [ ] **Step 2: Build again to confirm `Sidebar` imports resolve**

Run: `cd spa && npm run build`

Expected: still might fail on missing exports from other files (Task 8 hasn't built the dashboard components yet), but the `Sidebar` import should no longer be the blocker. If it is, fix the path or export.

---

### Task 8: Dashboard components and route wiring

**Files:**
- Create: `spa/src/components/NetWorthCard.tsx`, `spa/src/components/TypeTotalCard.tsx`, `spa/src/components/AccountListRow.tsx`
- Modify: `spa/src/routes/balances.tsx` (full implementation)

- [ ] **Step 1: Create `spa/src/components/NetWorthCard.tsx`**

```tsx
import { Card, CardContent } from '@/components/ui/card';
import { formatCents } from '@/lib/format';

interface Props {
  netWorth: number;
  assetsTotal: number;
  liabilitiesTotal: number;
  currency: string;
  excludedCount: number;
}

export function NetWorthCard({
  netWorth,
  assetsTotal,
  liabilitiesTotal,
  currency,
  excludedCount,
}: Props) {
  return (
    <Card className="mb-6 bg-gradient-to-br from-blue-700 to-blue-500 text-white">
      <CardContent className="p-6">
        <div className="text-xs uppercase tracking-wider opacity-80">Net Worth</div>
        <div className="mt-1 text-4xl font-extrabold tabular-nums">
          {formatCents(netWorth, currency)}
        </div>
        <div className="mt-2 text-sm opacity-85">
          {formatCents(assetsTotal, currency)} assets −{' '}
          {formatCents(-liabilitiesTotal, currency)} liabilities
        </div>
        {excludedCount > 0 && (
          <div className="mt-3 text-xs opacity-80">
            {excludedCount} account{excludedCount === 1 ? '' : 's'} in other currencies not
            included
          </div>
        )}
      </CardContent>
    </Card>
  );
}
```

- [ ] **Step 2: Create `spa/src/components/TypeTotalCard.tsx`**

```tsx
import { Card, CardContent } from '@/components/ui/card';
import { cn } from '@/lib/cn';
import { formatCents } from '@/lib/format';

interface Props {
  label: string;
  amount: number;
  currency: string;
  negative?: boolean;
}

export function TypeTotalCard({ label, amount, currency, negative }: Props) {
  return (
    <Card>
      <CardContent className="p-4">
        <div className="text-xs uppercase tracking-wider text-muted-foreground">
          {label}
        </div>
        <div
          className={cn(
            'mt-1 text-2xl font-bold tabular-nums',
            negative ? 'text-destructive' : 'text-foreground',
          )}
        >
          {formatCents(amount, currency)}
        </div>
      </CardContent>
    </Card>
  );
}
```

- [ ] **Step 3: Create `spa/src/components/AccountListRow.tsx`**

```tsx
import { cn } from '@/lib/cn';
import { formatCents } from '@/lib/format';
import type { AccountBalance } from '@/lib/types';

interface Props {
  row: AccountBalance;
}

export function AccountListRow({ row }: Props) {
  const negative = row.amount < 0;
  return (
    <div className="flex items-center justify-between border-b border-border/60 px-2 py-2 text-sm">
      <span>{row.name}</span>
      <span className={cn('tabular-nums', negative && 'text-destructive')}>
        {formatCents(row.amount, row.currency)}
      </span>
    </div>
  );
}
```

- [ ] **Step 4: Replace `spa/src/routes/balances.tsx` with the full implementation**

```tsx
import { createFileRoute } from '@tanstack/react-router';
import { useQuery } from '@tanstack/react-query';
import { Alert, AlertDescription, AlertTitle } from '@/components/ui/alert';
import { Button } from '@/components/ui/button';
import { Skeleton } from '@/components/ui/skeleton';
import { AccountListRow } from '@/components/AccountListRow';
import { NetWorthCard } from '@/components/NetWorthCard';
import { TypeTotalCard } from '@/components/TypeTotalCard';
import { getBalances } from '@/lib/api';
import { summarizeBalances } from '@/lib/balances';

const DEFAULT_CURRENCY = (import.meta.env.VITE_DEFAULT_CURRENCY as string) || 'USD';

export const Route = createFileRoute('/balances')({
  component: BalancesPage,
});

function BalancesPage() {
  const query = useQuery({ queryKey: ['balances'], queryFn: getBalances });

  if (query.isPending) {
    return (
      <div>
        <Skeleton className="mb-6 h-32 w-full" />
        <div className="mb-6 grid grid-cols-2 gap-4">
          <Skeleton className="h-20 w-full" />
          <Skeleton className="h-20 w-full" />
        </div>
        <div className="space-y-2">
          {[0, 1, 2, 3, 4, 5].map((i) => (
            <Skeleton key={i} className="h-8 w-full" />
          ))}
        </div>
      </div>
    );
  }

  if (query.isError) {
    return (
      <Alert variant="destructive">
        <AlertTitle>Failed to load balances</AlertTitle>
        <AlertDescription className="mt-2 space-y-3">
          <div>{query.error instanceof Error ? query.error.message : 'Unknown error'}</div>
          <Button onClick={() => query.refetch()} size="sm">
            Retry
          </Button>
        </AlertDescription>
      </Alert>
    );
  }

  const rows = query.data.items;

  if (rows.length === 0) {
    return (
      <div className="flex min-h-[40vh] items-center justify-center">
        <p className="max-w-md text-center text-sm text-muted-foreground">
          No accounts yet — run <code className="font-mono">kea ledger add</code> then
          create one via the CLI.
        </p>
      </div>
    );
  }

  const summary = summarizeBalances(rows, DEFAULT_CURRENCY);

  return (
    <div>
      <NetWorthCard
        netWorth={summary.netWorth}
        assetsTotal={summary.assetsTotal}
        liabilitiesTotal={summary.liabilitiesTotal}
        currency={DEFAULT_CURRENCY}
        excludedCount={summary.excluded.length}
      />

      <div className="mb-6 grid grid-cols-2 gap-4">
        <TypeTotalCard
          label="Assets"
          amount={summary.assetsTotal}
          currency={DEFAULT_CURRENCY}
        />
        <TypeTotalCard
          label="Liabilities"
          amount={summary.liabilitiesTotal}
          currency={DEFAULT_CURRENCY}
          negative
        />
      </div>

      <div className="rounded-md border bg-card">
        {summary.included.map((row) => (
          <AccountListRow key={row.account_id} row={row} />
        ))}
      </div>
    </div>
  );
}
```

- [ ] **Step 5: Build to verify everything typechecks**

Run: `cd spa && npm run build`

Expected: build succeeds. Output goes to `spa/dist/`. If shadcn's `Alert` exports differ (`Alert`, `AlertTitle`, `AlertDescription`) — adapt imports to whatever shadcn generated.

If TanStack Router complains about route registration, run `npm run dev` briefly to regenerate `routeTree.gen.ts`, then re-build.

---

### Task 9: Smoke test

**Files:**
- Create: `spa/src/test/setup.ts`, `spa/src/test/balances.test.tsx`

- [ ] **Step 1: Create `spa/src/test/setup.ts`** — test infrastructure + `TestApp` helper

```ts
import '@testing-library/jest-dom/vitest';
import { afterEach } from 'vitest';
import { cleanup } from '@testing-library/react';

afterEach(() => {
  cleanup();
});

// Re-export a TestApp helper for routing-aware tests.
import {
  RouterProvider,
  createMemoryHistory,
  createRouter,
} from '@tanstack/react-router';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { routeTree } from '../routeTree.gen';

export function makeTestApp(initialPath: string) {
  const history = createMemoryHistory({ initialEntries: [initialPath] });
  const router = createRouter({ routeTree, history });
  const queryClient = new QueryClient({
    defaultOptions: { queries: { retry: false, gcTime: 0 } },
  });
  return (
    <QueryClientProvider client={queryClient}>
      <RouterProvider router={router} />
    </QueryClientProvider>
  );
}
```

This file is `.ts` not `.tsx`, but it returns JSX. Rename to `setup.tsx` if Vitest complains; otherwise leave as `.ts` (Vite handles `.ts` with JSX since the tsconfig sets `jsx: react-jsx`).

If TypeScript errors on the JSX-in-`.ts` regardless, split: keep `setup.ts` for `afterEach`/`cleanup`, and put `makeTestApp` in `src/test/TestApp.tsx`. Update the import in the smoke test accordingly.

- [ ] **Step 2: Create `spa/src/test/balances.test.tsx`** — the one smoke test

```tsx
import { render, screen } from '@testing-library/react';
import { afterEach, beforeEach, expect, test, vi } from 'vitest';
import { makeTestApp } from './setup';

const okResponse = (body: unknown) =>
  new Response(JSON.stringify(body), {
    status: 200,
    headers: { 'Content-Type': 'application/json' },
  });

beforeEach(() => {
  vi.stubGlobal(
    'fetch',
    vi.fn(() =>
      okResponse({
        items: [
          {
            account_id: 1,
            name: 'Assets:Bank',
            type: 'A',
            currency: 'USD',
            amount: 125000,
            is_hidden: false,
          },
          {
            account_id: 2,
            name: 'Assets:Cash',
            type: 'A',
            currency: 'USD',
            amount: 3500,
            is_hidden: false,
          },
          {
            account_id: 3,
            name: 'Liab:Card',
            type: 'L',
            currency: 'USD',
            amount: -42000,
            is_hidden: false,
          },
        ],
        total_count: 3,
        limit: 0,
        offset: 0,
      }),
    ),
  );
});

afterEach(() => {
  vi.unstubAllGlobals();
});

test('renders Net Worth headline from a balances response', async () => {
  render(makeTestApp('/balances'));

  // Net Worth headline appears.
  expect(await screen.findByText(/Net Worth/i)).toBeInTheDocument();

  // Math: 125000 + 3500 + (-42000) = 86500 cents = $865.00
  expect(await screen.findByText('$865.00')).toBeInTheDocument();
});
```

- [ ] **Step 3: Run the smoke test**

Run: `cd spa && npm run test`

Expected: 10 tests PASS (9 unit + 1 smoke). If the smoke test fails because TanStack Router can't resolve the route, the `routeTree.gen.ts` is out of date — run `npm run build` once, then re-run `npm run test`.

---

### Task 10: Repo-level integration — Makefile, top-level `.gitignore`, `spa/README.md`

**Files:**
- Modify: `Makefile` (repo root)
- Modify: `.gitignore` (repo root)
- Create: `spa/README.md`

- [ ] **Step 1: Read the current `Makefile`**

Run: `cat Makefile`

The current Makefile (per prior PRs) has `build`, `run`, and `test-race` targets, each with tab-indented recipes.

- [ ] **Step 2: Append 3 SPA targets to `Makefile`**

After the existing `test-race` target, append:

```makefile

spa-install:
	cd spa && npm install

spa-dev:
	cd spa && npm run dev

spa-build:
	cd spa && npm run build
```

Use literal tab characters (not spaces) for the recipe lines. Preserve the trailing newline from PR #181.

- [ ] **Step 3: Update repo-root `.gitignore`**

Read the current file: `cat .gitignore`

Append these lines (after the existing entries):

```
# SPA workspace
spa/node_modules/
spa/dist/
spa/.env
spa/.env.*
!spa/.env.example
```

- [ ] **Step 4: Create `spa/README.md`**

```markdown
# kea SPA

React SPA paired with `kea serve`. Currently ships one route — `/balances` — backed by `GET /api/balances`.

## Stack

- Vite + React + TypeScript
- TanStack Router (file-based routes)
- TanStack Query (data fetching)
- Tailwind + shadcn/ui (styling and primitives)
- Biome (lint + format)
- Vitest + @testing-library/react (tests)

## First-time setup

```bash
cd spa
npm install
```

## Dev workflow

Two terminals:

```bash
# Terminal 1 — Go API on :8080
make run

# Terminal 2 — SPA dev server on :5173
cd spa && npm run dev
# or: make spa-dev
```

Open <http://localhost:5173>. Vite proxies `/api/**` to `http://localhost:8080`.

## Scripts

- `npm run dev` — Vite dev server on :5173
- `npm run build` — Production build to `spa/dist/`
- `npm run preview` — Preview the production build
- `npm run test` — Run all tests once (Vitest)
- `npm run test:watch` — Watch mode
- `npm run check` — Biome lint + format check
- `npm run check:write` — Apply Biome fixes

## Configuration

`VITE_DEFAULT_CURRENCY` — currency used for the Net Worth headline. Defaults to `USD` if unset. Copy `.env.example` to `.env` to override.

## Status

- ✅ `/balances` — Net Worth dashboard
- 🚧 `/accounts`, `/transactions`, `/reports`, `/reconcile` — sidebar stubs (disabled)
- 🚧 Embed via `go:embed` for single-binary distribution
- 🚧 `GET /api/config` endpoint for server-side default currency
```

- [ ] **Step 5: Verify Makefile targets work**

Run: `make spa-install` (should be a no-op since `npm install` was already run in Task 1; verify it doesn't error). Then `make spa-build` (should produce `spa/dist/`).

Expected: both succeed.

---

### Task 11: Full verification and commit

**Files:** none — verification and commit only.

- [ ] **Step 1: Run all SPA checks**

```
cd spa
npm run check
npm run test
npm run build
cd ..
```

Expected: all three green.

- [ ] **Step 2: Confirm Go still builds and tests**

```
go test ./...
go build ./...
```

Expected: green. (No Go source changes in this PR; only Makefile and `.gitignore` touched at repo root.)

- [ ] **Step 3: Manual smoke test (optional but recommended)**

In one terminal: `make run`.
In another: `make spa-dev`.
Browse to `http://localhost:5173`. Expect:
- Redirect from `/` to `/balances`.
- Sidebar with "kea" header and 5 nav items; "Balances" is active and highlighted; the other four are visibly disabled.
- Either the Net Worth headline + cards + account list, OR the empty-state message ("No accounts yet…") if the seeded ledger is fresh, OR an error alert if `kea serve` is not running.

Document any rendering issues but do not block the commit on them — they can be addressed in follow-up PRs.

- [ ] **Step 4: Inspect `git status`**

Expected new files (under `spa/`): the entire directory tree per the File Map, except `node_modules/` and `dist/` (gitignored).

Expected modifications at repo root:
- `.gitignore` (the new `spa/...` block)
- `Makefile` (the 3 new targets)

Stage everything:

```bash
git add spa/ Makefile .gitignore
```

Verify `git status` shows exactly those paths staged. If `package-lock.json` is staged from `spa/`, that's expected — commit it. If `node_modules/` somehow shows up staged, unstage it: `git restore --staged spa/node_modules`.

- [ ] **Step 5: Commit**

```
git commit -m "$(cat <<'EOF'
feat(spa): vertical-slice SPA at spa/ with Balances dashboard

Stands up the React SPA workspace (Vite + React + TypeScript + TanStack
Router + TanStack Query + Tailwind + shadcn/ui + Biome + Vitest) at spa/
and ships one working route — /balances — backed by GET /api/balances.

The Balances route renders a Net Worth headline, Assets/Liabilities totals,
and a per-account list. Aggregation is client-side (single pass) over the
endpoint's flat list. Multi-currency is single-currency-only with a
"N accounts in other currencies not included" footnote.

The sidebar exposes 5 nav entries; only Balances is wired up. The other
four (Accounts, Transactions, Reports, Reconcile) render as disabled stubs
ready to be enabled as their routes ship.

Dev workflow: \`make run\` + \`cd spa && npm run dev\` in two terminals; Vite
proxies /api/** to localhost:8080. \`npm run check\` / \`npm run test\` /
\`npm run build\` all green. Single Vitest smoke test pins the aggregation
math and proves the slice mounts without crashing.

Out of scope: go:embed integration, the other 4 routes, /api/config,
multi-currency net worth, codegen of TypeScript types from Go JSON tags.
EOF
)"
```

This project does not use a `Co-Authored-By` footer. Do not add one.

Expected: commit succeeds. `git log -1 --stat` shows the new spa/ tree plus the two modified root files.

---

## Self-Review

**Spec coverage:**

- Spec §Tech stack table: Task 1 (package.json + tsconfig), Task 2 (Tailwind/Biome), Task 3 (shadcn/ui) ✓
- Spec §Location (`spa/` at repo root): Task 1 ✓
- Spec §File structure: Tasks 1-10 collectively create every file ✓
- Spec §Data shape (TypeScript types): Task 4 Step 1 ✓
- Spec §Aggregation (`summarizeBalances` signature + semantics): Task 4 ✓
- Spec §Default currency (`VITE_DEFAULT_CURRENCY` env): Task 1 Step 7 + Task 8 Step 4 ✓
- Spec §Routing: Tasks 6-7 ✓
- Spec §Components: Tasks 7-8 ✓
- Spec §Loading & error states: Task 8 Step 4 ✓
- Spec §API client: Task 5 ✓
- Spec §Dev workflow / Makefile / README: Task 10 ✓
- Spec §Tests (smoke + unit on pure functions): Tasks 4 and 9 ✓
- Spec §Lint / format / package.json scripts: Task 1 + Task 2 ✓
- Spec §Verification: Task 11 ✓
- Spec §Commit shape (single commit, scope `feat(spa)`, body verbatim): Task 11 Step 5 ✓
- Spec §Out of scope: no tasks attempt go:embed, /api/config, other routes, multi-currency, codegen, auth ✓

**Placeholder scan:**

- No "TBD", "implement later", "similar to Task N".
- Every code step shows verbatim content for the file being created/modified.
- The shadcn CLI step (Task 3) notes that the CLI may rewrite Task 2 files and gives concrete instructions for re-applying — that's investigation guidance, not a placeholder.
- The `setup.ts` vs `setup.tsx` decision in Task 9 is acknowledged as version-dependent; the fallback path is specified.

**Type/name consistency:**

- `AccountBalance` / `ListResult<T>` field names match across types.ts (Task 4), api.ts (Task 5), balances.ts (Task 4), and balances.test.tsx (Task 9).
- `summarizeBalances` signature `(rows, summaryCurrency) → BalancesSummary` consistent between tests (Task 4 Step 6), implementation (Task 4 Step 8), and call site (Task 8 Step 4).
- `formatCents(cents, currency)` signature consistent across format.test.ts, format.ts, NetWorthCard, TypeTotalCard, AccountListRow.
- `BalancesSummary` field names (`assetsTotal`, `liabilitiesTotal`, `netWorth`, `included`, `excluded`) consistent across definition, tests, and `BalancesPage` consumption.
- Route paths (`/balances`) consistent across Sidebar, index.tsx redirect, balances.tsx, smoke test initial path.
- Smoke test math: 125000 + 3500 + (-42000) = 86500 cents → `$865.00` — matches Task 9 Step 2 assertion.
