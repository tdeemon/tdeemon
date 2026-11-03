import { cleanup, screen, within } from '@testing-library/react'
import { afterEach, expect, test, vi } from 'vitest'

import type { components } from '@/api/schema'
import type { Meal } from '@/api/types'
import { setLocale } from '@/i18n/locale'
import { day, fakeServer, renderAt, signedIn, today } from '@/test/app'
import { planBody } from '@/test/plan'

type Proposal = components['schemas']['ProposalBody']

afterEach(() => {
  cleanup()
  vi.unstubAllGlobals()
  vi.useRealTimers()
  setLocale('en')
  localStorage.clear()
})

// at sets the clock to a time of today in Los Angeles, the user's.
function at(hours: number) {
  vi.useFakeTimers({ toFake: ['Date'] })
  // Daylight time there: UTC less 7 hours.
  const utc = new Date(`${today}T00:00:00Z`)
  utc.setUTCHours(hours + 7)
  vi.setSystemTime(utc)
}

const dinner: Meal = {
  id: '0193c3a5-0000-7000-8000-0000000000a1',
  // 18:30 in Los Angeles.
  eaten_at: '2026-10-24T01:30:00Z',
  description: 'Dinner',
  client: 'web',
  total: { kcal: 1500, protein: 90, carbs: 150, fat: 50 },
  items: [],
}

const proposal: Proposal = {
  id: '0193c3a5-0000-7000-8000-0000000000b1',
  status: 'pending',
  rationale: 'Made up.',
  by: { client: 'mcp', model: 'claude-opus-5-5' },
  changes: {
    phases: [],
    targets: [],
    training_weekdays: { before: [], after: ['monday'] },
    limits: [],
  },
  created_at: '2026-10-23T08:00:00Z',
  expires_at: '2026-10-30T08:00:00Z',
}

// serve is today, 1,500 eaten of 2,000, in a week whose two days logged
// ate 1,850 and 1,960 of 2,000; and a proposal that waits, if given.
function serve(pending?: Proposal, locale: 'en' | 'hr' = 'en') {
  const target = { kcal: 2000, protein: 160, carbs: 200, fat: 70 }
  const logged = (date: string, kcal: number) => ({
    date,
    status: 'complete',
    type: 'rest',
    meals: 3,
    intake: { kcal, protein: 150, carbs: 200, fat: 70 },
    target,
  })
  fakeServer({
    ...signedIn(locale),
    'GET /api/v1/days/today': () =>
      Response.json(
        day({ target, total: { kcal: 1500, protein: 90, carbs: 150, fat: 50 }, meals: [dinner] }),
      ),
    'GET /api/v1/history': () =>
      Response.json({
        from: '2026-10-19',
        to: today,
        days: [logged('2026-10-19', 1850), logged('2026-10-20', 1960)],
        weeks: [],
      }),
    'GET /api/v1/plan': () => Response.json(planBody(pending === undefined ? {} : { pending })),
  })
}

// In the evening the card under the hero says how the week ends if today
// ends here: (1,850 + 1,960 + 1,500) / 3 = 1,770, 230 under 2,000.
test('in the evening, the card under the hero says how the day ends the week', async () => {
  at(19)
  serve()
  renderAt('/')

  const card = await screen.findByRole('region', { name: 'How the day ends' })
  expect(
    within(card).getByText(
      'If you stop here, the week averages 1,770 a day, 230 under its target.',
    ),
  ).toBeDefined()
})

test('the evening card speaks Croatian', async () => {
  at(20)
  serve(undefined, 'hr')
  renderAt('/')

  const card = await screen.findByRole('region', { name: 'Kako dan završava' })
  expect(card.textContent).toContain(
    'Ako ovdje staneš, tjedan je u prosjeku 1.770 na dan, 230 ispod cilja.',
  )
})

// Before the evening there is none; a proposal that waits takes its
// place, in the coach's colour, at any hour.
test('before the evening, or while a proposal waits, the day says no end', async () => {
  at(14)
  serve()
  renderAt('/')
  expect(await screen.findByRole('region', { name: 'Meals' })).toBeDefined()
  expect(screen.queryByRole('region', { name: 'How the day ends' })).toBeNull()
  cleanup()

  at(19)
  serve(proposal)
  renderAt('/')
  expect(await screen.findByRole('link', { name: /^Claude proposes 1 plan change / })).toBeDefined()
  expect(screen.queryByRole('region', { name: 'How the day ends' })).toBeNull()
})
