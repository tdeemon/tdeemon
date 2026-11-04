import { cleanup, fireEvent, screen, waitFor, within } from '@testing-library/react'
import { afterEach, beforeEach, expect, test, vi } from 'vitest'

import type { Food, Meal } from '@/api/types'
import { setLocale } from '@/i18n/locale'
import { day, fakeServer, renderAt, signedIn, today, type Handler } from '@/test/app'
import { dayServer } from '@/test/day-server'

beforeEach(() => {
  // 9:30 on the 23rd in Los Angeles, a breakfast's hour.
  vi.useFakeTimers({ toFake: ['Date'] })
  vi.setSystemTime(new Date('2026-10-23T16:30:00Z'))
  // jsdom scrolls nothing.
  Element.prototype.scrollIntoView = () => undefined
})

afterEach(() => {
  cleanup()
  vi.useRealTimers()
  vi.unstubAllGlobals()
  setLocale('en')
  localStorage.clear()
})

const oats: Food = {
  id: '0193c3a5-0000-7000-8000-0000000000f2',
  name: 'Oat flakes',
  basis: '100g',
  nutrients: { kcal: 372, protein: 13, carbs: 59, fat: 7 },
  source: 'label',
  archived: false,
}

// porridge is yesterday's breakfast.
const porridge: Meal = {
  id: '0193c3a5-0000-7000-8000-0000000000b1',
  eaten_at: '2026-10-22T15:00:00Z',
  description: 'Porridge',
  client: 'web',
  total: { kcal: 372, protein: 13, carbs: 59, fat: 7 },
  items: [
    {
      id: '0193c3a5-0000-7000-8000-0000000000c1',
      food_id: oats.id,
      name: 'Oat flakes',
      quantity: { amount: 100, unit: 'g' },
      amount: 100,
      unit: 'g',
      nutrients: { kcal: 372, protein: 13, carbs: 59, fat: 7 },
      source: 'label',
    },
  ],
}

// server is today's day, which takes a weigh-in, yesterday's breakfast,
// the foods "oat" finds, and a meal logged, which it keeps.
function server(over: Record<string, Handler> = {}) {
  const todays = dayServer()
  const logged: unknown[] = []
  // Yesterday's meals, which a test changes as a coach would.
  const yesterday = { meals: [porridge] }
  fakeServer({
    ...signedIn(),
    ...todays.routes,
    'GET /api/v1/days/2026-10-22': () =>
      Response.json(day({ date: '2026-10-22', meals: yesterday.meals })),
    'GET /api/v1/foods': (request) =>
      Response.json({
        foods: new URL(request.url).searchParams.get('query') === 'oat' ? [oats] : [],
        unfound: [],
      }),
    'GET /api/v1/foods/list': () => Response.json({ foods: [], total: 0 }),
    'GET /api/v1/meals/recent': () => Response.json({ meals: [] }),
    'POST /api/v1/meals': async (request) => {
      logged.push(await request.json())
      const meal = { ...porridge, id: '0193c3a5-0000-7000-8000-0000000000a1' }
      const { meals: _m, metrics: _x, ...summary } = day()
      return Response.json({ meal, day: summary, warnings: [] }, { status: 201 })
    },
    ...over,
  })
  return { sent: todays.sent, logged, yesterday }
}

// palette is the command palette, once open.
async function palette() {
  return within(await screen.findByRole('dialog', { name: 'Search and commands' }))
}

// options are the palette's rows: their names and their keys.
function options(p: Awaited<ReturnType<typeof palette>>) {
  return p
    .getAllByRole('option')
    .map((o) => [o.textContent, o.getAttribute('aria-keyshortcuts')] as const)
}

test('Ctrl+K opens the palette from any screen: its actions and sections, each with its key', async () => {
  server()
  renderAt('/foods')
  await screen.findByRole('heading', { name: 'Foods', level: 1 })

  fireEvent.keyDown(document.body, { key: 'k', ctrlKey: true })

  const p = await palette()
  const field = p.getByRole('combobox', { name: 'Search foods, days and settings, or a command' })
  await waitFor(() => {
    expect(document.activeElement).toBe(field)
  })
  await waitFor(() => {
    expect(options(p)).toContainEqual(["Repeat yesterday's breakfast", null])
  })
  expect(options(p)).toEqual([
    ['Log a mealN', 'N'],
    ['Log a weigh-inW', 'W'],
    // No one key writes a meal: the palette's row alone repeats it.
    ["Repeat yesterday's breakfast", null],
    ['Undo the latest changeCtrl+Z', 'Control+Z'],
    ['Keyboard shortcuts?', '?'],
    ['TodayG D', null],
    ['FoodsG F', null],
    ['ProgressG P', null],
    ['PlanG L', null],
    ['SettingsG S', null],
  ])
  // The combobox's pattern: the first row active, the count said.
  const first = p.getAllByRole('option')[0]
  expect(field.getAttribute('aria-activedescendant')).toBe(first?.id)
  expect(first?.getAttribute('aria-selected')).toBe('true')
  expect(field.getAttribute('aria-controls')).toBe(p.getByRole('listbox').id)
  expect(p.getByRole('status').textContent).toBe('10 results')
  expect(p.getByRole('group', { name: 'Actions' })).toBeDefined()

  // ↓ moves the active row, ↑ back, and past the first to the last.
  fireEvent.keyDown(field, { key: 'ArrowDown' })
  expect(field.getAttribute('aria-activedescendant')).toBe(p.getAllByRole('option')[1]?.id)
  fireEvent.keyDown(field, { key: 'ArrowUp' })
  fireEvent.keyDown(field, { key: 'ArrowUp' })
  expect(field.getAttribute('aria-activedescendant')).toBe(p.getAllByRole('option').at(-1)?.id)

  // Ctrl+K again closes it.
  fireEvent.keyDown(field, { key: 'k', ctrlKey: true })
  await waitFor(() => {
    expect(screen.queryByRole('dialog', { name: 'Search and commands' })).toBeNull()
  })
})

