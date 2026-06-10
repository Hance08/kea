import { z } from 'zod';
import type { TransactionFilter } from './types';

export const transactionsSearchSchema = z.object({
  account_id: z.coerce.number().int().positive().optional(),
  type: z
    .enum(['Expense', 'Income', 'Transfer', 'Opening', 'Deposit', 'Withdrawal', 'Other'])
    .optional(),
  status: z.enum(['Pending', 'Cleared', 'Reconciled']).optional(),
  start_time: z.coerce.number().int().optional(),
  end_time: z.coerce.number().int().optional(),
  description: z.string().min(1).optional(),
  limit: z.coerce.number().int().positive().default(50),
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
  return f;
}

export function searchToListOptions(search: TransactionsSearch) {
  return { limit: search.limit, offset: search.offset, include_count: true };
}
