import { focusManager } from '@tanstack/react-query'
import { act, cleanup, fireEvent, screen, waitFor, within } from '@testing-library/react'
import { afterEach, beforeEach, expect, test, vi } from 'vitest'

import type { components } from '@/api/schema'
import { setLocale } from '@/i18n/locale'
import { fakeServer, me, renderAt, signedIn } from '@/test/app'
import { limitsBody, planBody } from '@/test/plan'

// The clock is the tests' day, so a date of another year says its year.
beforeEach(() => {
  vi.useFakeTimers({ toFake: ['Date'] })
  vi.setSystemTime(new Date('2026-10-23T16:00:00Z'))
})

afterEach(() => {
  cleanup()
  vi.useRealTimers()
  vi.unstubAllGlobals()
  setLocale('en')
})

type Plan = components['schemas']['PlanBody']
type Proposal = components['schemas']['ProposalBody']

const step: Proposal = {
  id: '0193c3a5-0000-7000-8000-0000000000a1',
  status: 'pending',
  rationale: "The loss is under the cut's rate.",
  by: { client: 'mcp', model: 'claude-opus-5-5' },
  window: { from: '2026-09-25', to: '2026-10-22' },
  changes: {
    phases: [],
    targets: [
      {
        from: '2026-10-24',
        after: {
          training: { kcal: 1900, protein: 160, carbs: 220, fat: 60 },
          rest: { kcal: 1900, protein: 160, carbs: 220, fat: 60 },
        },
      },
    ],
    limits: [],
  },
  created_at: '2026-10-23T08:00:00Z',
  expires_at: '2026-10-30T08:00:00Z',
}

const preview: components['schemas']['PreviewBody'] = {
  trend_kg: 82.5,
  tdee: 2385,
  checks: [
    {
      of: 'target',
      day: '2026-10-24',
      weekly_kcal: 1900,
      expected_pct_week: -0.53,
      expected_kg_week: -0.44,
      warnings: [],
      violations: [],
    },
  ],
}

// sundays is a revision applied, of the training weekdays.
const sundays: Proposal = {
  id: '0193c3a5-0000-7000-8000-0000000000b1',
  status: 'applied',
  rationale: 'Train on Sundays too.',
  by: { client: 'mcp', model: 'claude-opus-5-5' },
  changes: {
    phases: [],
    targets: [],
    training_weekdays: { before: ['monday'], after: ['sunday', 'monday'] },
    limits: [],
  },
  created_at: '2026-10-20T08:00:00Z',
  expires_at: '2026-10-27T08:00:00Z',
  decided_at: '2026-10-20T09:00:00Z',
  decided_by: { client: 'mcp', model: 'claude-opus-5-5' },
}

// refusal is the API's refusal of a proposal past the limits, with its
// violation as an error detail's value.
function refusal(): Response {
  return Response.json(
    {
      status: 422,
      title: 'Unprocessable Entity',
      detail: 'the proposal breaks the limits: kcal_floor (target 2026-10-24)',
      errors: [
        {
          location: 'target 2026-10-24',
          message: 'kcal_floor: 1800 proposed, the limit 1850 (user)',
          value: { rule: 'kcal_floor', limit: 1850, proposed: 1800, level: 'user' },
        },
      ],
    },
    { status: 422, headers: { 'Content-Type': 'application/problem+json' } },
  )
}

// rowsOf are the rows of the tables in el, each cell's words.
function rowsOf(el: HTMLElement): (string | null)[][] {
  return within(el)
    .getAllByRole('row')
    .map((row) => [...row.querySelectorAll('th, td')].map((cell) => cell.textContent))
}

// revisionAt opens the revision id on the plan's history, and returns it.
async function revisionAt(id: string, name = /, by /) {
  renderAt(`/plan/history?rev=${id}`)
  return screen.findByRole('region', { name })
}

function closed(status: number): Response {
  return Response.json(
    { status, title: 'An error', detail: 'the proposal is superseded: it waits no more' },
    { status, headers: { 'Content-Type': 'application/problem+json' } },
  )
}

// coachServer keeps a plan as the server does, and what it was sent, by
// "METHOD /path"; over answers a route its own way.
function coachServer(
  start: Partial<Plan>,
  over: { apply?: () => Response; revert?: () => Response } = {},
) {
  let plan = planBody(start)
  const sent: string[] = []
  const done = (r: Request, status: Proposal['status']) => {
    sent.push(`${r.method} ${new URL(r.url).pathname}`)
    const pending = plan.pending
    if (pending === undefined) {
      return closed(409)
    }
    const decided = {
      ...pending,
      status,
      decided_at: '2026-10-23T09:00:00Z',
      decided_by: { client: 'web' },
    }
    plan = {
      ...planBody({
        revisions: status === 'applied' ? [decided, ...plan.revisions] : plan.revisions,
      }),
    }
    return Response.json({ proposal: decided })
  }
  fakeServer({
    ...signedIn(),
    'GET /api/v1/profile': () => Response.json({ missing: [] }),
    'GET /api/v1/limits': () => Response.json(limitsBody()),
    'GET /api/v1/plan': () => Response.json(plan),
    [`POST /api/v1/proposals/${step.id}/apply`]: (r) =>
      over.apply ? over.apply() : done(r, 'applied'),
    [`POST /api/v1/proposals/${step.id}/reject`]: (r) => done(r, 'rejected'),
    [`POST /api/v1/proposals/${sundays.id}/revert`]: (r) => {
      sent.push(`${r.method} ${new URL(r.url).pathname}`)
      if (over.revert) {
        return over.revert()
      }
      plan = { ...plan, pending: { ...step, reverts: sundays.id } }
      return Response.json({ proposal: plan.pending, preview }, { status: 201 })
    },
  })
  return { sent }
}

test('a proposal that waits says why, what it changes and what it comes to, and is accepted', async () => {
  const { sent } = coachServer({ pending: step, pending_preview: preview })
  renderAt('/plan')
  const region = await screen.findByRole('region', { name: 'A proposed change' })
  const card = within(region)
  card.getByText('Claude')
  card.getByText('Proposed by Claude (claude-opus-5-5) · 8 hr. ago')
  card.getByText("The loss is under the cut's rate.")
  card.getByText('Drawn from the days Sep 25 to Oct 22.')
  // A new target, the same on both types of day: every day's, its macros
  // under its kcal.
  expect(rowsOf(region)).toEqual([
    ['From Oct 24', 'Now', 'New', 'Δ'],
    ['Every day kcalP 160 · C 220 · F 60 g', '—', '1,900', ''],
  ])
  card.getByText('expires in 7 days')
  // The buttons at its foot, last.
  expect(
    within(region)
      .getAllByRole('button')
      .map((b) => b.textContent),
  ).toEqual(['Accept', 'Reject'])

  fireEvent.click(card.getByRole('button', { name: 'Accept' }))
  await waitFor(() => {
    expect(screen.queryByRole('region', { name: 'A proposed change' })).toBeNull()
  })
  expect(sent).toEqual([`POST /api/v1/proposals/${step.id}/apply`])
  // The answer is said, and the plan has a revision now, a row to its
  // history.
  await screen.findByText('Proposal accepted')
  const row = await screen.findByRole('link', { name: 'Plan history' })
  expect(document.getElementById(row.getAttribute('aria-describedby') ?? '')?.textContent).toBe(
    '1 revision · Oct 23',
  )
})

