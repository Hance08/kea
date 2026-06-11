import { z } from 'zod';

const sortSchema = z.enum(['balance_desc', 'balance_asc']);

export const balancesSearchSchema = z.object({
  a_offset: z.coerce.number().int().nonnegative().default(0),
  a_sort: sortSchema.default('balance_desc'),
  l_offset: z.coerce.number().int().nonnegative().default(0),
  l_sort: sortSchema.default('balance_desc'),
});

export type BalancesSearch = z.infer<typeof balancesSearchSchema>;

export function parseBalancesSearch(s: Record<string, unknown>): BalancesSearch {
  return balancesSearchSchema.parse(s);
}
