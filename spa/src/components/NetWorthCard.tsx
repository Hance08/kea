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
          {formatCents(assetsTotal, currency)} assets − {formatCents(-liabilitiesTotal, currency)}{' '}
          liabilities
        </div>
        {excludedCount > 0 && (
          <div className="mt-3 text-xs opacity-80">
            {excludedCount} account{excludedCount === 1 ? '' : 's'} in other currencies not included
          </div>
        )}
      </CardContent>
    </Card>
  );
}
