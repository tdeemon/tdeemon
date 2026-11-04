import { cleanup, fireEvent, screen, waitFor, within } from '@testing-library/react'
import { afterEach, expect, test, vi } from 'vitest'

import type { Meal } from '@/api/types'
import { setLocale } from '@/i18n/locale'
import { fakeServer, me, renderAt } from '@/test/app'
import { dayServer } from '@/test/day-server'
import { dayWithoutTarget, newUser, summaryOfNothing } from '@/test/new-user'
import { planBody } from '@/test/plan'

afterEach(() => {
  cleanup()
  vi.unstubAllGlobals()
  setLocale('en')
  localStorage.clear()
})

async function card() {
  return within(await screen.findByRole('region', { name: 'First steps' }))
}

// steps are what each first step says, its line under it after a |: a
// screen reader's words before it, and not its number, which is shown
// only.
function steps(): string[] {
  return within(screen.getByRole('region', { name: 'First steps' }))
    .getAllByRole('listitem')
    .map((li) => {
      const said = li.cloneNode(true)
      if (!(said instanceof HTMLElement)) {
        return ''
      }
      for (const hidden of said.querySelectorAll('[aria-hidden]')) {
        hidden.remove()
      }
      for (const line of said.querySelectorAll('p + p')) {
        line.prepend('|')
      }
      return said.textContent
    })
}

const used = {
  id: '0193c3a5-0000-7000-8000-0000000000c2',
  name: 'Claude',
  scopes: ['tools' as const],
  created_at: '2026-10-22T08:00:00Z',
  last_used_at: '2026-10-22T09:00:00Z',
}

const meal: Meal = {
  id: '0193c3a5-0000-7000-8000-0000000000a1',
  eaten_at: '2026-10-22T15:45:00Z',
  description: 'Oats',
  client: 'mcp',
  total: { kcal: 300, protein: 11, carbs: 54, fat: 6 },
  items: [],
}

test("a new user's today is the first steps, none done, each the way to its own", async () => {
  fakeServer(newUser())
  renderAt('/')

  const steps4 = await card()
  expect(steps4.getByText('0 of 4')).toBeDefined()
  expect(steps()).toEqual([
    'To do:Complete your profile|Sex, birth date and height',
    'To do:First weigh-inkg',
    'To do:Connect your AI coach or a phone|The MCP address and a key, in 3 steps',
    'To do:Log your first meal|Search, a barcode or an estimate',
  ])
  expect(
    steps4
      .getByRole('link', { name: 'Complete your profile Sex, birth date and height' })
      .getAttribute('href'),
  ).toBe('/plan/profile')
  expect(
    steps4
      .getByRole('link', {
        name: 'Connect your AI coach or a phone The MCP address and a key, in 3 steps',
      })
      .getAttribute('href'),
  ).toBe('/settings/devices?connect=choose')
  expect(
    steps4
      .getByRole('link', { name: 'Log your first meal Search, a barcode or an estimate' })
      .getAttribute('href'),
  ).toBe('/?log=true')
  // In place of the hero of a day without a target.
  expect(screen.queryByText('A plan gives the day its target.')).toBeNull()
  // The step after them, the AI coach's, already says what it will be.
  expect(
    steps4.getByRole('region', { name: 'Then: ask your AI coach for a plan' }).textContent,
  ).toContain('The proposal lands in Plan; nothing changes until you accept it.')
  // On a phone, the page has no room for how tdeemon works.
  expect(screen.queryByRole('region', { name: 'How tdeemon works' })).toBeNull()
})

test('the first weigh-in is given in the card, which ticks it', async () => {
  const server = dayServer(dayWithoutTarget())
  fakeServer({ ...newUser(), ...server.routes })
  renderAt('/')

  const steps4 = await card()
  fireEvent.change(steps4.getByLabelText('First weigh-in'), { target: { value: '72.5' } })
  fireEvent.click(steps4.getByRole('button', { name: 'Save the weigh-in' }))

  expect(await steps4.findByText('1 of 4')).toBeDefined()
  expect(steps()[1]).toBe('Done:First weigh-in|72.5 kg today')
  expect(await screen.findByText('Saved: Weight')).toBeDefined()
})

test('with the profile, a weigh-in and a call of the AI coach, each says what was given', async () => {
  const writeText = vi.fn<(s: string) => Promise<void>>(() => Promise.resolve())
  Object.defineProperty(navigator, 'clipboard', { value: { writeText }, configurable: true })
  fakeServer({
    ...newUser(),
    'GET /api/v1/me': () =>
      Response.json({
        ...me(),
        profile: { missing: [], sex: 'female', age: 34, height_cm: 166 },
      }),
    'GET /api/v1/summary': () =>
      Response.json(summaryOfNothing({ trend: { date: '2026-10-22', kg: 80.2 } })),
    'GET /api/v1/tokens': () => Response.json({ tokens: [used] }),
  })
  renderAt('/')

  const steps4 = await card()
  expect(await steps4.findByText('3 of 4')).toBeDefined()
  expect(steps()).toEqual([
    'Done:Your profile|Female · 34 years · 166 cm',
    'Done:First weigh-in|trend 80.2 kg',
    'Done:Your AI coach or a phone|Claude',
    'To do:Log your first meal|Search, a barcode or an estimate',
  ])
  // A step done is a tick, and no longer a link.
  expect(steps4.queryByRole('link', { name: /^Complete your profile/ })).toBeNull()
  Reflect.deleteProperty(navigator, 'clipboard')
})

