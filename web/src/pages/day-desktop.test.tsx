import { cleanup, fireEvent, screen, within } from '@testing-library/react'
import { afterEach, expect, test, vi } from 'vitest'

import type { Meal } from '@/api/types'
import { day, fakeServer, renderAt, signedIn, today } from '@/test/app'
import { planBody } from '@/test/plan'

afterEach(() => {
  cleanup()
  vi.unstubAllGlobals()
  localStorage.clear()
})

// desktop makes the window a desktop's width, as matchMedia says.
function desktop() {
  vi.stubGlobal('matchMedia', (query: string) => ({
    matches: query === '(min-width: 64rem)',
    media: query,
    addEventListener: () => undefined,
    removeEventListener: () => undefined,
  }))
}

const breakfast: Meal = {
  id: '0193c3a5-0000-7000-8000-0000000000a1',
  // 08:45 in Los Angeles, the session user's time zone.
  eaten_at: '2026-10-23T15:45:00Z',
  description: 'Oats with a banana',
  client: 'mcp',
  total: { kcal: 520, protein: 18, carbs: 92, fat: 9 },
  items: [
    {
      id: '0193c3a5-0000-7000-8000-0000000000b1',
      name: 'Oats',
      quantity: { amount: 80, unit: 'g' },
      amount: 80,
      unit: 'g',
      nutrients: { kcal: 520, protein: 18, carbs: 92, fat: 9 },
      source: 'label',
    },
  ],
}

// A day of a week whose days logged ate 1,905 a day, of a target of
// 2,079 (the week's most); in a cut, from 1 September to 23 November.
function serveDay() {
  const target = { kcal: 2079, protein: 160, carbs: 220, fat: 70 }
  const logged = (date: string, kcal: number) => ({
    date,
    status: 'complete',
    type: 'rest',
    meals: 3,
    intake: { kcal, protein: 150, carbs: 200, fat: 70 },
    target,
  })
  fakeServer({
    ...signedIn(),
    'GET /api/v1/days/today': () =>
      Response.json(
        day({
          target,
          total: { kcal: 1460, protein: 88, carbs: 146, fat: 41 },
          meals: [breakfast],
        }),
      ),
    'GET /api/v1/history': () =>
      Response.json({
        from: '2026-10-19',
        to: today,
        days: [logged('2026-10-19', 1850), logged('2026-10-20', 1960)],
        weeks: [],
      }),
    'GET /api/v1/plan': () =>
      Response.json(
        planBody({
          phases: [{ type: 'cut', start: '2026-09-01', end: '2026-11-23', rate_pct_week: -0.75 }],
        }),
      ),
  })
}

test("a desktop's day: the meals a table with their total, the week, the body and the check-in beside it", async () => {
  desktop()
  serveDay()
  renderAt('/')

  // The meals in columns: each row its time, name and marks, kcal and
  // each macro, said with its name; the heads for the eye; the total last.
  const meals = await screen.findByRole('region', { name: 'Meals' })
  const meal = within(meals).getByRole('article', { name: 'Oats with a banana' })
  const row = within(meal).getByRole('button', { expanded: false })
  expect(row.textContent).toBe('8:45 AMOats with a bananaAI520520 kcalP 18C 92F 9')
  const heads = meals.querySelector('[aria-hidden="true"]')
  expect(Array.from(heads?.children ?? [], (c) => c.textContent)).toEqual([
    'Time',
    'Meal',
    'kcal',
    'P',
    'C',
    'F',
  ])
  expect(within(meals).getByText('Total').parentElement?.textContent).toBe(
    'Total1,4601,460 kcalP 88C 146F 41',
  )
  // The row still opens to its items, and the ways to change it.
  fireEvent.click(row)
  expect(
    within(meal).getByRole('button', { name: 'Change the meal: Oats with a banana' }),
  ).toBeDefined()

  // Beside the day: the week first, its days and its average in a line.
  const week = screen.getByRole('region', { name: 'This week' })
  expect(within(week).getByRole('navigation', { name: 'The week' })).toBeDefined()
  expect(
    await within(week).findByRole('button', {
      name: 'This week: average 1,905 of 2,079 · 174 under a day · 2 days logged',
    }),
  ).toBeDefined()
  // Then the body and the check-in, open; the phase is the hero's.
  const order = screen
    .getAllByRole('region')
    .map((r) => r.getAttribute('aria-labelledby'))
    .filter((id) => ['day-week', 'day-body', 'day-check-in'].includes(id ?? ''))
  expect(order).toEqual(['day-week', 'day-body', 'day-check-in'])
  expect(screen.getByRole('group', { name: 'Mood' })).toBeDefined()
  const summary = screen.getByRole('region', { name: 'Energy and macros' })
  expect(await within(summary).findByRole('link', { name: 'Cut · week 8 of 12' })).toBeDefined()
  expect(screen.getAllByRole('link', { name: /^Cut/ })).toHaveLength(1)
})

