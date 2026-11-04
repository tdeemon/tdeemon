import { expect, test, type Page } from '@playwright/test'

import { signIn } from './user.ts'

// propose gives the signed-in user what a plan needs (a profile and a
// weigh-in) and a proposal of a target from tomorrow, as a coach would
// make it through MCP; it returns the proposal's rationale.
async function propose(page: Page): Promise<string> {
  const api = page.request
  expect(
    (
      await api.put('/api/v1/profile', {
        data: { sex: 'male', birth_date: '1990-06-15', height_cm: 180 },
      })
    ).ok(),
  ).toBe(true)
  expect((await api.put('/api/v1/days/today/metrics/weight', { data: { value: 85 } })).ok()).toBe(
    true,
  )
  const me: unknown = await (await api.get('/api/v1/me')).json()
  const today = typeof me === 'object' && me !== null && 'today' in me ? String(me.today) : ''
  const tomorrow = new Date(`${today}T00:00:00Z`)
  tomorrow.setUTCDate(tomorrow.getUTCDate() + 1)
  const rationale = 'A first target, made up for the end-to-end tests.'
  const made = await api.post('/api/v1/proposals', {
    data: {
      rationale,
      model: 'claude-opus-5-5',
      targets: [
        {
          from: tomorrow.toISOString().slice(0, 10),
          training: { kcal: 2100, protein: 150, fat: 60 },
        },
      ],
    },
  })
  expect(made.status()).toBe(201)
  return rationale
}

test('a proposed change is accepted in the app, and taken back from its history', async ({
  page,
}) => {
  await signIn(page)
  const rationale = await propose(page)
  // The day says it waits, in the coach's colour, and so does the plan's
  // section.
  await page.reload()
  await expect(
    page.getByRole('link', { name: /^Claude proposes 1 plan change New target/ }),
  ).toBeVisible()
  // The progress marks it on the chart it cites, the intake's, and leads
  // to the plan from there.
  await page
    .getByRole('navigation', { name: 'Sections' })
    .getByRole('link', { name: 'Progress' })
    .click()
  const tdee = page.getByRole('region', { name: 'TDEE and intake' })
  await expect(tdee.locator('[role="img"] svg')).toContainText('Proposal')
  await tdee.getByRole('link', { name: /^Claude proposes 1 plan change New target/ }).click()
  await expect(page).toHaveURL('/plan')
  const plan = page
    .getByRole('navigation', { name: 'Sections' })
    .getByRole('link', { name: 'Plan' })
  await expect(plan).toHaveAccessibleDescription('A proposal waits')
  await plan.click()

  const card = page.getByRole('region', { name: 'A proposed change' })
  await expect(card.getByText(rationale)).toBeVisible()
  await expect(card.getByText(/^Proposed by Claude \(claude-opus-5-5\) · /)).toBeVisible()
  await expect(card.getByText('expires in 7 days')).toBeVisible()
  // What it changes, now → new.
  await expect(card.getByRole('row', { name: /^Every day kcal .* 2,100/ })).toBeVisible()
  await card.getByRole('button', { name: 'Accept' }).click()
  await expect(card).toBeHidden()
  await expect(page.getByText('Proposal accepted', { exact: true })).toBeVisible()
  // The plan has its target, from tomorrow.
  const targets = page.getByRole('region', { name: 'Targets', exact: true })
  await expect(targets.getByText('No target yet.')).toBeVisible()
  await expect(
    targets.getByRole('listitem').filter({ hasText: /^From .*: 2,100 kcal/ }),
  ).toBeVisible()

  // Its history, a page of its own, takes it back.
  await page.getByRole('link', { name: 'Plan history' }).click()
  await page
    .getByRole('link', { name: /, by Claude/ })
    .first()
    .click()
  const revision = page.getByRole('region', { name: /, by Claude/ })
  await expect(revision.getByText(rationale)).toBeVisible()
  await revision.getByRole('button', { name: 'Take back' }).click()
  await revision.getByRole('button', { name: 'Take back' }).click()
  await expect(page.getByText('Change taken back', { exact: true })).toBeVisible()
  await expect(revision.getByText('Taken back', { exact: true })).toBeVisible()
  // The revision that took it back says so in the app's words.
  await page.getByRole('link', { name: 'All changes' }).click()
  await page
    .getByRole('link', { name: /, by you/ })
    .first()
    .click()
  await expect(page.getByText('Taken back in the app.')).toBeVisible()
  await page
    .getByRole('navigation', { name: 'Sections' })
    .getByRole('link', { name: 'Plan' })
    .click()
  await expect(page.getByRole('region', { name: 'No plan yet' })).toBeVisible()

  // What the plan needed cleared again, for the other tests.
  expect((await page.request.put('/api/v1/profile', { data: {} })).ok()).toBe(true)
  expect((await page.request.delete('/api/v1/days/today/metrics/weight')).ok()).toBe(true)
})

