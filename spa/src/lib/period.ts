export type PeriodRange = 'this-month' | 'last-month' | 'ytd' | 'last-12mo' | 'custom';

export interface PeriodSearch {
  range: PeriodRange;
  month?: string; // 'YYYY-MM'
  from?: string;  // 'YYYY-MM-DD'
  to?: string;    // 'YYYY-MM-DD'
}

export interface ResolvedPeriod {
  apiParams: { month?: string; from?: string; to?: string };
  startUnix: number;
  endUnix: number;
  label: string;
}

const MONTH_NAMES = [
  'January', 'February', 'March', 'April', 'May', 'June',
  'July', 'August', 'September', 'October', 'November', 'December',
];
const MONTH_SHORT = [
  'Jan', 'Feb', 'Mar', 'Apr', 'May', 'Jun',
  'Jul', 'Aug', 'Sep', 'Oct', 'Nov', 'Dec',
];

function pad2(n: number): string {
  return n < 10 ? `0${n}` : String(n);
}

function utcDate(year: number, month1to12: number, day: number): Date {
  return new Date(Date.UTC(year, month1to12 - 1, day));
}

// Days in a given month using JS Date overflow (day 0 of next month = last day).
function daysInMonth(year: number, month1to12: number): number {
  return new Date(Date.UTC(year, month1to12, 0)).getUTCDate();
}

function monthBounds(year: number, month1to12: number): { startUnix: number; endUnix: number } {
  const start = utcDate(year, month1to12, 1).getTime() / 1000;
  const last = daysInMonth(year, month1to12);
  // 23:59:59 of the last day
  const endDate = new Date(Date.UTC(year, month1to12 - 1, last, 23, 59, 59));
  return { startUnix: start, endUnix: endDate.getTime() / 1000 };
}

function isoDate(unix: number): string {
  const d = new Date(unix * 1000);
  return `${d.getUTCFullYear()}-${pad2(d.getUTCMonth() + 1)}-${pad2(d.getUTCDate())}`;
}

function parseMonth(s: string): { year: number; month: number } | null {
  const m = /^(\d{4})-(\d{2})$/.exec(s);
  if (!m) return null;
  const year = Number(m[1]);
  const month = Number(m[2]);
  if (month < 1 || month > 12) return null;
  return { year, month };
}

function parseDate(s: string): { year: number; month: number; day: number } | null {
  const m = /^(\d{4})-(\d{2})-(\d{2})$/.exec(s);
  if (!m) return null;
  return { year: Number(m[1]), month: Number(m[2]), day: Number(m[3]) };
}

function formatRangeLabel(from: string, to: string): string {
  const f = parseDate(from);
  const t = parseDate(to);
  if (!f || !t) return `${from} – ${to}`;
  const sameYear = f.year === t.year;
  const fStr = `${MONTH_SHORT[f.month - 1]} ${f.day}`;
  const tStr = `${MONTH_SHORT[t.month - 1]} ${t.day}, ${t.year}`;
  return sameYear ? `${fStr} – ${tStr}` : `${fStr}, ${f.year} – ${tStr}`;
}

export function resolvePeriod(search: PeriodSearch, nowUnix: number = Date.now() / 1000): ResolvedPeriod {
  const now = new Date(nowUnix * 1000);
  const curYear = now.getUTCFullYear();
  const curMonth = now.getUTCMonth() + 1; // 1-12

  if (search.range === 'this-month') {
    const parsed = search.month ? parseMonth(search.month) : null;
    const y = parsed?.year ?? curYear;
    const m = parsed?.month ?? curMonth;
    const monthStr = `${y}-${pad2(m)}`;
    const { startUnix, endUnix } = monthBounds(y, m);
    return {
      apiParams: { month: monthStr },
      startUnix,
      endUnix,
      label: `${MONTH_NAMES[m - 1]} ${y}`,
    };
  }

  if (search.range === 'last-month') {
    let y = curYear;
    let m = curMonth - 1;
    if (m === 0) {
      m = 12;
      y -= 1;
    }
    const monthStr = `${y}-${pad2(m)}`;
    const { startUnix, endUnix } = monthBounds(y, m);
    return {
      apiParams: { month: monthStr },
      startUnix,
      endUnix,
      label: `${MONTH_NAMES[m - 1]} ${y}`,
    };
  }

  if (search.range === 'ytd') {
    const from = `${curYear}-01-01`;
    const to = isoDate(nowUnix);
    const start = utcDate(curYear, 1, 1).getTime() / 1000;
    return {
      apiParams: { from, to },
      startUnix: start,
      endUnix: nowUnix,
      label: `YTD ${curYear}`,
    };
  }

  if (search.range === 'last-12mo') {
    const fromUnix = utcDate(curYear - 1, curMonth, now.getUTCDate()).getTime() / 1000;
    const from = isoDate(fromUnix);
    const to = isoDate(nowUnix);
    return {
      apiParams: { from, to },
      startUnix: fromUnix,
      endUnix: nowUnix,
      label: 'Last 12 months',
    };
  }

  // custom
  if (search.from && search.to) {
    const f = parseDate(search.from);
    const t = parseDate(search.to);
    if (f && t) {
      const startUnix = utcDate(f.year, f.month, f.day).getTime() / 1000;
      const endUnix = new Date(Date.UTC(t.year, t.month - 1, t.day, 23, 59, 59)).getTime() / 1000;
      return {
        apiParams: { from: search.from, to: search.to },
        startUnix,
        endUnix,
        label: formatRangeLabel(search.from, search.to),
      };
    }
  }

  // fallback: this-month
  return resolvePeriod({ range: 'this-month' }, nowUnix);
}
