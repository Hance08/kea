import { apiFetch } from './api';
import type {
  ReconcileCommitResponse,
  ReconcilePreviewResponse,
  UnreconciledResponse,
} from './types';

export function getUnreconciled(accountId: number): Promise<UnreconciledResponse> {
  return apiFetch<UnreconciledResponse>(`/api/accounts/${accountId}/unreconciled`);
}

export function previewReconcile(
  accountId: number,
  statementBalance: number,
  transactionIds: number[],
): Promise<ReconcilePreviewResponse> {
  return apiFetch<ReconcilePreviewResponse>(`/api/accounts/${accountId}/reconcile/preview`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({
      statement_balance: statementBalance,
      transaction_ids: transactionIds,
    }),
  });
}

export function commitReconcile(
  accountId: number,
  statementBalance: number,
  transactionIds: number[],
  allowMismatch: boolean,
): Promise<ReconcileCommitResponse> {
  return apiFetch<ReconcileCommitResponse>(`/api/accounts/${accountId}/reconcile`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({
      statement_balance: statementBalance,
      transaction_ids: transactionIds,
      allow_mismatch: allowMismatch,
    }),
  });
}