test('the phone has the hero, then its week, the meals as cards, the body and the check-in as a row', async () => {
  serveDay()
  renderAt('/')

  const meals = await screen.findByRole('region', { name: 'Meals' })
  expect(within(meals).queryByText('Total')).toBeNull()
  const order = screen
    .getAllByRole('region')
    .map((r) => r.getAttribute('aria-labelledby'))
    .filter((id) =>
      ['day-summary', 'day-week', 'day-meals', 'day-body', 'day-check-in'].includes(id ?? ''),
    )
  expect(order).toEqual(['day-summary', 'day-week', 'day-meals', 'day-body', 'day-check-in'])
  const summary = screen.getByRole('region', { name: 'Energy and macros' })
  expect(await within(summary).findByRole('link', { name: 'Cut · week 8 of 12' })).toBeDefined()
  // The check-in a row, which opens.
  const checkIn = screen.getByRole('region', { name: 'Check-in' })
  expect(within(checkIn).queryByRole('group', { name: 'Mood' })).toBeNull()
  fireEvent.click(within(checkIn).getByRole('button', { name: 'Add a check-in' }))
  expect(screen.getByRole('group', { name: 'Mood' })).toBeDefined()
})

// On a desktop another day's header is one row as a phone's: the arrows
// either side of the date, the way back to today, and the user's mark in
// a window too low for the sidebar.
test("another day's header on a desktop is one row", async () => {
  desktop()
  fakeServer({
    ...signedIn(),
    'GET /api/v1/days/2026-10-22': () => Response.json(day({ date: '2026-10-22' })),
  })
  renderAt('/?date=2026-10-22')

  const date = await screen.findByRole('heading', { level: 1, name: 'Thursday, October 22' })
  const header = date.closest('header')
  const parts = Array.from(header?.querySelectorAll('a, h1, button') ?? []).map(
    (e) => e.getAttribute('aria-label') ?? e.textContent,
  )
  expect(parts).toEqual([
    'The day before',
    'Thursday, October 22',
    'The day after',
    'Back to today',
    // The user's mark, shown in a window too low for the sidebar.
    'Settings and account',
  ])
  // The week beside the day: that week's.
  expect(await screen.findByRole('region', { name: 'This week' })).toBeDefined()
})

const lunch: Meal = {
  ...breakfast,
  id: '0193c3a5-0000-7000-8000-0000000000a2',
  // 12:30 in Los Angeles.
  eaten_at: '2026-10-23T19:30:00Z',
  description: 'Lunch',
  client: 'web',
}

// keyServer is today with breakfast and lunch, which takes a meal logged
// (a repeat, at 13:00) and a meal deleted as the API does; each change
// sent is kept.
function keyServer() {
  let meals: Meal[] = [breakfast, lunch]
  const sent: string[] = []
  const todays = () => day({ meals })
  const summary = () => {
    const { meals: _meals, metrics: _metrics, ...rest } = todays()
    return rest
  }
  fakeServer({
    ...signedIn(),
    'GET /api/v1/days/today': () => Response.json(todays()),
    [`GET /api/v1/days/${today}`]: () => Response.json(todays()),
    'POST /api/v1/meals': () => {
      sent.push('POST /api/v1/meals')
      const meal: Meal = {
        ...lunch,
        id: '0193c3a5-0000-7000-8000-0000000000a3',
        eaten_at: '2026-10-23T20:00:00Z',
      }
      meals = [...meals, meal]
      return Response.json({ meal, day: summary(), warnings: [] }, { status: 201 })
    },
    [`DELETE /api/v1/meals/${breakfast.id}`]: () => {
      sent.push(`DELETE ${breakfast.id}`)
      meals = meals.filter((m) => m.id !== breakfast.id)
      return Response.json({ day: summary() })
    },
  })
  return sent
}