test('"weigh" and Enter open the weigh-in, and Escape gives the focus back', async () => {
  const { sent } = server()
  renderAt('/')
  await screen.findByRole('heading', { name: 'Today', level: 1 })
  const sidebar = within(await screen.findByRole('navigation', { name: 'Sections' }))
  const search = sidebar.getByRole('button', { name: 'Search…' })
  expect(search.getAttribute('aria-keyshortcuts')).toBe('Control+K')
  search.focus()
  fireEvent.click(search)

  const p = await palette()
  const field = p.getByRole('combobox')
  fireEvent.change(field, { target: { value: 'weigh' } })
  expect(options(p)).toEqual([['Log a weigh-inW', 'W']])
  expect(p.getByRole('status').textContent).toBe('1 result')
  fireEvent.keyDown(field, { key: 'Enter' })

  const sheet = within(await screen.findByRole('dialog', { name: 'Log a weigh-in' }))
  const weight = await sheet.findByLabelText('Weight')
  await waitFor(() => {
    expect(document.activeElement).toBe(weight)
  })
  fireEvent.change(weight, { target: { value: '80.1' } })
  fireEvent.click(sheet.getByRole('button', { name: 'Save the weigh-in' }))
  expect(await screen.findByText('Saved: Weight')).toBeDefined()
  expect(sent).toEqual([
    { method: 'PUT', path: `/api/v1/days/${today}/metrics/weight`, body: { value: 80.1 } },
  ])
  // Saved, the sheet closes, and the focus is back on the search.
  await waitFor(() => {
    expect(screen.queryByRole('dialog', { name: 'Log a weigh-in' })).toBeNull()
  })
  await waitFor(() => {
    expect(document.activeElement).toBe(search)
  })

  // Escape closes the palette, and the focus is back too.
  fireEvent.keyDown(document.body, { key: 'k', ctrlKey: true })
  fireEvent.keyDown((await palette()).getByRole('combobox'), { key: 'Escape' })
  await waitFor(() => {
    expect(screen.queryByRole('dialog', { name: 'Search and commands' })).toBeNull()
  })
  await waitFor(() => {
    expect(document.activeElement).toBe(search)
  })
})

test('typing on the day opens the palette with what was typed; a shortcut, a field and a modifier do not', async () => {
  server()
  renderAt('/')
  await screen.findByRole('heading', { name: 'Today', level: 1 })

  // A field's keys are its own.
  const note = document.createElement('input')
  document.body.append(note)
  fireEvent.keyDown(note, { key: 'o' })
  note.remove()
  // Held with Alt or Ctrl, or within an IME's composition, a key types
  // nothing.
  fireEvent.keyDown(document.body, { key: 'o', altKey: true })
  fireEvent.keyDown(document.body, { key: 'o', isComposing: true })
  // ? is the shortcuts' list, G the start of a section's keys.
  fireEvent.keyDown(document.body, { key: 'g' })
  fireEvent.keyDown(document.body, { key: 'd' })
  await new Promise((resolve) => setTimeout(resolve, 20))
  expect(screen.queryByRole('dialog')).toBeNull()

  fireEvent.keyDown(document.body, { key: 'o' })
  const p = await palette()
  const field = p.getByRole('combobox')
  expect(field).toHaveProperty('value', 'o')
  fireEvent.change(field, { target: { value: 'oat' } })
  // The foods the server finds, a food's page a row away.
  await waitFor(() => {
    expect(options(p)).toEqual([['Oat flakes', null]])
  })
  expect(p.getByRole('group', { name: 'Foods' })).toBeDefined()
})

// fading is an animation as a browser plays it, which finishes once end
// is called.
function fading(): { finished: Promise<void>; end: () => void } {
  let end: (() => void) | undefined
  const finished = new Promise<void>((resolve) => {
    end = resolve
  })
  return {
    finished,
    end: () => {
      end?.()
    },
  }
}

