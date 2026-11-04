import { addDays, daysBetween } from '@/i18n/format'

// Span is the period the progress shows: so many days up to today, the
// phase today is in, or all the days a history has (all).
export type Span = '30' | 'phase' | '90' | '365' | 'all'

export const spans: readonly Span[] = ['30', 'phase', '90', '365', 'all']

// A history is of 366 days at most (the API's), and its table of weeks
// of a year's.
const mostDays = 366
const mostWeeks = 52

// A phase younger than this many days is shown from the start by
// default: its own days would be a few points.
const youngDays = 14

// defaultSpan is the span shown when none is asked for: the phase's,
// once it has days enough to show; else all of them.
export function defaultSpan(today: string, phaseStart?: string): Span {
  return phaseStart !== undefined && daysBetween(phaseStart, today) >= youngDays ? 'phase' : 'all'
}

// firstDay is a span's first day, up to today: the phase's first, but no
// more than the days a history has; all, those days. A chart starts at
// its first data after it (fromFirst).
export function firstDay(span: Span, today: string, phaseStart?: string): string {
  const earliest = addDays(today, 1 - mostDays)
  if (span === 'all') {
    return earliest
  }
  if (span !== 'phase') {
    return addDays(today, 1 - Number(span))
  }
  return phaseStart === undefined || phaseStart < earliest ? earliest : phaseStart
}

// byMonth reports whether a span's axis names months rather than days:
// one of more than half a year.
export function byMonth(from: string, today: string): boolean {
  return daysBetween(from, today) + 1 > 180
}

// weekStart is the Monday of a date's week: a calendar week runs from
// Monday to Sunday, as the server's (day.Date.WeekStart).
export function weekStart(date: string): string {
  const weekday = new Date(`${date}T00:00:00Z`).getUTCDay()
  return addDays(date, -((weekday + 6) % 7))
}

// weeksOf is the table of weeks of a span: the calendar weeks with a day
// of it up to yesterday (today is open), the last up to yesterday, at
// most a year's. from is the history to ask for, the Monday a week before
// the first, whose trend weight the first's change is measured from;
// count the weeks shown, 0 when the span has none over yet.
export function weeksOf(first: string, today: string): { from: string; to: string; count: number } {
  const to = addDays(today, -1)
  const last = weekStart(to)
  const weeks = to < first ? 0 : daysBetween(weekStart(first), last) / 7 + 1
  const count = Math.max(0, Math.min(mostWeeks, weeks))
  return { from: addDays(last, -7 * count), to, count }
}
