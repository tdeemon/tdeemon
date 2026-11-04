import { cleanup, screen, within } from '@testing-library/react'
import { afterEach, expect, test, vi } from 'vitest'

import { fakeServer, renderAt, signedIn, today } from '@/test/app'

afterEach(() => {
  cleanup()
  vi.unstubAllGlobals()
  localStorage.clear()
})

// A day of the week's history, eaten of a target of 2,250.
function eaten(date: string, kcal: number) {
  return {
    date,
    status: 'complete',
    type: 'rest',
    meals: 3,
    intake: { kcal, protein: 150, carbs: 250, fat: 70 },
    target: { kcal: 2250, protein: 160, carbs: 250, fat: 70 },
  }
}

// Wednesday 21 a little over its target, on it; Thursday 22 well over.
function serveWeek() {
  fakeServer({
    ...signedIn(),
    'GET /api/v1/history': () =>
      Response.json({
        from: '2026-10-17',
        to: today,
        days: [eaten('2026-10-21', 2300), eaten('2026-10-22', 2600)],
        weeks: [],
      }),
  })
}

async function dayOf(name: string) {
  const week = await screen.findByRole('navigation', { name: 'The week' })
  return within(week).findByRole('link', { name })
}

// On its target, the column is the brand's colour alone, up to its tick.
test('a day a little over its target is on it in the week, in one colour', async () => {
  serveWeek()
  renderAt('/')

  const day = await dayOf('Wed 21: 2,300 of 2,250 kcal')
  expect(day.querySelector('.bg-over')).toBeNull()
  expect(day.querySelectorAll('.bg-primary')).toHaveLength(1)
  expect(day.querySelector('[data-target]')).not.toBeNull()
})

// Well over it, only what is above the target's tick is in the over
// colour, and thinner, so it shows without the colour too, and its name
// says it; nothing is an error's.
test('a day well over its target shows its excess above the target alone, thinner', async () => {
  serveWeek()
  renderAt('/')

  const day = await dayOf('Thu 22: 2,600 of 2,250 kcal, over target')
  // The week's most is 2,600: the brand's column up to the target, the
  // excess on it, narrower; the target's tick across.
  const bar = day.querySelector<HTMLElement>('span.bg-primary')
  const excess = day.querySelector<HTMLElement>('span.bg-over')
  const tick = day.querySelector<HTMLElement>('[data-target]')
  expect(Number.parseFloat(bar?.style.height ?? '')).toBeCloseTo((2250 / 2600) * 100, 5)
  expect(Number.parseFloat(excess?.style.bottom ?? '')).toBeCloseTo((2250 / 2600) * 100, 5)
  expect(Number.parseFloat(excess?.style.height ?? '')).toBeCloseTo((350 / 2600) * 100, 5)
  expect(Number.parseFloat(tick?.style.bottom ?? '')).toBeCloseTo((2250 / 2600) * 100, 5)
  expect(excess?.classList.contains('inset-x-1.5')).toBe(true)
  expect(day.querySelector('.bg-destructive')).toBeNull()
})

// Before any target a day's column is what it ate, at every width: the
// week shows before there is a plan.
test('a day before any target is a column of what it ate, with no tick', async () => {
  fakeServer({
    ...signedIn(),
    'GET /api/v1/history': () =>
      Response.json({
        from: '2026-10-19',
        to: today,
        days: [
          {
            date: '2026-10-20',
            status: 'complete',
            type: 'rest',
            meals: 2,
            intake: { kcal: 2651, protein: 120, carbs: 300, fat: 90 },
          },
          {
            date: '2026-10-21',
            status: 'complete',
            type: 'rest',
            meals: 2,
            intake: { kcal: 1300, protein: 90, carbs: 150, fat: 40 },
          },
        ],
        weeks: [],
      }),
  })
  renderAt('/')

  const most = await dayOf('Tue 20: 2,651 kcal')
  const half = await dayOf('Wed 21: 1,300 kcal')
  expect(most.querySelector<HTMLElement>('span.bg-primary')?.style.height).toBe('100%')
  expect(
    Number.parseFloat(half.querySelector<HTMLElement>('span.bg-primary')?.style.height ?? ''),
  ).toBeCloseTo((1300 / 2651) * 100, 5)
  expect(most.querySelector('[data-target]')).toBeNull()
})

// Today is outlined, and its column lighter while it is open; the day
// shown is the current one.
test('today is outlined, and the day shown is the current one', async () => {
  serveWeek()
  renderAt('/?date=2026-10-21')

  const shown = await dayOf('Wed 21: 2,300 of 2,250 kcal')
  expect(shown.getAttribute('aria-current')).toBe('page')
  const week = await screen.findByRole('navigation', { name: 'The week' })
  const now = within(week).getByRole('link', { name: /^Fri 23: / })
  expect(now.querySelector('.outline-foreground')).not.toBeNull()
  expect(shown.querySelector('.outline-foreground')).toBeNull()
})
