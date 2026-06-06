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
        <div className="text-xs uppercase tracking-wider text-muted-foreground">{label}</div>
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
