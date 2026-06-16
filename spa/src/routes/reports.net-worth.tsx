import { AsOfPicker } from '@/components/reports/AsOfPicker';
import { CurrencyFooter } from '@/components/reports/CurrencyFooter';
import { Alert, AlertDescription, AlertTitle } from '@/components/ui/alert';
import { Button } from '@/components/ui/button';
import { Skeleton } from '@/components/ui/skeleton';
import { formatCents } from '@/lib/format';
import { useNetWorth } from '@/lib/hooks/useReport';
import { type AtSearchParams, parseAtSearch } from '@/lib/reports-search-params';
import { useServerConfig } from '@/lib/server-config';
import { Link, createFileRoute, useNavigate } from '@tanstack/react-router';

export const Route = createFileRoute('/reports/net-worth')({
  validateSearch: (s): AtSearchParams => parseAtSearch(s),
  component: NetWorthPage,
});

function NetWorthPage() {
  const { defaults } = useServerConfig();
  const currency = defaults.currency;
  const search = Route.useSearch();
  const navigate = useNavigate({ from: '/reports/net-worth' });
  const query = useNetWorth(search);

  const setAt = (v: number | undefined) =>
    navigate({ search: () => (v === undefined ? {} : { at: v }) });

  if (query.isPending) {
    return (
      <div className="space-y-4">
        <AsOfPicker label="At" value={search.at} onChange={setAt} />
        <Skeleton className="h-24" />
      </div>
    );
  }
  if (query.isError) {
    return (
      <div className="space-y-3">
        <AsOfPicker label="At" value={search.at} onChange={setAt} />
        <Alert variant="destructive">
          <AlertTitle>Failed to load net worth</AlertTitle>
          <AlertDescription className="mt-2 space-y-3">
            <div>{query.error instanceof Error ? query.error.message : 'Unknown error'}</div>
            <Button onClick={() => query.refetch()} size="sm">
              Retry
            </Button>
          </AlertDescription>
        </Alert>
      </div>
    );
  }

  const nw = query.data.net_worth[currency] ?? 0;
  const otherCurrencies = Object.keys(query.data.net_worth).filter((c) => c !== currency);

  return (
    <div className="space-y-6">
      <AsOfPicker label="At" value={search.at} onChange={setAt} />
      <section>
        <div className="text-xs uppercase tracking-wide text-muted-foreground">Net Worth</div>
        <div className="mt-1 text-4xl font-semibold">{formatCents(nw, currency)}</div>
        {Object.keys(query.data.net_worth).length === 0 && (
          <p className="mt-3 text-sm text-muted-foreground">
            No accounts yet —{' '}
            <Link
              to="/accounts/new"
              className="underline hover:text-foreground"
              search={{ include_hidden: false, show_parents: false, limit: 10, offset: 0 }}
            >
              create one
            </Link>
            .
          </p>
        )}
      </section>
      {otherCurrencies.length > 0 && (
        <CurrencyFooter
          defaultCurrency={currency}
          entries={[{ label: 'Net worth', byCurrency: query.data.net_worth }]}
        />
      )}
    </div>
  );
}
