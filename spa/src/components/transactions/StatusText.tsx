import type { TransactionStatus } from '@/lib/types';

interface Props {
  status: TransactionStatus;
}

// Plain text only — no color, no emoji (design decision).
export function StatusText({ status }: Props) {
  return <span className="text-xs text-foreground">{status}</span>;
}
