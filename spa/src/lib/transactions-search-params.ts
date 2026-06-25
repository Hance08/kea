import { z } from 'zod';
import type { TransactionFilter } from './types';

// Single source of truth for default page size. Used by the URL search
// schema and by every navigation that needs a fresh search context
// (e.g., post-create redirect, "Back to transactions" link).
export const DEFAULT_TRANSACTIONS_LIMIT = 10;

export const transactionsSearchSchema = z.object({
  account_id: z.coerce.number().int().positive().optional(),
  type: z
    .enum([
      'Expense',
      'Income',
      'Transfer',
      'Opening',
      'Deposit',
      'Withdrawal',
      'Other',
      'Investment',
    ])
    .optional(),
  status: z.enum(['Pending', 'Cleared', 'Reconciled']).optional(),
  start_time: z.coerce.number().int().optional(),
  end_time: z.coerce.number().int().optional(),
  description: z.string().min(1).optional(),
  regular: z
    .union([z.literal('true'), z.literal('false'), z.boolean()])
    .transform((v) => (typeof v === 'boolean' ? v : v === 'true'))
    .optional(),
  limit: z.coerce.number().int().positive().default(DEFAULT_TRANSACTIONS_LIMIT),
  offset: z.coerce.number().int().nonnegative().default(0),
});

export type TransactionsSearch = z.infer<typeof transactionsSearchSchema>;

export function parseTransactionsSearch(input: unknown): TransactionsSearch {
  const result = transactionsSearchSchema.safeParse(input);
  if (result.success) return result.data;
  return transactionsSearchSchema.parse({});
}

export function searchToFilter(search: TransactionsSearch): TransactionFilter {
  const f: TransactionFilter = {};
  if (search.account_id !== undefined) f.account_id = search.account_id;
  if (search.type !== undefined) f.type = search.type;
  if (search.status !== undefined) f.status = search.status;
  if (search.start_time !== undefined) f.start_time = search.start_time;
  if (search.end_time !== undefined) f.end_time = search.end_time;
  if (search.description !== undefined) f.description = search.description;
  if (search.regular !== undefined) f.regular = search.regular;
  return f;
}

export function searchToListOptions(search: TransactionsSearch) {
  return { limit: search.limit, offset: search.offset, include_count: true };
}
