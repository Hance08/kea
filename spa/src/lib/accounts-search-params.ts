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
  limit: z.coerce.number().int().positive().default(10),
  offset: z.coerce.number().int().nonnegative().default(0),
  // Sort is intentionally optional rather than `.default()` so the rest of
  // the codebase doesn't have to thread it through every `<Link search>`.
  // Consumers fall back to 'balance_desc' when undefined.
  sort: z.enum(['balance_desc', 'balance_asc']).optional(),
});

export type AccountsSearch = z.infer<typeof accountsSearchSchema>;

export function parseAccountsSearch(s: Record<string, unknown>): AccountsSearch {
  return accountsSearchSchema.parse(s);
}