// rowOf is the row of a meal, of its name: the button that opens it.
function rowOf(name: string, which = 0): HTMLElement {
  const meal = screen.getAllByRole('article', { name })[which]
  const row = meal && within(meal).getAllByRole('button')[0]
  if (row === undefined) {
    throw new Error(`no row of ${name}`)
  }
  return row
}

test("a desktop's meals are one stop of Tab, whose arrows move between the rows", async () => {
  desktop()
  keyServer()
  renderAt('/')
  await screen.findByRole('article', { name: 'Lunch' })

  // The first row is Tab's stop at first, and only it.
  expect(rowOf('Oats with a banana').tabIndex).toBe(0)
  expect(rowOf('Lunch').tabIndex).toBe(-1)
  rowOf('Oats with a banana').focus()
  fireEvent.keyDown(rowOf('Oats with a banana'), { key: 'ArrowDown' })
  expect(document.activeElement).toBe(rowOf('Lunch'))
  // The row the keyboard is on is Tab's stop now.
  expect(rowOf('Lunch').tabIndex).toBe(0)
  expect(rowOf('Oats with a banana').tabIndex).toBe(-1)
  // The last goes no further; Home and End go to the ends.
  fireEvent.keyDown(rowOf('Lunch'), { key: 'ArrowDown' })
  expect(document.activeElement).toBe(rowOf('Lunch'))
  fireEvent.keyDown(rowOf('Lunch'), { key: 'Home' })
  expect(document.activeElement).toBe(rowOf('Oats with a banana'))
  fireEvent.keyDown(rowOf('Oats with a banana'), { key: 'End' })
  expect(document.activeElement).toBe(rowOf('Lunch'))
  fireEvent.keyDown(rowOf('Lunch'), { key: 'ArrowUp' })
  expect(document.activeElement).toBe(rowOf('Oats with a banana'))
})

test("Enter opens a desktop's meal in its editor; a click opens it to its items", async () => {
  desktop()
  keyServer()
  renderAt('/')
  await screen.findByRole('article', { name: 'Lunch' })

  fireEvent.click(rowOf('Lunch'))
  expect(rowOf('Lunch').getAttribute('aria-expanded')).toBe('true')
  expect(screen.queryByRole('dialog')).toBeNull()

  const oats = rowOf('Oats with a banana')
  oats.focus()
  fireEvent.keyDown(oats, { key: 'Enter' })
  expect(await screen.findByRole('dialog', { name: 'Change the meal' })).toBeDefined()
  expect(oats.getAttribute('aria-expanded')).toBe('false')
})

test('R repeats a meal today, whose new row takes the focus; Delete deletes one, with Undo', async () => {
  desktop()
  const sent = keyServer()
  renderAt('/')
  await screen.findByRole('article', { name: 'Lunch' })

  rowOf('Lunch').focus()
  fireEvent.keyDown(rowOf('Lunch'), { key: 'r' })
  expect(await screen.findByText('Repeated: Lunch · 520 kcal')).toBeDefined()
  await vi.waitFor(() => {
    expect(document.activeElement).toBe(rowOf('Lunch', 1))
  })

  rowOf('Oats with a banana').focus()
  fireEvent.keyDown(rowOf('Oats with a banana'), { key: 'Delete' })
  expect(await screen.findByText('Deleted: Oats with a banana')).toBeDefined()
  expect(screen.getByRole('button', { name: 'Undo' })).toBeDefined()
  expect(sent).toEqual(['POST /api/v1/meals', `DELETE ${breakfast.id}`])
  // The row went, and the focus to the meals' heading.
  await vi.waitFor(() => {
    expect(screen.queryByRole('article', { name: 'Oats with a banana' })).toBeNull()
  })
  expect(document.activeElement).toBe(screen.getByRole('heading', { name: 'Meals' }))
})