// Opened again by typing while it still fades out, the palette starts
// anew, with the key typed, not the words of before.
test('the palette opened again as it fades out starts anew', async () => {
  server()
  renderAt('/')
  await screen.findByRole('heading', { name: 'Today', level: 1 })

  fireEvent.keyDown(document.body, { key: 'o' })
  const field = (await palette()).getByRole('combobox')
  fireEvent.change(field, { target: { value: 'oat' } })
  // Its fade, which a browser plays and jsdom does not: it ends when
  // fade() is called.
  const { finished: faded, end: fade } = fading()
  Object.defineProperty(Element.prototype, 'getAnimations', {
    configurable: true,
    value: () => [{ finished: faded }],
  })
  try {
    fireEvent.keyDown(field, { key: 'Escape' })
    expect(screen.getByRole('combobox')).toHaveProperty('value', 'oat')
    fireEvent.keyDown(document.body, { key: 'b' })
    await waitFor(() => {
      expect(screen.getByRole('combobox')).toHaveProperty('value', 'b')
    })
  } finally {
    fade()
    // jsdom has none of its own to put back.
    Reflect.deleteProperty(Element.prototype, 'getAnimations')
  }
})

// choose types in the palette and runs the row named.
async function choose(typed: string, row: string) {
  fireEvent.keyDown(document.body, { key: 'k', ctrlKey: true })
  const p = await palette()
  fireEvent.change(p.getByRole('combobox'), { target: { value: typed } })
  fireEvent.click(await p.findByRole('option', { name: row }))
}

test("a food found, a day and a setting lead to their pages; yesterday's breakfast is repeated", async () => {
  const { logged } = server()
  const router = renderAt('/')
  await screen.findByRole('heading', { name: 'Today', level: 1 })

  await choose('oat', 'Oat flakes')
  await waitFor(() => {
    expect(router.state.location.pathname).toBe(`/foods/${oats.id}`)
  })
  await choose('yesterday', 'Yesterday · Thursday, October 22')
  await waitFor(() => {
    expect(router.state.location.search).toEqual({ date: '2026-10-22' })
  })
  await choose('2026-09-01', 'Tuesday, September 1')
  await waitFor(() => {
    expect(router.state.location.search).toEqual({ date: '2026-09-01' })
  })
  await choose('notif', 'Notifications')
  await waitFor(() => {
    expect(router.state.location.pathname).toBe('/settings/notifications')
  })

  // Away from the day, a letter is no search, and R is no shortcut: it
  // logs nothing.
  fireEvent.keyDown(document.body, { key: 'o' })
  fireEvent.keyDown(document.body, { key: 'r' })
  await new Promise((resolve) => setTimeout(resolve, 50))
  expect(logged).toEqual([])
  expect(screen.queryByRole('dialog', { name: 'Search and commands' })).toBeNull()

  // The palette's row logs yesterday's breakfast again.
  await choose('rep', "Repeat yesterday's breakfast")
  expect(await screen.findByText(/^Repeated: Porridge/)).toBeDefined()
  expect(logged).toHaveLength(1)
})

test('the palette says it in Croatian, and nothing found', async () => {
  setLocale('hr')
  server(signedIn('hr'))
  renderAt('/foods')
  await screen.findByRole('heading', { level: 1 })

  fireEvent.keyDown(document.body, { key: 'k', ctrlKey: true })
  const p = within(await screen.findByRole('dialog', { name: 'Traženje i naredbe' }))
  fireEvent.change(p.getByRole('combobox'), { target: { value: 'vaga' } })
  expect(options(p)).toEqual([['Upiši vaganjeW', 'W']])
  fireEvent.change(p.getByRole('combobox'), { target: { value: 'zzz' } })
  await waitFor(() => {
    expect(p.getByRole('status').textContent).toBe('Nema rezultata')
  })
  expect(p.getByText('Ništa nije pronađeno.')).toBeDefined()
  expect(p.queryByRole('option')).toBeNull()
})

// The repeat asks for yesterday anew: a meal deleted since the palette
// showed it is not logged again, and it says there is none; a refusal
// is said, in a toast of its own, and the change before keeps its Undo.
test("yesterday's meal again is of yesterday as it is now, and says why it was not logged", async () => {
  const { logged, yesterday } = server({
    'POST /api/v1/meals': () =>
      Response.json(
        { status: 409, title: 'Conflict', detail: 'the food is archived' },
        { status: 409, headers: { 'Content-Type': 'application/problem+json' } },
      ),
  })
  renderAt('/foods')
  await screen.findByRole('heading', { name: 'Foods', level: 1 })

  await choose('rep', "Repeat yesterday's breakfast")
  expect(
    await screen.findByText('The server did not take this meal: the food is archived'),
  ).toBeDefined()

  // Deleted since, as a coach would: the palette showed it, but nothing
  // is logged, and it says so.
  fireEvent.keyDown(document.body, { key: 'k', ctrlKey: true })
  const p = await palette()
  fireEvent.change(p.getByRole('combobox'), { target: { value: 'rep' } })
  const row = await p.findByRole('option', { name: "Repeat yesterday's breakfast" })
  yesterday.meals = []
  fireEvent.click(row)
  expect(
    await screen.findByText('Yesterday has no meal of this time of day to repeat.'),
  ).toBeDefined()
  expect(logged).toEqual([])
})
