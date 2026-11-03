import { cleanup, fireEvent, screen, waitFor, within } from '@testing-library/react'
import { afterEach, expect, test, vi } from 'vitest'

import type { components } from '@/api/schema'
import type { Day } from '@/api/types'
import { setLocale } from '@/i18n/locale'
import { day, fakeServer, renderAt, signedIn, type Handler } from '@/test/app'
import { dayServer } from '@/test/day-server'

afterEach(() => {
  cleanup()
  vi.unstubAllGlobals()
  setLocale('en')
  localStorage.clear()
})

type HistoryDay = components['schemas']['HistoryDayBody']

// A day before today, of the week the history answers.
const before = '2026-10-22'

// summaryWith is calibration's summary, its TDEE today's: measured and
// trusted at kcal, or nothing measured.
function summaryWith(kcal?: number) {
  return {
    date: '2026-10-23',
    incomplete_days: [],
    recovery: { flags: [] },
    tdee: {
      before_phase: false,
      doubts: kcal === undefined ? ['few_days'] : [],
      from: '2026-09-11',
      to: before,
      logged_days: 40,
      measured: kcal !== undefined,
      trusted: kcal !== undefined,
      weigh_ins: 30,
      ...(kcal === undefined ? {} : { kcal, half_width: 60 }),
      needs: {
        days: { has: kcal === undefined ? 9 : 42, need: 14 },
        logged_days: { has: 40, need: 7 },
        weigh_ins: { has: 30, need: 6 },
        weigh_in_days: { has: 41, need: 7 },
      },
      trust: { half_width: 200, weigh_ins: 3, logged_days: 4 },
    },
    verdict: {
      action: 'none',
      flags: [],
      gap_kcal: 0,
      guards: [],
      rate: 'none',
      reasons: ['no_phase'],
      step_kcal: 0,
    },
    weeks: [],
  }
}

// eaten is a day's total of kcal, its macros on their targets.
function eaten(kcal: number, over: Partial<Day> = {}): Day {
  return day({
    target: { kcal: 2250, protein: 160, carbs: 250, fat: 70 },
    total: { kcal, protein: 160, carbs: 250, fat: 70 },
    ...over,
  })
}

// serve answers for a day (today unless its date says), today's TDEE
// (the summary's) and the week's history, of which the days given.
function serve(
  shown: Day,
  tdee: number | undefined,
  days: HistoryDay[] = [],
  locale: 'en' | 'hr' = 'en',
) {
  const routes: Record<string, Handler> = {
    ...signedIn(locale),
    ...dayServer(shown).routes,
    'GET /api/v1/summary': () => Response.json(summaryWith(tdee)),
    'GET /api/v1/history': (request) => {
      const query = new URL(request.url).searchParams
      return Response.json({
        from: query.get('from'),
        to: query.get('to'),
        days,
        weeks: [],
      })
    },
  }
  fakeServer(routes)
}

// historyDay is a day of the history, eaten of its target, with the TDEE
// it had then.
function historyDay(date: string, intake: number, target: number, tdee?: number): HistoryDay {
  return {
    date,
    status: 'complete',
    type: 'rest',
    meals: 3,
    intake: { kcal: intake, protein: 150, carbs: 250, fat: 70 },
    target: { kcal: target, protein: 160, carbs: 250, fat: 70 },
    ...(tdee === undefined
      ? {}
      : { tdee: { kcal: tdee, half_width: 60, trusted: true, before_phase: false, to: date } }),
  }
}

async function summaryRegion() {
  return screen.findByRole('region', { name: 'Energy and macros' })
}

// equationOf is the target's arithmetic, as it reads, once it shows.
async function equationOf(summary: HTMLElement): Promise<string> {
  const tdee = await within(summary).findByRole('link', { name: /^TDEE / })
  return (tdee.parentElement?.textContent ?? '').trim()
}

// Within 5 % of the target, and 100 kcal at least, a day over it is on
// it: in the brand's colour, and said so.
test('a day a little over its target is on it, in no other colour', async () => {
  serve(eaten(2300), undefined)
  renderAt('/')

  const summary = await summaryRegion()
  expect(within(summary).getByText('Eaten 2,300 of 2,250 kcal, on target (+50)')).toBeDefined()
  expect(within(summary).getByText('+50')).toBeDefined()
  expect(within(summary).getByText('on target')).toBeDefined()
  expect(summary.querySelector('.stroke-over, .text-over')).toBeNull()
})