test('a proposal that waits is rejected', async () => {
  const { sent } = coachServer({ pending: step })
  renderAt('/plan')
  const card = within(await screen.findByRole('region', { name: 'A proposed change' }))
  fireEvent.click(card.getByRole('button', { name: 'Reject' }))
  await waitFor(() => {
    expect(screen.queryByRole('region', { name: 'A proposed change' })).toBeNull()
  })
  expect(sent).toEqual([`POST /api/v1/proposals/${step.id}/reject`])
  await screen.findByText('Proposal rejected')
  // No plan yet, and no history of one.
  screen.getByRole('region', { name: 'No plan yet' })
  expect(screen.queryByRole('link', { name: 'Plan history' })).toBeNull()
})

test('a proposal the limits refuse now says why, in words', async () => {
  coachServer({ pending: step }, { apply: refusal })
  renderAt('/plan')
  const card = within(await screen.findByRole('region', { name: 'A proposed change' }))
  fireEvent.click(card.getByRole('button', { name: 'Accept' }))
  const alert = await card.findByRole('alert')
  expect(alert.textContent).toBe(
    'The limits refuse it now:the kcal of a day: 1,800 proposed, the limit 1,850: your limit, which you may loosen below',
  )
})

test('a proposal that waits no more says so', async () => {
  coachServer({ pending: step }, { apply: () => closed(409) })
  renderAt('/plan')
  const card = within(await screen.findByRole('region', { name: 'A proposed change' }))
  fireEvent.click(card.getByRole('button', { name: 'Accept' }))
  await card.findByText('This proposal is no longer open. The plan changed, or it expired.')
  expect(card.getByRole('button', { name: 'Accept' })).toHaveProperty('disabled', true)
})

test('the history shows a revision, which is taken back in a step', async () => {
  const { sent } = coachServer({ revisions: [sundays] })
  const region = await revisionAt(sundays.id)
  const history = within(region)
  history.getByRole('heading', { name: 'Oct 20, by Claude (claude-opus-5-5)' })
  history.getByText('Train on Sundays too.')
  expect(rowsOf(region)).toEqual([
    // Of the day it was applied, not today: 2 a.m. in Los Angeles, before
    // the user's day starts at 4, is the 19th's.
    ['From Oct 19', 'Before', 'After', 'Δ'],
    ['Training days', 'Mon', 'Sun, Mon', ''],
  ])

  fireEvent.click(history.getByRole('button', { name: 'Take back' }))
  history.getByText('Take this change back, from now on?')
  fireEvent.click(history.getByRole('button', { name: 'Take back' }))
  await waitFor(() => {
    expect(sent).toEqual([
      `POST /api/v1/proposals/${sundays.id}/revert`,
      `POST /api/v1/proposals/${step.id}/apply`,
    ])
  })
  // Said in a toast, with what it took back.
  const toast = await screen.findByText('Change taken back')
  expect(toast.parentElement?.textContent).toContain('Training days: Sun, Mon')
})

