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
              search={{ include_hidden: false, show_parents: false, limit: 10, offset: 0 }}
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