// Past that, the ring is full, a thinner arc runs on in the over colour,
// and the number says how far: never an error's colour.
test('a day well over its target says by how much, in a warm colour and a thinner arc', async () => {
  serve(eaten(2930), undefined)
  renderAt('/')

  const summary = await summaryRegion()
  expect(within(summary).getByText('Eaten 2,930 of 2,250 kcal, 680 over target')).toBeDefined()
  const number = within(summary).getByText('+680')
  expect(number.classList.contains('text-over')).toBe(true)
  const arcs = summary.querySelectorAll('svg circle')
  const over = [...arcs].find((c) => c.classList.contains('stroke-over'))
  expect(over?.getAttribute('stroke-width')).toBe('6')
  const full = [...arcs].find((c) => c.classList.contains('stroke-primary'))
  expect(full?.getAttribute('stroke-width')).toBe('12')
  expect(summary.querySelector('.text-destructive, .stroke-destructive')).toBeNull()
})

// The middle's number takes a smaller size the more characters it has,
// so +1,210 fits within the ring.
test("a long number in the ring's middle is smaller, to fit within it", async () => {
  serve(eaten(3460), undefined)
  renderAt('/')

  const summary = await summaryRegion()
  const number = within(summary).getByText('+1,210')
  expect(number.className).toContain('text-[1.75rem]')
  cleanup()

  serve(eaten(1130), undefined)
  renderAt('/')
  expect(within(await summaryRegion()).getByText('1,120').className).toContain('text-[2.125rem]')
})

// The target's arithmetic: the TDEE less the planned deficit, or plus a
// surplus, its TDEE a link to the progress; and only the ring says what
// is left.
test("the target's arithmetic is the TDEE less the deficit, or plus the surplus", async () => {
  serve(eaten(1136), 2409)
  renderAt('/')
  let summary = await summaryRegion()
  expect(await equationOf(summary)).toBe('Target 2,250 = TDEE 2,409 − 159 deficit')
  expect(within(summary).getByRole('link', { name: 'TDEE 2,409' }).getAttribute('href')).toBe(
    '/progress',
  )
  // What the TDEE leaves after what was eaten is no number of its own.
  expect(within(summary).queryByText('Left to TDEE')).toBeNull()
  expect(within(summary).queryByText('1,273')).toBeNull()
  cleanup()

  serve(
    eaten(1136, { type: 'training', target: { kcal: 2800, protein: 160, carbs: 330, fat: 70 } }),
    2409,
  )
  renderAt('/')
  summary = await summaryRegion()
  expect(await equationOf(summary)).toBe('Target 2,800 = TDEE 2,409 + 391 surplus')
})

// countsOf are calibration's counts in the summary, each as it reads.
async function countsOf(summary: HTMLElement): Promise<(string | null)[]> {
  const term = await within(summary).findByText('Calibration')
  return within(term.parentElement ?? summary)
    .getAllByRole('listitem')
    .map((li) => li.textContent)
}

// Until calibration has a TDEE, it says in the TDEE's place how far it is
// from one: its counts, short, and the day a first may come.
test.each([
  ['a target', eaten(1800)],
  ['no target yet', day({ total: { kcal: 1800, protein: 120, carbs: 200, fat: 60 } })],
])('without a TDEE, calibration says how far it is, with %s', async (_, shown) => {
  fakeServer({
    ...signedIn(),
    ...dayServer(shown).routes,
    'GET /api/v1/summary': () =>
      Response.json({ ...summaryWith(), tdee: { ...summaryWith().tdee, expected: '2026-11-02' } }),
  })
  renderAt('/')

  const summary = await summaryRegion()
  expect(await countsOf(summary)).toEqual([
    'Days9/14',
    'Logged days40/7enough',
    'Weigh-ins30/6enough',
  ])
  expect(within(summary).getByText('First TDEE around Nov 2 if you keep this up.')).toBeDefined()
  expect(within(summary).queryByRole('link', { name: /^TDEE / })).toBeNull()
})

test("a day before says nothing of calibration now, nor a TDEE's day of one", async () => {
  serve(eaten(2001, { date: before, status: 'complete' }), undefined, [
    historyDay(before, 2001, 2250, 2450),
  ])
  renderAt(`/?date=${before}`)

  const summary = await summaryRegion()
  expect(await equationOf(summary)).toBe('Target 2,250 = TDEE 2,450 − 200 deficit')
  expect(within(summary).queryByText('Calibration')).toBeNull()
})

