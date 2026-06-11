import { Button } from '@/components/ui/button';

interface Props {
  blockedReason?: string;
  onDelete: () => void;
  pending?: boolean;
}

export function DeleteAccountButton({ blockedReason, onDelete, pending }: Props) {
  if (blockedReason) {
    return (
      <Button size="sm" variant="destructive" disabled title={blockedReason}>
        Delete
      </Button>
    );
  }
  return (
    <Button size="sm" variant="destructive" onClick={onDelete} disabled={pending}>
      {pending ? 'Deleting…' : 'Delete'}
    </Button>
  );
}
