import { Button } from '@/components/ui/button';

interface Props {
  label: string; // "As of" or "At"
  value: number | undefined; // Unix seconds; undefined = now
  onChange: (next: number | undefined) => void;
}

function unixToInputDate(unix: number): string {
  const d = new Date(unix * 1000);
  const y = d.getUTCFullYear();
  const m = String(d.getUTCMonth() + 1).padStart(2, '0');
  const day = String(d.getUTCDate()).padStart(2, '0');
  return `${y}-${m}-${day}`;
}

function inputDateToUnix(s: string): number | undefined {
  if (!/^\d{4}-\d{2}-\d{2}$/.test(s)) return undefined;
  const [y, m, d] = s.split('-').map(Number);
  // Treat the picked date as end-of-day UTC so the snapshot includes the full day.
  return Math.floor(Date.UTC(y, m - 1, d, 23, 59, 59) / 1000);
}

export function AsOfPicker({ label, value, onChange }: Props) {
  return (
    <div className="mb-4 flex items-center gap-2 text-sm">
      <label className="flex items-center gap-2">
        <span className="font-medium">{label}</span>
        <input
          type="date"
          aria-label={label}
          className="rounded border bg-background px-2 py-1"
          value={value === undefined ? '' : unixToInputDate(value)}
          onChange={(e) => onChange(inputDateToUnix(e.target.value))}
        />
      </label>
      {value === undefined ? (
        <span className="text-muted-foreground">Now</span>
      ) : (
        <Button size="sm" variant="ghost" onClick={() => onChange(undefined)}>
          Now
        </Button>
      )}
    </div>
  );
}