test('a day before under its target says how far under, not what is left', async () => {
  serve(eaten(2001, { date: before, status: 'complete' }), 2409, [
    historyDay(before, 2001, 2250, 2450),
  ])
  renderAt(`/?date=${before}`)

  const summary = await summaryRegion()
  expect(within(summary).getByText('Eaten 2,001 of 2,250 kcal, 249 under target')).toBeDefined()
  expect(within(summary).getByText('under target')).toBeDefined()
  expect(within(summary).queryByText('kcal left')).toBeNull()
  // The TDEE it had then, from its week's history, not today's.
  expect(await equationOf(summary)).toBe('Target 2,250 = TDEE 2,450 − 200 deficit')
})

test('a day before over its target says it in a sentence, beside its TDEE', async () => {
  serve(eaten(2930, { date: before, status: 'complete' }), 2409, [
    historyDay(before, 2930, 2250, 3430),
  ])
  renderAt(`/?date=${before}`)

  const summary = await summaryRegion()
  expect(await within(summary).findByText('680 over target, still 500 under TDEE.')).toBeDefined()
  expect(within(summary).getByText('over target')).toBeDefined()
})

test('a day before over its target and its TDEE says both, and no number below 0', async () => {
  serve(eaten(2930, { date: before, status: 'complete' }), 2409, [
    historyDay(before, 2930, 2250, 2730),
  ])
  renderAt(`/?date=${before}`)

  const summary = await summaryRegion()
  expect(await within(summary).findByText('680 over target and 200 over TDEE.')).toBeDefined()
  expect(within(summary).queryByText(/-\d/)).toBeNull()
})

// A day before on its target, or without a TDEE then, has no sentence.
test('a day before on its target, or with no TDEE then, says nothing more', async () => {
  serve(eaten(2300, { date: before, status: 'complete' }), 2409, [
    historyDay(before, 2300, 2250, 2600),
  ])
  renderAt(`/?date=${before}`)

  let summary = await summaryRegion()
  expect(within(summary).getByText('on target')).toBeDefined()
  expect(await equationOf(summary)).toBe('Target 2,250 = TDEE 2,600 − 350 deficit')
  expect(within(summary).queryByText(/over target/)).toBeNull()
  cleanup()

  serve(eaten(2930, { date: before, status: 'complete' }), 2409, [historyDay(before, 2930, 2250)])
  renderAt(`/?date=${before}`)
  summary = await summaryRegion()
  expect(within(summary).getByText('over target')).toBeDefined()
  expect(within(summary).queryByText(/TDEE\.$/)).toBeNull()
  expect(within(summary).queryByRole('link', { name: /^TDEE / })).toBeNull()
})

test('the day speaks Croatian, over its target and beside its TDEE', async () => {
  serve(
    eaten(2430, { date: before, status: 'complete' }),
    2409,
    [historyDay(before, 2430, 2250, 2930)],
    'hr',
  )
  renderAt(`/?date=${before}`)

  const summary = await screen.findByRole('region', { name: 'Energija i makroi' })
  expect(within(summary).getByText('+180')).toBeDefined()
  expect(within(summary).getByText('iznad cilja')).toBeDefined()
  expect(
    await within(summary).findByText('180 iznad cilja, i dalje 500 ispod TDEE-a.'),
  ).toBeDefined()
  expect(await equationOf(summary)).toBe('Cilj 2.250 = TDEE 2.930 − 680 deficit')
})

// What the TDEE is a tap away: a dialog named by the number, its
// sentences its description; Escape closes it.
test('the TDEE says what it is, an ⓘ away', async () => {
  serve(eaten(1136), 2409)
  renderAt('/')

  const summary = await summaryRegion()
  const tip = await within(summary).findByRole('button', { name: 'What TDEE is' })
  fireEvent.click(tip)
  const dialog = await screen.findByRole('dialog', { name: 'TDEE' })
  expect(dialog.textContent).toContain('with 80% probability, your real TDEE is within that range')
  fireEvent.keyDown(dialog, { key: 'Escape' })
  await waitFor(() => {
    expect(screen.queryByRole('dialog', { name: 'TDEE' })).toBeNull()
  })
})
