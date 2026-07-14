import { Input } from '@/components/ui/input';
import { Label } from '@/components/ui/label';
import { cn } from '@/lib/cn';
import { useAmountFormat } from '@/lib/server-config';

interface Props {
  accountName: string;
  currency: string;
  unreconciledCount: number;
  statement: string;
  onStatementChange: (next: string) => void;
  statementInvalid: boolean;
  lastReconciledCents: number;
  clearedCents: number;
  differenceCents: number | null;
}

export function ReconcileHeader({
  accountName,
  currency,
  unreconciledCount,
  statement,
  onStatementChange,
  statementInvalid,
  lastReconciledCents,
  clearedCents,
  differenceCents,
}: Props) {
  const { formatBalanceAbs } = useAmountFormat();

  let diffLabel = '—';
  let diffClass = 'text-muted-foreground';
  if (differenceCents === 0) {
    diffLabel = 'Balanced';
    diffClass = 'text-green-600';
  } else if (differenceCents !== null && differenceCents > 0) {
    diffLabel = `Short ${formatBalanceAbs(differenceCents)}`;
    diffClass = 'text-red-600';
  } else if (differenceCents !== null && differenceCents < 0) {
    diffLabel = `Over ${formatBalanceAbs(differenceCents)}`;
    diffClass = 'text-amber-600';
  }

  return (
    <div className="mb-4 rounded-md border bg-card p-4">
      <div className="mb-3 flex items-baseline justify-between gap-3">
        <h1 className="text-lg font-semibold">{accountName}</h1>
        <div className="text-sm text-muted-foreground">
          {currency} · {unreconciledCount} unreconciled
        </div>
      </div>
      <div className="grid grid-cols-1 gap-4 sm:grid-cols-4">
        <div>
          <Label htmlFor="reconcile-statement" className="text-xs uppercase text-muted-foreground">
            Statement
          </Label>
          <Input
            id="reconcile-statement"
            inputMode="decimal"
            value={statement}
            onChange={(e) => onStatementChange(e.target.value)}
            aria-invalid={statementInvalid || undefined}
            placeholder="0.00"
          />
        </div>
        <div>
          <div className="text-xs uppercase text-muted-foreground">Last reconciled</div>
          <div className="pt-2 tabular-nums">{formatBalanceAbs(lastReconciledCents)}</div>
        </div>
        <div>
          <div className="text-xs uppercase text-muted-foreground">Cleared</div>
          <div className="pt-2 tabular-nums">{formatBalanceAbs(clearedCents)}</div>
        </div>
        <div>
          <div className="text-xs uppercase text-muted-foreground">Difference</div>
          <div className={cn('pt-2 tabular-nums font-medium', diffClass)}>{diffLabel}</div>
        </div>
      </div>
    </div>
  );
}
