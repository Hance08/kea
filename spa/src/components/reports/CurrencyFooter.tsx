import { formatCents } from '@/lib/format';

export interface CurrencyEntry {
  label: string;
  byCurrency: Record<string, number>;
}

interface Props {
  defaultCurrency: string;
  entries: CurrencyEntry[];
}

export function CurrencyFooter({ defaultCurrency, entries }: Props) {
  const lines = entries
    .map((entry) => {
      const others = Object.entries(entry.byCurrency).filter(([cur]) => cur !== defaultCurrency);
      if (others.length === 0) return null;
      const parts = others.map(([cur, amt]) => formatCents(amt, cur)).join(', ');
      return { label: entry.label, parts };
    })
    .filter((x): x is { label: string; parts: string } => x !== null);

  if (lines.length === 0) return null;

  return (
    <div className="mt-3 text-xs text-muted-foreground">
      <div className="font-medium">Also in this report:</div>
      {lines.map((line) => (
        <div key={line.label} data-testid="cur-footer-line">
          {line.label}: {line.parts}
        </div>
      ))}
    </div>
  );
}
