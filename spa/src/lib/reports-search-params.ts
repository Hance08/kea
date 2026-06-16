import { z } from 'zod';
import type { PeriodRange } from './period';

const monthPattern = /^\d{4}-(0[1-9]|1[0-2])$/;
const datePattern = /^\d{4}-\d{2}-\d{2}$/;

const rangeSchema = z.enum(['this-month', 'last-month', 'ytd', 'last-12mo', 'custom']);

export const periodSearchSchema = z.object({
  range: rangeSchema.default('this-month'),
  month: z.string().regex(monthPattern).optional(),
  from: z.string().regex(datePattern).optional(),
  to: z.string().regex(datePattern).optional(),
});

export type PeriodSearchParams = {
  range: PeriodRange;
  month?: string;
  from?: string;
  to?: string;
};

export function parsePeriodSearch(input: unknown): PeriodSearchParams {
  const result = periodSearchSchema.safeParse(input);
  if (result.success) {
    const out: PeriodSearchParams = { range: result.data.range };
    if (result.data.month !== undefined) out.month = result.data.month;
    if (result.data.from !== undefined) out.from = result.data.from;
    if (result.data.to !== undefined) out.to = result.data.to;
    return out;
  }
  return { range: 'this-month' };
}

export const asOfSearchSchema = z.object({
  as_of: z.coerce.number().int().positive().optional(),
});

export type AsOfSearchParams = z.infer<typeof asOfSearchSchema>;

export function parseAsOfSearch(input: unknown): AsOfSearchParams {
  const result = asOfSearchSchema.safeParse(input);
  if (!result.success) return {};
  return result.data.as_of === undefined ? {} : { as_of: result.data.as_of };
}

export const atSearchSchema = z.object({
  at: z.coerce.number().int().positive().optional(),
});

export type AtSearchParams = z.infer<typeof atSearchSchema>;

export function parseAtSearch(input: unknown): AtSearchParams {
  const result = atSearchSchema.safeParse(input);
  if (!result.success) return {};
  return result.data.at === undefined ? {} : { at: result.data.at };
}