test('what to ask the AI coach for a plan copies in a tap', async () => {
  const writeText = vi.fn<(s: string) => Promise<void>>(() => Promise.resolve())
  Object.defineProperty(navigator, 'clipboard', { value: { writeText }, configurable: true })
  fakeServer(newUser())
  renderAt('/')

  const coach = within(
    (await card()).getByRole('region', { name: 'Then: ask your AI coach for a plan' }),
  )
  const prompt =
    'Look at my profile and first weigh-ins in tdeemon and propose a phase and targets. Ask me what you need.'
  expect(coach.getByText(prompt)).toBeDefined()
  fireEvent.click(coach.getByRole('button', { name: 'Copy' }))
  await waitFor(() => {
    expect(writeText).toHaveBeenCalledWith(prompt)
  })
  expect(await coach.findByRole('button', { name: 'Copied' })).toBeDefined()
  expect(coach.getByRole('status').textContent).toBe('Copied. Paste it to your AI coach.')
  Reflect.deleteProperty(navigator, 'clipboard')
})

test('a browser that cannot copy selects the message, to copy by hand', async () => {
  Object.defineProperty(navigator, 'clipboard', {
    value: { writeText: () => Promise.reject(new Error('denied')) },
    configurable: true,
  })
  fakeServer(newUser())
  renderAt('/')

  const coach = within(
    (await card()).getByRole('region', { name: 'Then: ask your AI coach for a plan' }),
  )
  fireEvent.click(coach.getByRole('button', { name: 'Copy' }))
  expect((await coach.findByRole('alert')).textContent).toBe(
    "Couldn't copy. It's selected, so copy it by hand.",
  )
  expect(window.getSelection()?.toString()).toContain('Look at my profile')
  Reflect.deleteProperty(navigator, 'clipboard')
})

test('on a desktop, how tdeemon works is beside the first steps, in four rows', async () => {
  vi.stubGlobal('matchMedia', (query: string) => ({
    matches: query === '(min-width: 64rem)',
    addEventListener: () => undefined,
    removeEventListener: () => undefined,
  }))
  fakeServer(newUser())
  renderAt('/')

  await card()
  const how = within(screen.getByRole('region', { name: 'How tdeemon works' }))
  expect(how.getAllByRole('listitem').map((li) => li.textContent)).toEqual([
    'You log what you eatIn 2 taps: recent, a barcode, or an estimate.',
    'You weigh in most morningsThe trend, not a day, counts.',
    'tdeemon measures your TDEEFrom intake and the trend, with its ± interval.',
    'Your AI coach proposes; you decideIn your own assistant. Accept, reject or undo.',
  ])
})

test('a meal logged before counts, and a key never used does not', async () => {
  fakeServer({
    ...newUser(),
    'GET /api/v1/meals/recent': () =>
      Response.json({ meals: [{ ...meal, eaten: [{ at: meal.eaten_at, day: '2026-10-22' }] }] }),
    'GET /api/v1/tokens': () => Response.json({ tokens: [{ ...used, last_used_at: undefined }] }),
  })
  renderAt('/')

  const steps4 = await card()
  expect(await steps4.findByText('1 of 4')).toBeDefined()
  expect(steps()[2]).toBe(
    'To do:Connect your AI coach or a phone|The MCP address and a key, in 3 steps',
  )
  expect(steps()[3]).toBe('Done:Your first meal|Oats')
})

test('put off for later, the hero is back, until the plan shows the steps again', async () => {
  fakeServer(newUser())
  const router = renderAt('/')

  fireEvent.click((await card()).getByRole('button', { name: 'Later' }))
  expect(await screen.findByText('A plan gives the day its target.')).toBeDefined()
  expect(screen.queryByRole('region', { name: 'First steps' })).toBeNull()
  expect(screen.queryByRole('region', { name: 'How tdeemon works' })).toBeNull()

  fireEvent.click(
    within(screen.getByRole('navigation', { name: 'Sections' })).getByRole('link', {
      name: 'Plan',
    }),
  )
  fireEvent.click(await screen.findByRole('link', { name: 'Show the first steps' }))
  await waitFor(() => {
    expect(router.state.location.pathname).toBe('/')
  })
  expect(await card()).toBeDefined()
  expect(screen.queryByText('A plan gives the day its target.')).toBeNull()
})

test.each([
  ['a plan', { phases: [{ start: '2026-10-30', type: 'cut' as const, rate_pct_week: -0.5 }] }],
  [
    'a proposal that waits',
    {
      pending: {
        id: '0193c3a5-0000-7000-8000-0000000000d1',
        status: 'pending' as const,
        rationale: 'A first plan.',
        by: { client: 'mcp' as const },
        created_at: '2026-10-23T08:00:00Z',
        expires_at: '2026-10-30T08:00:00Z',
        changes: { phases: [], targets: [], limits: [] },
      },
    },
  ],
])('there are no first steps with %s', async (_, over) => {
  fakeServer({ ...newUser(), 'GET /api/v1/plan': () => Response.json(planBody(over)) })
  renderAt('/')

  expect(await screen.findByRole('region', { name: 'Eaten' })).toBeDefined()
  expect(screen.queryByRole('region', { name: 'First steps' })).toBeNull()
})

test('the first steps speak Croatian', async () => {
  fakeServer(newUser('hr'))
  renderAt('/')

  const steps4 = within(await screen.findByRole('region', { name: 'Prvi koraci' }))
  expect(steps4.getByText('0 od 4')).toBeDefined()
  expect(steps4.getByRole('link', { name: /^Dopuni profil/ })).toBeDefined()
  expect(steps4.getByRole('button', { name: 'Kasnije' })).toBeDefined()
  expect(
    steps4.getByRole('region', { name: 'Zatim: zatraži plan od svog AI trenera' }),
  ).toBeDefined()
})
