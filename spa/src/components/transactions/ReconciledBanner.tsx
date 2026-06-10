import { Alert, AlertDescription, AlertTitle } from '@/components/ui/alert';

export function ReconciledBanner() {
  return (
    <Alert>
      <AlertTitle>This transaction is reconciled</AlertTitle>
      <AlertDescription>
        Reconciled transactions are locked. To edit or delete this transaction, unreconcile it from
        the Reconcile screen first.
      </AlertDescription>
    </Alert>
  );
}
