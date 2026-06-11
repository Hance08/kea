import { z } from 'zod';

export const accountsSearchSchema = z.object({
  q: z.string().optional(),
  type: z.enum(['A', 'L', 'C', 'R', 'E']).optional(),
  include_hidden: z
    .union([z.boolean(), z.enum(['true', 'false']).transform((v) => v === 'true')])
    .default(false),
  show_parents: z
    .union([z.boolean(), z.enum(['true', 'false']).transform((v) => v === 'true')])
    .default(false),
});

export type AccountsSearch = z.infer<typeof accountsSearchSchema>;

export function parseAccountsSearch(s: Record<string, unknown>): AccountsSearch {
  return accountsSearchSchema.parse(s);
}
