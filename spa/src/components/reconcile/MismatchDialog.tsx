import { Button } from '@/components/ui/button';
import { useAmountFormat } from '@/lib/server-config';
import { useEffect } from 'react';

interface Props {
  open: boolean;
  differenceCents: number;
  pending: boolean;
  onCancel: () => void;
  onConfirm: () => void;
}

export function MismatchDialog({ open, differenceCents, pending, onCancel, onConfirm }: Props) {
  const { formatBalanceAbs } = useAmountFormat();

  useEffect(() => {
    if (!open) return;
    const onKey = (e: KeyboardEvent) => {
      if (e.key === 'Escape') onCancel();
    };
    document.addEventListener('keydown', onKey);
    return () => document.removeEventListener('keydown', onKey);
  }, [open, onCancel]);

  if (!open) return null;

  const direction = differenceCents > 0 ? 'short' : 'over';
  const amount = formatBalanceAbs(differenceCents);

  return (
    <div
      className="fixed inset-0 z-50 flex items-center justify-center bg-black/50 p-4"
      onClick={(e) => {
        if (e.target === e.currentTarget) onCancel();
      }}
    >
      <div
        role="alertdialog"
        aria-labelledby="mismatch-title"
        aria-describedby="mismatch-desc"
        className="w-full max-w-sm rounded-md border bg-card p-5 shadow-lg"
      >
        <h2 id="mismatch-title" className="text-base font-semibold">
          Statement balance mismatch
        </h2>
        <p id="mismatch-desc" className="mt-2 text-sm text-muted-foreground">
          Statement balance is{' '}
          <span className="font-medium">
            {direction} {amount}
          </span>
          . Reconcile anyway?
        </p>
        <div className="mt-5 flex justify-end gap-2">
          <Button variant="outline" onClick={onCancel} disabled={pending}>
            Cancel
          </Button>
          <Button onClick={onConfirm} disabled={pending}>
            Reconcile anyway
          </Button>
        </div>
      </div>
    </div>
  );
}