test('a proposed change of the limits is accepted in the app, once the user loosens it', async ({
  page,
}) => {
  await signIn(page)
  const made = await page.request.post('/api/v1/proposals', {
    data: {
      rationale: 'A faster cut, made up for the end-to-end tests.',
      model: 'claude-opus-5-5',
      limits: { cut_rate: -1 },
    },
  })
  expect(made.status()).toBe(201)
  await page
    .getByRole('navigation', { name: 'Sections' })
    .getByRole('link', { name: 'Plan' })
    .click()

  const card = page.getByRole('region', { name: 'A proposed change' })
  await expect(
    card.getByRole('row', {
      name: 'Fastest loss in a cut, BMI under 30 0.7% a week 1% a week loosened',
    }),
  ).toBeVisible()
  await card.getByRole('button', { name: 'Accept' }).click()
  await card.getByRole('button', { name: 'Loosen' }).click()
  await expect(card).toBeHidden()
  // The plan's list says it, and so does the limits' page.
  const row = page.getByRole('link', { name: 'Limits of the plan' })
  await expect(row).toHaveAccessibleDescription(/^Loss up to 1% a week/)
  await row.click()
  const limits = page.getByRole('form', { name: 'Limits of the plan' })
  await expect(
    limits.getByRole('textbox', { name: /^Fastest loss in a cut, BMI under 30/ }),
  ).toHaveValue('1')

  // The limit as it was, for the other tests.
  expect(
    (await page.request.put('/api/v1/limits', { data: { limits: { cut_rate: -0.7 } } })).ok(),
  ).toBe(true)
})

// On a phone, the decision is in reach (U2 PL1): a proposal of two
// periods and both types of day, below the resting burn on each, says
// the warning once, and its buttons come within the card's first 600 px.
test.describe('at 390 px', () => {
  test.use({ viewport: { width: 390, height: 844 } })

  test('a proposal warns once and keeps its buttons near its top', async ({ page }) => {
    await signIn(page)
    const api = page.request
    expect(
      (
        await api.put('/api/v1/profile', {
          data: { sex: 'male', birth_date: '1990-06-15', height_cm: 180 },
        })
      ).ok(),
    ).toBe(true)
    expect((await api.put('/api/v1/days/today/metrics/weight', { data: { value: 85 } })).ok()).toBe(
      true,
    )
    const me: unknown = await (await api.get('/api/v1/me')).json()
    const today = typeof me === 'object' && me !== null && 'today' in me ? String(me.today) : ''
    const day = (n: number) => {
      const d = new Date(`${today}T00:00:00Z`)
      d.setUTCDate(d.getUTCDate() + n)
      return d.toISOString().slice(0, 10)
    }
    const made = await api.post('/api/v1/proposals', {
      data: {
        rationale:
          'Two lower targets, made up for the end-to-end tests: a week of one, then the other, each low on both types of day, so the limits warn of each.',
        model: 'claude-opus-5-5',
        targets: [
          {
            from: day(1),
            training: { kcal: 1700, protein: 150, fat: 55 },
            rest: { kcal: 1600, protein: 150, fat: 45 },
          },
          {
            from: day(8),
            training: { kcal: 1650, protein: 150, fat: 55 },
            rest: { kcal: 1550, protein: 150, fat: 45 },
          },
        ],
      },
    })
    expect(made.status()).toBe(201)
    try {
      await page.goto('/plan')
      const card = page.getByRole('region', { name: 'A proposed change' })
      const warning = card.getByRole('button', { name: /guardrail warning/ })
      await expect(warning).toContainText('1 guardrail warning · both day types, both periods')
      await expect(card.getByText(/resting burn/)).toHaveCount(1)
      const top = (await card.boundingBox())?.y ?? 0
      const boxes = await Promise.all(
        ['Accept', 'Reject'].map((name) => card.getByRole('button', { name }).boundingBox()),
      )
      for (const box of boxes) {
        expect((box?.y ?? Infinity) + (box?.height ?? 0) - top).toBeLessThanOrEqual(600)
      }
    } finally {
      // Nothing of it left, for the other tests.
      const plan: unknown = await (await api.get('/api/v1/plan')).json()
      const pending =
        typeof plan === 'object' && plan !== null && 'pending' in plan ? plan.pending : undefined
      if (typeof pending === 'object' && pending !== null && 'id' in pending) {
        await api.post(`/api/v1/proposals/${String(pending.id)}/reject`)
      }
      expect((await api.put('/api/v1/profile', { data: {} })).ok()).toBe(true)
      expect((await api.delete('/api/v1/days/today/metrics/weight')).ok()).toBe(true)
    }
  })
})
