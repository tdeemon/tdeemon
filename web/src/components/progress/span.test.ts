import { expect, test } from 'vitest'

import { defaultSpan, firstDay, weeksOf, weekStart } from '@/components/progress/span'

test('a week starts on Monday, as a calendar week does', () => {
  // Monday 26 October 2026 to Sunday 1 November.
  for (const date of ['2026-10-26', '2026-10-29', '2026-11-01']) {
    expect(weekStart(date)).toBe('2026-10-26')
  }
  expect(weekStart('2026-10-25')).toBe('2026-10-19')
})

test.each([
  // Thursday the 29th: the weeks of the span up to yesterday, the last of
  // its Monday to Wednesday, and the Monday a week before the first.
  { first: '2026-09-30', today: '2026-10-29', count: 5, from: '2026-09-21', to: '2026-10-28' },
  // On a Monday, last week is the last, whole.
  { first: '2026-09-28', today: '2026-10-26', count: 4, from: '2026-09-21', to: '2026-10-25' },
  // A span of today alone has no week over yet.
  { first: '2026-10-29', today: '2026-10-29', count: 0, from: '2026-10-26', to: '2026-10-28' },
  // A year's are 52 at most.
  { first: '2025-10-30', today: '2026-10-29', count: 52, from: '2025-10-27', to: '2026-10-28' },
])('the span from $first to $today has $count calendar weeks', ({ first, today, ...weeks }) => {
  expect(weeksOf(first, today)).toEqual(weeks)
})

test.each([
  // A phase of two weeks and more is shown by default; a younger one, or
  // none, from the start.
  { start: '2026-10-01', span: 'phase' },
  { start: '2026-10-15', span: 'phase' },
  { start: '2026-10-16', span: 'all' },
  { start: undefined, span: 'all' },
])('a phase from $start is shown by default as $span', ({ start, span }) => {
  expect(defaultSpan('2026-10-29', start)).toBe(span)
})

test('all is the days a history has, the phase its own', () => {
  expect(firstDay('all', '2026-10-29')).toBe('2025-10-29')
  expect(firstDay('phase', '2026-10-29', '2026-10-01')).toBe('2026-10-01')
  expect(firstDay('30', '2026-10-29')).toBe('2026-09-30')
})