test('a change that took effect cannot be taken back, and says so', async () => {
  coachServer(
    { revisions: [sundays] },
    { revert: () => Response.json({ status: 422, detail: 'invalid proposal' }, { status: 422 }) },
  )
  const history = within(await revisionAt(sundays.id))
  fireEvent.click(history.getByRole('button', { name: 'Take back' }))
  fireEvent.click(history.getByRole('button', { name: 'Take back' }))
  expect((await history.findByRole('alert')).textContent).toBe(
    "This change can't be taken back anymore. It took effect, or the plan changed since. Ask for a new change.",
  )
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

// The history is a page of its own: on a phone its rows, each to its
// revision, which leads back to them.
test("the plan's history is a row to its page, of rows to each revision", async () => {
  const edited: Proposal = {
    ...sundays,
    id: '0193c3a5-0000-7000-8000-0000000000b2',
    rationale: 'changed in the app',
    by: { client: 'web' },
    decided_at: '2026-10-22T09:00:00Z',
  }
  coachServer({ revisions: [edited, { ...sundays, status: 'reverted' }] })
  const router = renderAt('/plan')
  fireEvent.click(await screen.findByRole('link', { name: 'Plan history' }))
  await screen.findByRole('heading', { name: 'Plan history', level: 1 })
  const rows = screen.getAllByRole('link', { name: /, by / })
  expect(rows.map((r) => r.textContent)).toEqual([
    'Oct 22, by youTraining days: Sun, Mon',
    'Oct 20, by Claude (claude-opus-5-5) · Taken backTraining days: Sun, Mon',
  ])
  fireEvent.click(rows[1] ?? document.body)
  const revision = within(await screen.findByRole('region', { name: /^Oct 20, by/ }))
  revision.getByText('Train on Sundays too.')
  // Taken back, it is taken back no more.
  expect(revision.queryByRole('button', { name: 'Take back' })).toBeNull()
  fireEvent.click(screen.getByRole('link', { name: 'All changes' }))
  await waitFor(() => {
    expect(router.state.location.search).toEqual({})
  })
  expect(screen.getAllByRole('link', { name: /, by / })).toHaveLength(2)
})

// On a desktop the rows stay beside the revision open, the latest at
// first; the plan has its latest revisions beside it.
test("on a desktop the history's rows stay beside the revision open", async () => {
  desktop()
  coachServer({ revisions: [sundays] })
  renderAt('/plan')
  const card = within(await screen.findByRole('region', { name: 'Plan history' }))
  expect(screen.queryByRole('link', { name: 'Plan history' })).toBeNull()
  fireEvent.click(card.getByRole('link', { name: '1 revision' }))
  const revision = await screen.findByRole('region', { name: /^Oct 20, by/ })
  within(revision).getByText('Train on Sundays too.')
  expect(screen.getByRole('link', { name: /^Oct 20, by/ }).getAttribute('aria-current')).toBe(
    'page',
  )
})

// On a desktop the keyboard answers a proposal: A accepts it, X rejects
// it; the header says so.
test('on a desktop A accepts a proposal and X rejects one', async () => {
  desktop()
  const { sent } = coachServer({ pending: step })
  renderAt('/plan')
  await screen.findByRole('region', { name: 'A proposed change' })
  const header = screen.getByRole('heading', { name: 'Plan', level: 1 }).parentElement
  expect(header?.querySelector('p')?.textContent).toBe('A proposal waits · A accept · X reject')
  fireEvent.keyDown(document.body, { key: 'x' })
  await waitFor(() => {
    expect(sent).toEqual([`POST /api/v1/proposals/${step.id}/reject`])
  })
  cleanup()

  const accepted = coachServer({ pending: step })
  renderAt('/plan')
  await screen.findByRole('region', { name: 'A proposed change' })
  // With Caps Lock on, as the key comes.
  fireEvent.keyDown(document.body, { key: 'A' })
  await waitFor(() => {
    expect(accepted.sent).toEqual([`POST /api/v1/proposals/${step.id}/apply`])
  })
})

// G, then A, is a way to a section the app does not have, not an
// answer.
test('on a desktop G then A or X does not answer a proposal', async () => {
  desktop()
  const { sent } = coachServer({ pending: step })
  renderAt('/plan')
  await screen.findByRole('region', { name: 'A proposed change' })
  fireEvent.keyDown(document.body, { key: 'g' })
  fireEvent.keyDown(document.body, { key: 'a' })
  fireEvent.keyDown(document.body, { key: 'g' })
  fireEvent.keyDown(document.body, { key: 'x' })
  await act(() => Promise.resolve())
  expect(sent).toEqual([])
})

// A revision past the latest the plan holds is said gone, not shown as
// another.
test('a revision the plan does not hold is said older, not another shown', async () => {
  desktop()
  coachServer({ revisions: [sundays] })
  renderAt('/plan/history?rev=0193c3a5-0000-7000-8000-00000000ffff')
  await screen.findByText('This change is older than the latest ones shown here.')
  expect(screen.queryByRole('region', { name: /^Oct 20, by/ })).toBeNull()
  expect(screen.getAllByRole('link', { name: /, by / })).toHaveLength(1)
})

// A phone has no such keys: a letter typed is no answer.
test("on a phone a letter doesn't answer a proposal", async () => {
  const { sent } = coachServer({ pending: step })
  renderAt('/plan')
  await screen.findByRole('region', { name: 'A proposed change' })
  fireEvent.keyDown(document.body, { key: 'a' })
  await act(() => Promise.resolve())
  expect(sent).toEqual([])
})

// A proposal that came in place of one that waits no more starts afresh:
// its buttons work.
test('a new proposal, in place of one that waits no more, can be answered', async () => {
  let plan = planBody({ pending: step })
  const smaller: Proposal = {
    ...step,
    id: '0193c3a5-0000-7000-8000-0000000000a2',
    rationale: 'A smaller step.',
  }
  fakeServer({
    ...signedIn(),
    'GET /api/v1/profile': () => Response.json({ missing: [] }),
    'GET /api/v1/limits': () => Response.json(limitsBody()),
    'GET /api/v1/plan': () => Response.json(plan),
    [`POST /api/v1/proposals/${step.id}/apply`]: () => {
      plan = planBody({ pending: smaller })
      return closed(409)
    },
  })
  renderAt('/plan')
  const card = within(await screen.findByRole('region', { name: 'A proposed change' }))
  fireEvent.click(card.getByRole('button', { name: 'Accept' }))
  await card.findByText('This proposal is no longer open. The plan changed, or it expired.')
  // The plan is read again, as when the app comes back to the front.
  act(() => {
    focusManager.setFocused(false)
    focusManager.setFocused(true)
  })
  await screen.findByText('A smaller step.')
  const fresh = within(screen.getByRole('region', { name: 'A proposed change' }))
  expect(fresh.queryByRole('alert')).toBeNull()
  expect(fresh.getByRole('button', { name: 'Accept' })).toHaveProperty('disabled', false)
  focusManager.setFocused(undefined)
})

// A proposal a day of which went by is refused for good: it says so, and
// may still be rejected.
test('a proposal that cannot be applied any more says so, and may be rejected', async () => {
  coachServer(
    { pending: step },
    { apply: () => Response.json({ status: 422, detail: 'invalid proposal' }, { status: 422 }) },
  )
  renderAt('/plan')
  const card = within(await screen.findByRole('region', { name: 'A proposed change' }))
  fireEvent.click(card.getByRole('button', { name: 'Accept' }))
  expect((await card.findByRole('alert')).textContent).toBe(
    "It can't be applied anymore, as it would now start before today. Ask for a new one.",
  )
  expect(card.getByRole('button', { name: 'Accept' })).toHaveProperty('disabled', true)
  expect(card.getByRole('button', { name: 'Reject' })).toHaveProperty('disabled', false)
})

// The latest answer is the one shown: a reject after a refused accept
// says what became of the reject.
test("an action's answer is its own, not the one before's", async () => {
  fakeServer({
    ...signedIn(),
    'GET /api/v1/profile': () => Response.json({ missing: [] }),
    'GET /api/v1/limits': () => Response.json(limitsBody()),
    'GET /api/v1/plan': () => Response.json(planBody({ pending: step })),
    [`POST /api/v1/proposals/${step.id}/apply`]: refusal,
    [`POST /api/v1/proposals/${step.id}/reject`]: () => closed(409),
  })
  renderAt('/plan')
  const card = within(await screen.findByRole('region', { name: 'A proposed change' }))
  fireEvent.click(card.getByRole('button', { name: 'Accept' }))
  await card.findByText('The limits refuse it now:')
  fireEvent.click(card.getByRole('button', { name: 'Reject' }))
  await card.findByText('This proposal is no longer open. The plan changed, or it expired.')
  expect(card.queryByText('The limits refuse it now:')).toBeNull()
  expect(card.getByRole('button', { name: 'Reject' })).toHaveProperty('disabled', true)
})

// A date of another year says its year: a phase made a year longer does
// not read as it was.
test("a change's date of another year says its year", async () => {
  const longer: Proposal = {
    ...step,
    changes: {
      phases: [
        {
          start: '2026-09-07',
          before: { type: 'cut', start: '2026-09-07', end: '2026-12-31', rate_pct_week: -0.5 },
          after: { type: 'cut', start: '2026-09-07', end: '2027-12-31', rate_pct_week: -0.5 },
        },
      ],
      targets: [],
      limits: [],
    },
  }
  coachServer({ pending: longer })
  renderAt('/plan')
  const card = within(await screen.findByRole('region', { name: 'A proposed change' }))
  expect(card.getByText('until Dec 31 → Dec 31, 2027')).toBeTruthy()
})

type Summary = components['schemas']['SummaryBody']

// summaryOf is calibration's summary of nothing measured yet; over is
// laid over it.
function summaryOf(over: Partial<Summary> = {}): Summary {
  return {
    date: '2026-10-23',
    incomplete_days: [],
    recovery: { flags: [] },
    tdee: {
      before_phase: false,
      doubts: ['few_days'],
      from: '2026-09-25',
      to: '2026-10-22',
      logged_days: 0,
      measured: false,
      trusted: false,
      weigh_ins: 0,
      needs: {
        days: { has: 0, need: 14 },
        logged_days: { has: 0, need: 7 },
        weigh_ins: { has: 0, need: 6 },
        weigh_in_days: { has: 0, need: 7 },
      },
      trust: { half_width: 200, weigh_ins: 3, logged_days: 4 },
    },
    verdict: {
      action: 'none',
      flags: [],
      gap_kcal: 0,
      guards: [],
      rate: 'none',
      reasons: ['no_estimate'],
      step_kcal: 0,
    },
    weeks: [],
    ...over,
  }
}

// planPage serves a plan, calibration's summary, the user's profile,
// and the trend of the days asked for, from trends by day.
function planPage(
  plan: Partial<Plan>,
  summary: Summary = summaryOf(),
  profile = me().profile,
  trends: Record<string, number> = {},
) {
  fakeServer({
    ...signedIn(),
    'GET /api/v1/me': () => Response.json({ ...me(), profile }),
    'GET /api/v1/limits': () => Response.json(limitsBody()),
    'GET /api/v1/plan': () => Response.json(planBody(plan)),
    'GET /api/v1/summary': () => Response.json(summary),
    'GET /api/v1/history': (r) => {
      const q = new URL(r.url).searchParams
      const from = q.get('from') ?? ''
      const to = q.get('to') ?? ''
      const days = Object.entries(trends)
        .filter(([date]) => date >= from && date <= to)
        .map(([date, kg]) => ({ date, trend_kg: kg, meals: 0, status: 'open', type: 'rest' }))
      return Response.json({ from, to, days, weeks: [] })
    },
  })
}

// cut is a cut of 14 weeks, a phase of maintenance before it, and one
// after; today, October 23, is in its seventh week.
const cut: Partial<Plan> = {
  phases: [
    { type: 'maintain', start: '2026-06-01', end: '2026-09-06', rate_pct_week: 0 },
    { type: 'cut', start: '2026-09-07', end: '2026-12-13', rate_pct_week: -0.5, goal_weight: 80 },
    { type: 'maintain', start: '2026-12-14', rate_pct_week: 0 },
  ],
  targets: [
    {
      effective_from: '2026-09-07',
      to: '2026-10-25',
      training: { kcal: 2300, protein: 160, carbs: 230, fat: 70 },
      rest: { kcal: 2000, protein: 160, carbs: 170, fat: 60 },
    },
    {
      effective_from: '2026-10-26',
      training: { kcal: 2200, protein: 160, carbs: 205, fat: 70 },
      rest: { kcal: 2200, protein: 160, carbs: 205, fat: 70 },
    },
  ],
  training_weekdays: ['monday', 'wednesday'],
}

test('a phase with a milestone and no goal shows the milestone', async () => {
  planPage(
    {
      ...cut,
      phases: [{ type: 'cut', start: '2026-09-07', rate_pct_week: -0.5, milestone_weight: 82 }],
    },
    summaryOf({ trend: { date: '2026-10-22', kg: 82.5 } }),
  )
  renderAt('/plan')
  const hero = within(await screen.findByRole('region', { name: 'Cut' }))
  expect(hero.getByText(/^Milestone/).textContent).toBe('Milestone 82 kg')
  expect(hero.queryByText(/^Goal/)).toBeNull()
})

// The phase of today is the plan's hero: its type, its pace and range,
// the rate measured, its week, how far it has come, and its goal; and
// the phases on a timeline.
test('the phase of today shows its pace, its week and the way to its goal, among the phases', async () => {
  planPage(
    {
      ...cut,
      phases: [
        { type: 'maintain', start: '2026-06-01', end: '2026-09-06', rate_pct_week: 0 },
        {
          type: 'cut',
          start: '2026-09-07',
          end: '2026-12-13',
          rate_pct_week: -0.5,
          goal_weight: 80,
          milestone_weight: 83.75,
        },
        { type: 'maintain', start: '2026-12-14', rate_pct_week: 0 },
      ],
    },
    summaryOf({
      trend: { date: '2026-10-22', kg: 82.5 },
      verdict: {
        ...summaryOf().verdict,
        rate: 'on_target',
        measured_pct_week: -0.48,
        target_pct_week: -0.5,
        tolerance_pct_week: 0.1,
      },
    }),
    me().profile,
    // The trend as the cut began, a day after its start.
    { '2026-09-08': 85, '2026-09-09': 84.9 },
  )
  renderAt('/plan')
  const region = await screen.findByRole('region', { name: 'Cut' })
  const hero = within(region)
  hero.getByText('Current phase · since Sep 7')
  hero.getByText('Week 7 of 14')
  expect((await hero.findByText('-0.5% a week')).parentElement?.textContent).toBe(
    '-0.5% a week · ≈ -0.41 kg a week',
  )
  expect(hero.getByText(/^Measured/).textContent).toBe(
    'Measured -0.48% a week, in range· target range -0.6% to -0.4%',
  )
  // From 85 kg as it began to the goal of 80: half the way, the
  // milestone a quarter of it.
  const way = hero.getByRole('progressbar', { name: 'The way to the goal' })
  await waitFor(() => {
    expect([way.getAttribute('value'), way.getAttribute('max')]).toEqual(['50', '100'])
  })
  expect(way.getAttribute('aria-valuetext')).toBe('trend 82.5 kg, 2.5 kg to go')
  const marks = [...region.querySelectorAll<HTMLElement>('[style]')].map((m) => m.style.left)
  expect(marks).toEqual(['50%', '25%'])
  hero.getByText('82.5 kg trend')
  hero.getByText('milestone 83.8 kg')
  hero.getByText('Goal 80 kg')

  const phases = within(hero.getByRole('list', { name: 'Phases' }))
    .getAllByRole('listitem')
    .map((li) => li.textContent)
  expect(phases).toEqual([
    'Maintenance · 0% a weekJun 1 – Sep 6Past',
    'Cut · -0.5% a weekSep 7 – Dec 13Now',
    'Maintenance · 0% a weekfrom Dec 14Planned',
  ])
})

// Without a goal, a phase with a planned end says how far it came in
// days.
test('a phase with an end and no goal shows its days', async () => {
  planPage({
    phases: [{ type: 'cut', start: '2026-09-07', end: '2026-12-13', rate_pct_week: -0.5 }],
  })
  renderAt('/plan')
  const hero = within(await screen.findByRole('region', { name: 'Cut' }))
  // 47 days gone of 98, today's included.
  const days = hero.getByRole('progressbar', { name: 'Days of the phase' })
  expect([days.getAttribute('value'), days.getAttribute('max')]).toEqual(['47', '98'])
  expect(days.getAttribute('aria-valuetext')).toBe('51 days left')
  hero.getByText('Sep 7 – Dec 13')
  expect(hero.queryByRole('progressbar', { name: 'The way to the goal' })).toBeNull()
})

test('a phase not measured yet says so, and one with no end its week alone', async () => {
  planPage({ phases: [{ type: 'bulk', start: '2026-10-12', rate_pct_week: 0.25 }] })
  renderAt('/plan')
  const hero = within(await screen.findByRole('region', { name: 'Bulk' }))
  hero.getByText('Week 2')
  hero.getByText('0.25% a week')
  expect(await hero.findByText('Not measured yet')).toBeTruthy()
  expect(hero.queryByRole('progressbar')).toBeNull()
  // A phase alone needs no timeline.
  expect(hero.queryByRole('list', { name: 'Phases' })).toBeNull()
})

// The targets: each type of day's, its days of the week, and the weekly
// average they come to against the TDEE; and what is to come.
test('the targets by type of day come to a weekly average, against the TDEE', async () => {
  planPage(
    cut,
    summaryOf({
      tdee: { ...summaryOf().tdee, measured: true, trusted: true, kcal: 2400.4 },
    }),
  )
  renderAt('/plan')
  const targets = within(await screen.findByRole('region', { name: 'Targets' }))
  targets.getByText('since Sep 7')
  const tile = (name: string) => targets.getByRole('heading', { name }).parentElement?.textContent
  expect(tile('Training')).toBe('Training2,300 kcalP 160 · C 230 · F 70 g2 days a week · Mon, Wed')
  expect(tile('Rest')).toBe(
    'Rest2,000 kcalP 160 · C 170 · F 60 g5 days a week · Tue, Thu, Fri, Sat, Sun',
  )
  // (2 × 2,300 + 5 × 2,000) / 7.
  const term = (name: string) => targets.getByText(name).nextElementSibling?.textContent
  expect(term('Weekly average')).toBe('2,086 kcal a day')
  await waitFor(() => {
    expect(term('TDEE')).toBe('2,400 kcal a day')
  })
  // A deficit in words, beside a number never below 0.
  expect(term('Planned deficit')).toBe('315 kcal a day, weekly average')
  expect(targets.getByText('From Oct 26:').parentElement?.textContent).toBe(
    'From Oct 26: 2,200 kcal · P 160 g · C 205 g · F 70 g',
  )
  // The TDEE and the deficit, each an ⓘ away from what it is.
  expect(targets.getByRole('button', { name: 'What TDEE is' })).toBeDefined()
  fireEvent.click(targets.getByRole('button', { name: 'What the planned deficit is' }))
  expect((await screen.findByRole('dialog', { name: 'Planned deficit' })).textContent).toContain(
    'A deficit of about 500 kcal a day is about half a kilo a week.',
  )
})

test('a TDEE not trusted yet is about so much, and one not measured says so', async () => {
  planPage(
    cut,
    summaryOf({ tdee: { ...summaryOf().tdee, measured: true, trusted: false, kcal: 1900 } }),
  )
  renderAt('/plan')
  const targets = within(await screen.findByRole('region', { name: 'Targets' }))
  const term = (name: string) => targets.getByText(name).nextElementSibling?.textContent
  await waitFor(() => {
    expect(term('TDEE')).toBe('≈1,900 kcal a day')
  })
  expect(term('Planned surplus')).toBe('186 kcal a day, weekly average')
  cleanup()

  planPage(cut)
  renderAt('/plan')
  const without = within(await screen.findByRole('region', { name: 'Targets' }))
  await waitFor(() => {
    expect(without.getByText('TDEE').nextElementSibling?.textContent).toBe('not measured yet')
  })
  expect(without.queryByText('Planned deficit')).toBeNull()
})

// A plan that starts tomorrow is all to come.
test('a plan that starts tomorrow is one to come', async () => {
  planPage({
    phases: [{ type: 'cut', start: '2026-10-24', rate_pct_week: -0.4 }],
    targets: [
      {
        effective_from: '2026-10-24',
        training: { kcal: 2400, protein: 200, carbs: 231, fat: 75 },
        rest: { kcal: 2400, protein: 200, carbs: 231, fat: 75 },
      },
    ],
  })
  renderAt('/plan')
  const hero = within(await screen.findByRole('region', { name: 'No phase now' }))
  hero.getByText('Next: Cut, from Oct 24')
  expect(within(hero.getByRole('list', { name: 'Phases' })).getByRole('listitem').textContent).toBe(
    'Cut · -0.4% a weekfrom Oct 24Planned',
  )
  const targets = within(screen.getByRole('region', { name: 'Targets' }))
  targets.getByText('No target yet.')
  expect(targets.getByText('From Oct 24:').parentElement?.textContent).toBe(
    'From Oct 24: 2,400 kcal · P 200 g · C 231 g · F 75 g',
  )
})

test('no plan yet says how one comes, and the first step to it', async () => {
  planPage({}, summaryOf(), { missing: ['height_cm'] })
  const router = renderAt('/plan')
  const empty = within(await screen.findByRole('region', { name: 'No plan yet' }))
  empty.getByText(
    'Your AI coach proposes one: ask it to "look at the last month and propose a plan". Its proposal waits here for your answer.',
  )
  // A plan needs the profile first.
  const first = await empty.findByRole('link', { name: 'Complete your profile' })
  // The coach's link asks for a token it can use: MCP takes no other.
  expect(empty.getAllByRole('link').map((a) => a.getAttribute('href'))).toEqual([
    '/plan/profile',
    '/settings/devices?connect=coach',
  ])
  expect(screen.queryByRole('region', { name: 'Targets' })).toBeNull()
  fireEvent.click(first)
  await screen.findByRole('heading', { name: 'Profile', level: 1 })
  expect(router.state.location.pathname).toBe('/plan/profile')
  cleanup()

  // With the profile whole, the coach is the first step.
  planPage({})
  renderAt('/plan')
  const whole = within(await screen.findByRole('region', { name: 'No plan yet' }))
  expect(whole.getAllByRole('link').map((a) => a.textContent)).toEqual(['Connect an AI coach'])
})

// Training days alone are a plan's part too: they give each day its type.
test('a plan of training days alone shows them', async () => {
  planPage({ training_weekdays: ['monday', 'wednesday'] })
  renderAt('/plan')
  const targets = within(await screen.findByRole('region', { name: 'Targets' }))
  expect(targets.getByText('Training days:').parentElement?.textContent).toBe(
    'Training days: Mon, Wed',
  )
  targets.getByText('No target yet.')
})

test('a plan loading is a skeleton of its shape, and one that did not come says why', async () => {
  let answer: ((r: Response) => void) | undefined
  let fails = true
  fakeServer({
    ...signedIn(),
    'GET /api/v1/limits': () => Response.json(limitsBody()),
    'GET /api/v1/plan': () =>
      fails
        ? new Promise<Response>((resolve) => {
            answer = resolve
          })
        : Response.json(planBody(cut)),
  })
  renderAt('/plan')
  await screen.findByRole('heading', { name: 'Plan', level: 1 })
  expect(screen.getByText('Loading…')).toBeTruthy()
  await waitFor(() => {
    expect(answer).toBeDefined()
  })
  answer?.(
    Response.json(
      { status: 500, title: 'An error', detail: 'what went wrong' },
      { status: 500, headers: { 'Content-Type': 'application/problem+json' } },
    ),
  )
  const alert = await screen.findByRole('alert')
  expect(within(alert).getByText("Couldn't load your plan.")).toBeTruthy()
  expect(within(alert).getByText('The server failed to answer.')).toBeTruthy()
  fails = false
  fireEvent.click(screen.getByRole('button', { name: 'Try again' }))
  expect(await screen.findByRole('region', { name: 'Cut' })).toBeTruthy()
})

// faster is a proposal of a faster cut's limit, loosened, and of more
// protein, tightened.
const faster: Proposal = {
  ...step,
  rationale: 'A faster cut, with more protein.',
  changes: {
    phases: [],
    targets: [],
    limits: [
      { name: 'cut_rate', before: -0.7, after: -1, loosens: true },
      { name: 'protein_floor', before: 1.6, after: 1.8, loosens: false },
    ],
  },
}

// limitsServer serves a plan with pending, and keeps the body of each
// apply sent.
function limitsServer(pending: Proposal) {
  const applied: string[] = []
  fakeServer({
    ...signedIn(),
    'GET /api/v1/profile': () => Response.json({ missing: [] }),
    'GET /api/v1/limits': () => Response.json(limitsBody()),
    'GET /api/v1/plan': () => Response.json(planBody({ pending })),
    [`POST /api/v1/proposals/${pending.id}/apply`]: async (r) => {
      applied.push(await r.text())
      return Response.json({ proposal: { ...pending, status: 'applied' } })
    },
  })
  return { applied }
}

test('a proposal of the limits shows each, loosened or tightened, and asks before it loosens one', async () => {
  const { applied } = limitsServer(faster)
  renderAt('/plan')
  const region = await screen.findByRole('region', { name: 'A proposed change' })
  const card = within(region)
  expect(rowsOf(region)).toEqual([
    ['From Oct 23', 'Now', 'New', 'Δ'],
    ['Fastest loss in a cut, BMI under 30', '0.7% a week', '1% a week', 'loosened'],
    ['Least protein', '1.6 g/kg', '1.8 g/kg', 'tightened'],
  ])
  card.getByText(
    'A change of your limits is accepted only here, in the app, never by your AI coach.',
  )

  // Asked first, the user may go back: nothing is sent.
  fireEvent.click(card.getByRole('button', { name: 'Accept' }))
  card.getByText(
    'Loosening Fastest loss in a cut, BMI under 30 allows a plan the limit refused. Loosen it?',
  )
  fireEvent.click(card.getByRole('button', { name: 'Back' }))
  expect(card.queryByText(/^Loosening/)).toBeNull()
  expect(applied).toEqual([])

  fireEvent.click(card.getByRole('button', { name: 'Accept' }))
  fireEvent.click(card.getByRole('button', { name: 'Loosen' }))
  await waitFor(() => {
    expect(applied).toEqual(['{"confirm":true}'])
  })
})

// Rejected while the question is open, it is rejected alone: the
// question goes, so no apply follows the reject.
test('a proposal rejected while the loosening is asked closes the question', async () => {
  const { applied } = limitsServer(faster)
  renderAt('/plan')
  const card = within(await screen.findByRole('region', { name: 'A proposed change' }))
  fireEvent.click(card.getByRole('button', { name: 'Accept' }))
  card.getByRole('button', { name: 'Loosen' })
  fireEvent.click(card.getByRole('button', { name: 'Reject' }))
  expect(card.queryByRole('button', { name: 'Loosen' })).toBeNull()
  expect(applied).toEqual([])
})

test('a proposal that only tightens the limits is accepted without a question', async () => {
  const tighter: Proposal = {
    ...faster,
    changes: { ...faster.changes, limits: faster.changes.limits.slice(1) },
  }
  const { applied } = limitsServer(tighter)
  renderAt('/plan')
  const card = within(await screen.findByRole('region', { name: 'A proposed change' }))
  fireEvent.click(card.getByRole('button', { name: 'Accept' }))
  await waitFor(() => {
    expect(applied).toEqual([''])
  })
  expect(card.queryByText(/^Loosening/)).toBeNull()
})

// A limit the limits check refuses is named as the page names it.
test("a limit past the instance's is said in words", async () => {
  fakeServer({
    ...signedIn(),
    'GET /api/v1/profile': () => Response.json({ missing: [] }),
    'GET /api/v1/limits': () => Response.json(limitsBody()),
    'GET /api/v1/plan': () =>
      Response.json(
        planBody({
          pending: faster,
          pending_preview: {
            checks: [
              {
                of: 'limits',
                day: '2026-10-23',
                warnings: [],
                violations: [
                  { rule: 'kcal_floor_male', limit: 1200, proposed: 1100, level: 'instance' },
                ],
              },
            ],
          },
        }),
      ),
  })
  renderAt('/plan')
  const card = within(await screen.findByRole('region', { name: 'A proposed change' }))
  expect(
    card.getByText(
      "Fewest kcal of a day, for a man: 1,100 proposed, the limit 1,200: the instance's limit",
    ),
  ).toBeTruthy()
})

// takeBackServer serves the revision tightened, whose take-back is back,
// and keeps the body of each apply sent.
function takeBackServer(tightened: Proposal, back: Proposal) {
  const applied: string[] = []
  fakeServer({
    ...signedIn(),
    'GET /api/v1/profile': () => Response.json({ missing: [] }),
    'GET /api/v1/limits': () => Response.json(limitsBody()),
    'GET /api/v1/plan': () => Response.json(planBody({ revisions: [tightened] })),
    [`POST /api/v1/proposals/${tightened.id}/revert`]: () =>
      Response.json({ proposal: back, preview: { checks: [] } }, { status: 201 }),
    [`POST /api/v1/proposals/${back.id}/apply`]: async (r) => {
      applied.push(await r.text())
      return Response.json({ proposal: { ...back, status: 'applied' } })
    },
  })
  return { applied }
}

const tightened: Proposal = {
  ...sundays,
  rationale: 'More protein.',
  changes: {
    phases: [],
    targets: [],
    limits: [{ name: 'protein_floor', before: 1.6, after: 1.8, loosens: false }],
  },
}

// Taking back a revision that tightened a limit loosens it: the question
// says so, and the confirmation goes with it.
test('taking back a tightened limit says it loosens it, and confirms it', async () => {
  const back: Proposal = {
    ...step,
    reverts: tightened.id,
    changes: {
      phases: [],
      targets: [],
      limits: [{ name: 'protein_floor', before: 1.8, after: 1.6, loosens: true }],
    },
  }
  const { applied } = takeBackServer(tightened, back)
  const region = await revisionAt(tightened.id)
  const history = within(region)
  expect(rowsOf(region)).toContainEqual(['Least protein', '1.6 g/kg', '1.8 g/kg', 'tightened'])
  fireEvent.click(history.getByRole('button', { name: 'Take back' }))
  history.getByText('Taking it back loosens Least protein, which allows a plan the limit refused.')
  fireEvent.click(history.getByRole('button', { name: 'Take back' }))
  await waitFor(() => {
    expect(applied).toEqual(['{"confirm":true}'])
  })
})

// A take-back that loosens more than the question said is not confirmed:
// it waits, to be asked about as any proposal.
test('a take-back that loosens what the question did not say is not confirmed', async () => {
  const back: Proposal = {
    ...step,
    reverts: tightened.id,
    changes: {
      phases: [],
      targets: [],
      limits: [
        { name: 'cut_rate', before: -0.7, after: -1, loosens: true },
        { name: 'protein_floor', before: 1.8, after: 1.6, loosens: true },
      ],
    },
  }
  const { applied } = takeBackServer(tightened, back)
  const history = within(await revisionAt(tightened.id))
  fireEvent.click(history.getByRole('button', { name: 'Take back' }))
  fireEvent.click(history.getByRole('button', { name: 'Take back' }))
  await waitFor(() => {
    expect(applied).toEqual([''])
  })
})

// splitTarget is a plan's target from before the proposals, split: rest
// days of 1,950 kcal, and carbs with a decimal, which none shows.
const splitTarget: Plan['targets'][number] = {
  effective_from: '2026-09-07',
  training: { kcal: 2250, protein: 165, carbs: 251.3, fat: 65 },
  rest: { kcal: 1950, protein: 160, carbs: 197, fat: 58 },
}

// fewerOnRest proposes 100 kcal fewer on rest days, from tomorrow.
const fewerOnRest: Proposal = {
  ...step,
  changes: {
    phases: [],
    targets: [
      {
        from: '2026-10-24',
        after: {
          training: splitTarget.training,
          rest: { ...splitTarget.rest, kcal: 1850, carbs: 172 },
        },
      },
    ],
    limits: [],
  },
}

test('a proposal says only what it changes, from what it was to what it is to be', async () => {
  planPage({ targets: [splitTarget], training_weekdays: ['monday'], pending: fewerOnRest })
  renderAt('/plan')
  const region = await screen.findByRole('region', { name: 'A proposed change' })
  // Each type of day's kcal, and under it only the macros it changes:
  // not the training days' carbs, whose decimal none shows.
  expect(rowsOf(region)).toEqual([
    ['From Oct 24', 'Now', 'New', 'Δ'],
    ['Training kcal', '2,250', '2,250', '—'],
    ['Rest kcalC 197 → 172 g', '1,950', '1,850', '-100'],
  ])
  expect(within(region).queryByText(/251/)).toBeNull()
})

// A target of a later day is told against the one the proposal sets
// before it.
test('a second period is told against the first', async () => {
  const lower = (kcal: number) => ({
    training: { ...splitTarget.training, kcal: kcal + 300 },
    rest: { ...splitTarget.rest, kcal },
  })
  planPage({
    targets: [splitTarget],
    training_weekdays: ['monday'],
    pending: {
      ...step,
      changes: {
        phases: [],
        targets: [
          { from: '2026-11-07', after: lower(1800) },
          { from: '2026-10-24', after: lower(1900) },
        ],
        limits: [],
      },
    },
  })
  renderAt('/plan')
  const region = await screen.findByRole('region', { name: 'A proposed change' })
  expect(rowsOf(region)).toEqual([
    ['From Oct 24', 'Now', 'New', 'Δ'],
    ['Training kcal', '2,250', '2,200', '-50'],
    ['Rest kcal', '1,950', '1,900', '-50'],
    ['From Nov 7', 'Now', 'New', 'Δ'],
    ['Training kcal', '2,200', '2,100', '-100'],
    ['Rest kcal', '1,900', '1,800', '-100'],
  ])
})

test('a change of a phase says each value it changes', async () => {
  const later: Proposal = {
    ...step,
    changes: {
      phases: [
        {
          start: '2026-09-07',
          before: { type: 'cut', start: '2026-09-07', rate_pct_week: -0.5, goal_weight: 80 },
          after: {
            type: 'cut',
            start: '2026-09-07',
            end: '2026-12-13',
            rate_pct_week: -0.7,
            goal_weight: 78,
          },
        },
      ],
      targets: [],
      limits: [],
    },
  }
  coachServer({ pending: later })
  renderAt('/plan')
  const card = within(await screen.findByRole('region', { name: 'A proposed change' }))
  card.getByText('Phase from Sep 7')
  expect(
    card.getByText('-0.5% → -0.7% a week · no end → until Dec 13 · goal 80 → 78 kg'),
  ).toBeTruthy()
})

test('what the data say stands beside the proposal, and says when they ask for no change', async () => {
  planPage(
    { targets: [splitTarget], training_weekdays: ['monday'], pending: fewerOnRest },
    summaryOf({
      tdee: {
        ...summaryOf().tdee,
        measured: true,
        trusted: true,
        kcal: 2400,
        half_width: 40,
        doubts: [],
      },
      verdict: {
        ...summaryOf().verdict,
        rate: 'on_target',
        measured_pct_week: -0.55,
        target_pct_week: -0.5,
        tolerance_pct_week: 0.1,
      },
    }),
  )
  renderAt('/plan')
  const card = within(await screen.findByRole('region', { name: 'A proposed change' }))
  expect((await card.findByText('The data:')).parentElement?.textContent).toBe(
    'The data: TDEE 2,400 ± 40 kcal · pace -0.55% a week, in range (-0.6% to -0.4%)',
  )
  expect(
    card.getByText("The pace is in range, so the data alone don't ask for this change."),
  ).toBeTruthy()
})

test('the data that ask the other way say so, and a TDEE not trusted is about so much', async () => {
  planPage(
    { targets: [splitTarget], training_weekdays: ['monday'], pending: fewerOnRest },
    summaryOf({
      tdee: { ...summaryOf().tdee, measured: true, kcal: 2400 },
      verdict: {
        ...summaryOf().verdict,
        rate: 'below',
        measured_pct_week: -0.8,
        target_pct_week: -0.5,
        tolerance_pct_week: 0.1,
        action: 'raise_kcal',
        gap_kcal: 150,
      },
    }),
  )
  renderAt('/plan')
  const card = within(await screen.findByRole('region', { name: 'A proposed change' }))
  expect((await card.findByText('The data:')).parentElement?.textContent).toBe(
    'The data: TDEE ≈ 2,400 kcal, not reliable yet · pace -0.8% a week, below range (-0.6% to -0.4%)',
  )
  expect(
    card.getByText('The data point the other way: they ask for more kcal, not fewer.'),
  ).toBeTruthy()
})

test("a limit's warning is said in plain words, with why it matters", async () => {
  planPage({
    pending: step,
    pending_preview: {
      checks: [
        {
          of: 'target',
          day: '2026-10-24',
          weekly_kcal: 1900,
          warnings: [
            { rule: 'low_energy_availability', day_type: 'rest', value: 27, threshold: 30 },
          ],
          violations: [],
        },
      ],
    },
  })
  renderAt('/plan')
  const card = within(await screen.findByRole('region', { name: 'A proposed change' }))
  const row = card.getByRole('button', { name: /guardrail warning/ })
  expect(row.textContent).toBe(
    '1 guardrail warning · rest daysLow energy for your lean mass (27 kcal/kg, at least 30 advised). Over time it can tire you and slow recovery.',
  )
  // Opened, each where it is found.
  expect(row.getAttribute('aria-expanded')).toBe('false')
  fireEvent.click(row)
  expect(row.getAttribute('aria-expanded')).toBe('true')
  expect(card.getByText('From Oct 24:').parentElement?.textContent).toBe(
    'From Oct 24: Rest days: low energy for your lean mass (27 kcal/kg, at least 30 advised). Over time it can tire you and slow recovery.',
  )
})

// below is a warning of a target below the resting burn.
function below(day_type: 'training' | 'rest', value: number) {
  return { rule: 'below_bmr' as const, day_type, value, threshold: 1830 }
}

// A period the proposal deletes is not the one a later period is told
// against: the target before it is.
test('a period after one deleted is told against the target before it', async () => {
  const lower = (kcal: number) => ({
    training: { ...splitTarget.training, kcal: kcal + 300 },
    rest: { ...splitTarget.rest, kcal },
  })
  const deleted = { effective_from: '2026-10-26', ...lower(1700) }
  planPage({
    targets: [splitTarget, deleted],
    training_weekdays: ['monday'],
    pending: {
      ...step,
      changes: {
        phases: [],
        targets: [
          { from: '2026-10-26', before: lower(1700) },
          { from: '2026-11-07', after: lower(1800) },
        ],
        limits: [],
      },
    },
  })
  renderAt('/plan')
  const region = await screen.findByRole('region', { name: 'A proposed change' })
  expect(rowsOf(region).slice(3)).toEqual([
    ['From Nov 7', 'Now', 'New', 'Δ'],
    ['Training kcal', '2,250', '2,100', '-150'],
    ['Rest kcal', '1,950', '1,800', '-150'],
  ])
})

// A rule the limits warn of on both types of day, in two periods, is
// said once, with the values from the least to the most (U2 PL1).
test('a warning of both types of day and two periods is said once', async () => {
  planPage({
    pending: step,
    pending_preview: {
      checks: [
        {
          of: 'target',
          day: '2026-10-24',
          warnings: [below('training', 1700), below('rest', 1600)],
          violations: [],
        },
        {
          of: 'target',
          day: '2026-11-07',
          warnings: [below('training', 1650), below('rest', 1600)],
          violations: [],
        },
      ],
    },
  })
  renderAt('/plan')
  const card = within(await screen.findByRole('region', { name: 'A proposed change' }))
  expect(card.getAllByText(/resting burn/i)).toHaveLength(1)
  expect(card.getByRole('button', { name: /guardrail warning/ }).textContent).toBe(
    '1 guardrail warning · both day types, both periodsBelow your resting burn (1,600–1,700 kcal, BMR about 1,830). Fine for a while, but hard to keep up.',
  )
  fireEvent.click(card.getByRole('button', { name: /guardrail warning/ }))
  expect(card.getAllByText(/resting burn/i)).toHaveLength(5)
})

test('a proposal that waits marks the plan in the sections, and none does not', async () => {
  coachServer({ pending: step })
  renderAt('/plan')
  const nav = within(await screen.findByRole('navigation', { name: 'Sections' }))
  expect(
    await nav.findByRole('link', { name: 'Plan', description: 'A proposal waits' }),
  ).toBeTruthy()
  cleanup()
  coachServer({})
  renderAt('/plan')
  await screen.findByRole('region', { name: 'No plan yet' })
  const plan = within(screen.getByRole('navigation', { name: 'Sections' })).getByRole('link', {
    name: 'Plan',
  })
  expect(plan.getAttribute('aria-describedby')).toBeNull()
})

test('today says who proposes how many changes, what they are, and leads to them', async () => {
  planPage({ targets: [splitTarget], training_weekdays: ['monday'], pending: fewerOnRest })
  const router = renderAt('/')
  const line = await screen.findByRole('link', { name: /^Claude proposes 1 plan change / })
  expect(line.textContent).toMatch(
    /^Claude proposes 1 plan change Rest days 1,950 → 1,850 kcal · expires in \d+ days?$/,
  )
  // In the coach's colour, as what an AI coach did.
  expect(line.className).toContain('bg-coach-soft')
  fireEvent.click(line)
  await waitFor(() => {
    expect(router.state.location.pathname).toBe('/plan')
  })
})

// In Croatian the count takes its case: predlaže 1 promjenu, 2 promjene,
// 5 promjena.
test("in Croatian today's banner counts the changes in their case", async () => {
  const two: Proposal = {
    ...fewerOnRest,
    changes: {
      ...fewerOnRest.changes,
      training_weekdays: { before: ['monday'], after: ['monday', 'thursday'] },
    },
  }
  fakeServer({
    ...signedIn('hr'),
    'GET /api/v1/plan': () =>
      Response.json(
        planBody({ targets: [splitTarget], training_weekdays: ['monday'], pending: two }),
      ),
  })
  renderAt('/')
  const line = await screen.findByRole('link', { name: /^Claude predlaže 2 promjene plana / })
  expect(line.textContent).toMatch(
    /^Claude predlaže 2 promjene plana Dani odmora .* · Dani treninga: /,
  )
})

test('in Croatian the proposal says who made it, when, and how long it waits', async () => {
  fakeServer({
    ...signedIn('hr'),
    'GET /api/v1/limits': () => Response.json(limitsBody()),
    'GET /api/v1/plan': () =>
      Response.json(
        planBody({ targets: [splitTarget], training_weekdays: ['monday'], pending: fewerOnRest }),
      ),
  })
  renderAt('/plan')
  const card = within(await screen.findByRole('region', { name: 'Prijedlog promjene' }))
  card.getByText('Predložio Claude (claude-opus-5-5) · prije 8 h')
  card.getByText('istječe za 7 dana')
  const rows = rowsOf(screen.getByRole('region', { name: 'Prijedlog promjene' }))
  expect(rows[0]).toEqual(['Od 24. lis', 'Sad', 'Novo', 'Δ'])
  expect(rows.map((r) => r.slice(0, 3))).toContainEqual([
    'Odmor kcalUH 197 → 172 g',
    '1.950',
    '1.850',
  ])
})

// The app's own changes have a code for their rationale, said in the
// language of whoever reads them; one stored in a language before the
// codes is said in the language too.
test("a revision made or taken back in the app is said in the reader's language", async () => {
  const edited: Proposal = {
    ...sundays,
    id: '0193c3a5-0000-7000-8000-0000000000b2',
    rationale: 'changed in the app',
    by: { client: 'web' },
  }
  const before: Proposal = { ...sundays, status: 'reverted', rationale: 'Vraćeno u aplikaciji.' }
  coachServer({ revisions: [edited, before] })
  const yours = within(await revisionAt(edited.id))
  yours.getByRole('heading', { name: 'Oct 20, by you' })
  yours.getByText('Changed in the app.')
  cleanup()
  const back = within(await revisionAt(before.id))
  back.getByText('Taken back in the app.')
  expect(back.queryByText('Vraćeno u aplikaciji.')).toBeNull()
})

test('a revision is taken back with a code, in any language', async () => {
  let sent: unknown
  fakeServer({
    ...signedIn('hr'),
    'GET /api/v1/limits': () => Response.json(limitsBody()),
    'GET /api/v1/plan': () => Response.json(planBody({ revisions: [sundays] })),
    [`POST /api/v1/proposals/${sundays.id}/revert`]: async (r) => {
      sent = await r.json()
      return Response.json({ proposal: { ...step, reverts: sundays.id }, preview }, { status: 201 })
    },
    [`POST /api/v1/proposals/${step.id}/apply`]: () =>
      Response.json({ proposal: { ...step, status: 'applied' } }),
  })
  const history = within(await revisionAt(sundays.id, /claude-opus-5-5/))
  fireEvent.click(history.getByRole('button', { name: 'Vrati' }))
  fireEvent.click(history.getByRole('button', { name: 'Vrati' }))
  await waitFor(() => {
    expect(sent).toEqual({ rationale: 'taken back in the app' })
  })
})
