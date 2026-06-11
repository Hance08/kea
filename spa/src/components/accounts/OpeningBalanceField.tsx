import { Input } from '@/components/ui/input';
import { Label } from '@/components/ui/label';

interface Props {
  enabled: boolean;
  onEnabledChange: (next: boolean) => void;
  amount: string;
  onAmountChange: (v: string) => void;
  currency: string;
  error?: string;
}

export function OpeningBalanceField({
  enabled,
  onEnabledChange,
  amount,
  onAmountChange,
  currency,
  error,
}: Props) {
  return (
    <div className="rounded-md border bg-muted/20 p-3">
      <label className="flex items-center gap-2 text-sm">
        <input
          type="checkbox"
          checked={enabled}
          onChange={(e) => onEnabledChange(e.target.checked)}
        />
        Set opening balance
      </label>
      {enabled && (
        <div className="mt-3 space-y-1">
          <Label htmlFor="opening-amount">Amount ({currency})</Label>
          <Input
            id="opening-amount"
            inputMode="decimal"
            value={amount}
            onChange={(e) => onAmountChange(e.target.value)}
            placeholder="0.00"
            aria-invalid={error ? true : undefined}
          />
          <p className="text-xs text-muted-foreground">
            Posts an opening transaction against Equity:OpeningBalances_{currency}.
          </p>
          {error && <p className="text-xs text-destructive">{error}</p>}
        </div>
      )}
    </div>
  );
}
