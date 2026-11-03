// The checks of a meal's lines run code in the page, which has the DOM's
// types.
/// <reference lib="dom" />

import { expect, test, type Locator, type Page } from '@playwright/test'

import { openMeal, signIn } from './user.ts'

test('the day shows what was logged, and the days before it', async ({ page }) => {
  await signIn(page)
  // Logged as a model would, through the API, with the page's session.
  const meal = await page.request.post('/api/v1/meals', {
    data: {
      description: 'Lunch',
      items: [
        { name: 'Chicken with rice', nutrients: { kcal: 640, protein: 45, carbs: 70, fat: 15 } },
      ],
    },
  })
  expect(meal.ok()).toBe(true)
  const weighing = await page.request.put('/api/v1/days/today/metrics/weight', {
    data: { value: 72.5 },
  })
  expect(weighing.ok()).toBe(true)

  await page.goto('/')

  const lunch = page.getByRole('article').filter({ hasText: 'Lunch' })
  await expect(lunch).toContainText('640 kcal')
  await openMeal(lunch)
  await expect(lunch).toContainText('Chicken with rice')
  await expect(page.getByRole('region', { name: 'Body today' })).toContainText('72.5 kg')

  await page.getByRole('link', { name: 'The day before' }).click()
  await expect(page.getByText('Nothing logged yet.')).toBeVisible()
  await expect(page.getByRole('region', { name: 'Body', exact: true })).toBeVisible()
  await page.getByRole('link', { name: 'Back to today' }).click()
  await expect(page.getByRole('heading', { name: 'Today', exact: true })).toBeVisible()
})

test("another day's header is one row: ‹ its date ›, the Today pill and the user's mark", async ({
  page,
}) => {
  await page.setViewportSize({ width: 390, height: 844 })
  await signIn(page)
  await page.getByRole('link', { name: 'The day before' }).click()

  const date = page.getByRole('heading', { level: 1 })
  const back = page.getByRole('link', { name: 'Back to today' })
  await expect(back).toBeVisible()
  await expect(back).toHaveText('Today')
  const [header, before, title, after, pill, mark] = await Promise.all([
    boxOf(page.getByRole('main').locator('header').first()),
    boxOf(page.getByRole('link', { name: 'The day before' })),
    boxOf(date),
    boxOf(page.getByRole('link', { name: 'The day after' })),
    boxOf(back),
    boxOf(page.getByRole('link', { name: 'Settings and account' })),
  ])
  // One row, no higher than two lines of the date: the arrows either side
  // of it, then the pill and the mark, each in it.
  expect(header.height).toBeLessThanOrEqual(64)
  expect(before.x + before.width).toBeLessThanOrEqual(title.x)
  expect(after.x).toBeGreaterThanOrEqual(title.x + title.width)
  expect(pill.x).toBeGreaterThanOrEqual(after.x + after.width)
  expect(mark.x).toBeGreaterThanOrEqual(pill.x + pill.width)
  for (const part of [before, title, after, pill, mark]) {
    expect(part.y).toBeGreaterThanOrEqual(header.y - 1)
    expect(part.y + part.height).toBeLessThanOrEqual(header.y + header.height + 1)
  }
  // Nothing of the history there: it is the settings' and the undo's.
  await expect(page.getByRole('main').locator('header').first().getByRole('button')).toHaveCount(0)
})

test('at 390 px the hero and the week are on the first screen', async ({ page }) => {
  await page.setViewportSize({ width: 390, height: 844 })
  await signIn(page)
  await withTarget(page)
  await page.goto('/')
  const [hero, week, bar] = await Promise.all([
    boxOf(page.getByRole('region', { name: 'Energy and macros' })),
    boxOf(page.getByRole('region', { name: /^This week$/ })),
    boxOf(page.getByRole('navigation', { name: 'Sections' })),
  ])
  expect(week.y).toBeGreaterThan(hero.y + hero.height - 1)
  // Above the bar at the bottom, without a scroll.
  expect(week.y + week.height).toBeLessThanOrEqual(bar.y)
  expect(await page.evaluate(() => window.scrollY)).toBe(0)
  // The one number of what is left: the ring's; the target's arithmetic
  // beside it.
  await expect(page.getByText('Left to TDEE')).toHaveCount(0)
  await expect(
    page.getByRole('region', { name: 'Energy and macros' }).getByRole('link', { name: /^TDEE / }),
  ).toBeVisible()
  // The ⓘ is a finger's whole target: the day's type's chip under it
  // reaches none of it.
  const tip = page.getByRole('button', { name: 'What TDEE is' })
  const box = await boxOf(tip)
  const hit = await page.evaluate(
    ([x, y]) => document.elementFromPoint(x, y)?.closest('button')?.getAttribute('aria-label'),
    [box.x + box.width / 2, box.y + box.height + 10] as const,
  )
  expect(hit).toBe('What TDEE is')
})

test("a meal's row gives its name the room, and its values stay with their labels", async ({
  page,
}) => {
  await page.setViewportSize({ width: 390, height: 844 })
  await signIn(page)
  const long = 'Grilled chicken with roasted vegetables, rice and a yoghurt sauce'
  const logged = await Promise.all(
    (
      [
        ['Yoghurt, granola, honey', 386],
        [long, 1180],
      ] as const
    ).map(([description, kcal]) =>
      page.request.post('/api/v1/meals', {
        data: {
          description,
          items: [
            {
              name: 'A wholegrain sourdough loaf with sunflower seeds, from the bakery',
              nutrients: { kcal, protein: 47.2, carbs: 0.4, fat: 5.4 },
            },
          ],
        },
      }),
    ),
  )
  expect(logged.every((r) => r.ok())).toBe(true)

  await page.goto('/')
  const meal = (description: string) => page.getByRole('article').filter({ hasText: description })
  // The day shown, not its parts while they are hidden under its skeleton.
  await expect(meal('Yoghurt, granola, honey')).toBeVisible()
  // A name of a few words is whole: nothing of it is cut.
  const name = meal('Yoghurt, granola, honey').getByText('Yoghurt, granola, honey', {
    exact: true,
  })
  expect(await name.evaluate((e) => e.scrollWidth <= e.clientWidth)).toBe(true)
  // The time beside it, and not over it.
  const time = await meal('Yoghurt, granola, honey').locator('time').boundingBox()
  const named = await name.boundingBox()
  expect((time?.x ?? 0) + (time?.width ?? 0)).toBeLessThanOrEqual(named?.x ?? 0)
  // The energy on one line, as high as a short meal's, which has room.
  const energy = (description: string, kcal: string) =>
    meal(description).getByText(kcal, { exact: true })
  const short = await energy('Yoghurt, granola, honey', '386').boundingBox()
  const beside = await energy(long, '1,180').boundingBox()
  expect(short).not.toBeNull()
  expect(beside?.height).toBe(short?.height)

  // An item of a long name leaves its values little room: they break
  // between pairs, never within one ("F" on a line, "5.4" on the next).
  await openMeal(meal(long))
  const values = meal(long).getByText(/^1,180\skcal\s·/)
  const lines = await values.evaluate((e) => {
    const text = e.firstChild
    if (!(text instanceof Text)) {
      return []
    }
    // Each character's line, by the top of its box.
    const range = document.createRange()
    const byTop = new Map<number, string>()
    for (let i = 0; i < text.length; i++) {
      range.setStart(text, i)
      range.setEnd(text, i + 1)
      const top = Math.round(range.getBoundingClientRect().top)
      byTop.set(top, (byTop.get(top) ?? '') + (text.data[i] ?? ''))
    }
    return Array.from(byTop.values(), (line) => line.replaceAll('\u00a0', ' ').trim())
  })
  expect(lines.length).toBeGreaterThan(1)
  const pair = String.raw`(?:[\d,.]+ kcal|[PCF] [\d,.]+)`
  for (const line of lines) {
    expect(line).toMatch(new RegExp(`^${pair}(?: · ${pair})*(?: ·)?$`))
  }
})

// linesOf is how many lines an element's text takes, by the tops of its
// boxes.
async function linesOf(element: Locator): Promise<number> {
  return element.evaluate((e) => {
    const range = document.createRange()
    range.selectNodeContents(e)
    return new Set(Array.from(range.getClientRects(), (r) => Math.round(r.top))).size
  })
}

// boxOf is an element's box, which it must have.
async function boxOf(element: Locator) {
  const box = await element.boundingBox()
  if (!box) {
    throw new Error('an element is not shown')
  }
  return box
}

// isObject tells a JSON object, as the server's answers are.
function isObject(body: unknown): body is Record<string, unknown> {
  return typeof body === 'object' && body !== null
}

// withTarget lays a target of 2,250 kcal over today's day and a measured
// TDEE over the summary, as the server answers them: the e2e user has no
// plan, and the day's hero shows all its numbers only with them.
async function withTarget(page: Page) {
  await page.route('**/api/v1/days/today', async (route) => {
    const response = await route.fetch()
    const day: unknown = await response.json()
    await route.fulfill({
      response,
      json: {
        ...(isObject(day) ? day : {}),
        target: { kcal: 2250, protein: 160, carbs: 250, fat: 70 },
      },
    })
  })
  await page.route('**/api/v1/summary', async (route) => {
    const response = await route.fetch()
    const summary: unknown = await response.json()
    const tdee = isObject(summary) && isObject(summary.tdee) ? summary.tdee : {}
    await route.fulfill({
      response,
      json: {
        ...(isObject(summary) ? summary : {}),
        tdee: { ...tdee, measured: true, trusted: true, kcal: 2409, half_width: 37 },
      },
    })
  })
}

test("a desktop's day is in two columns, its meals a table with their total, the week beside it", async ({
  page,
}) => {
  await page.setViewportSize({ width: 1440, height: 900 })
  await signIn(page)
  const logged = await Promise.all(
    (
      [
        ['Granola bowl', 410, 14, 62, 9],
        ['Tuna salad', 520, 38, 20, 28],
      ] as const
    ).map(([description, kcal, protein, carbs, fat]) =>
      page.request.post('/api/v1/meals', {
        data: {
          description,
          items: [{ name: description, nutrients: { kcal, protein, carbs, fat } }],
        },
      }),
    ),
  )
  expect(logged.every((r) => r.ok())).toBe(true)
  await withTarget(page)
  await page.goto('/')

  // The day at the left, the wider; the week at the right, level with it,
  // its days in its card and not above the day.
  const summary = page.getByRole('region', { name: 'Energy and macros' })
  const week = page.getByRole('region', { name: 'This week' })
  await expect(week.getByRole('navigation', { name: 'The week' })).toBeVisible()
  const [day, side] = await Promise.all([boxOf(summary), boxOf(week)])
  expect(side.x).toBeGreaterThan(day.x + day.width)
  expect(day.width).toBeGreaterThan(side.width * 1.5)
  expect(Math.abs(side.y - day.y)).toBeLessThan(2)
  // Beside the ring its numbers side by side, and the macros in three
  // columns.
  const [eaten, target] = await Promise.all([
    boxOf(summary.getByText('Eaten', { exact: true })),
    boxOf(summary.getByText('Target', { exact: true })),
  ])
  expect(target.y).toBe(eaten.y)
  expect(target.x).toBeGreaterThan(eaten.x + eaten.width)
  const macros = await Promise.all(
    ['Protein', 'Carbs', 'Fat'].map((m) => boxOf(summary.getByText(m, { exact: true }))),
  )
  expect(new Set(macros.map((m) => m.y)).size).toBe(1)
  // Then the body, the check-in and the phase, one under another.
  const body = await boxOf(page.getByRole('region', { name: 'Body today' }))
  const checkIn = await boxOf(page.getByRole('region', { name: 'Check-in' }))
  expect(body.x).toBe(side.x)
  expect(body.y).toBeGreaterThan(side.y + side.height)
  expect(checkIn.y).toBeGreaterThan(body.y + body.height)

  // The meals a table: its heads, each row's kcal and macros under them,
  // and the total, the day's.
  const meals = page.getByRole('region', { name: 'Meals' })
  await expect(meals.getByRole('article').filter({ hasText: 'Tuna salad' })).toBeVisible()
  const columns = await meals.evaluate((section) => {
    const heads = section.querySelector('[aria-hidden="true"]')
    const total = Array.from(section.querySelectorAll('p')).find(
      (p) => p.firstElementChild?.textContent === 'Total',
    )
    // The heads, each meal's row and the total, and of each the right
    // edges of its kcal, protein, carbs and fat: after the time and the
    // meal, or after the total's word, which spans both.
    const lines = [heads, ...section.querySelectorAll('article h3 button'), total]
    return {
      heads: Array.from(heads?.children ?? [], (c) => c.textContent),
      rights: lines.map((line) =>
        Array.from(line?.children ?? [])
          .slice(line === total ? 1 : 2)
          .slice(0, 4)
          .map((c) => Math.round(c.getBoundingClientRect().right)),
      ),
    }
  })
  expect(columns.heads).toEqual(['Time', 'Meal', 'kcal', 'P', 'C', 'F'])
  // The heads, two meals at least, and the total.
  expect(columns.rights.length).toBeGreaterThanOrEqual(4)
  expect(columns.rights[0]).toHaveLength(4)
  expect(new Set(columns.rights.map((r) => r.join(' '))).size).toBe(1)
  // The total is what the day's summary says was eaten.
  const eatenToday = await summary
    .getByText('Eaten', { exact: true })
    .locator('xpath=..')
    .innerText()
  await expect(meals.getByText('Total', { exact: true }).locator('xpath=..')).toContainText(
    eatenToday.replace('Eaten', '').trim(),
  )
  // A row still opens to its items, and the ways to change it.
  const tuna = meals.getByRole('article').filter({ hasText: 'Tuna salad' })
  await openMeal(tuna)
  await expect(tuna.getByRole('button', { name: 'Change the meal: Tuna salad' })).toBeVisible()

  // Another day's header is as wide as the day: the date by the arrows,
  // the way back to today at the other end, level with it.
  await page.getByRole('link', { name: 'The day before' }).click()
  const back = page.getByRole('link', { name: 'Back to today' })
  await expect(back).toBeVisible()
  const [date, pill, before, card] = await Promise.all([
    boxOf(page.getByRole('heading', { level: 1 })),
    boxOf(back),
    boxOf(page.getByRole('link', { name: 'The day before' })),
    // On a Monday the day before is in the week before: That week.
    boxOf(page.getByRole('region', { name: /^(This|That) week$/ })),
  ])
  expect(date.x).toBeGreaterThan(before.x + before.width)
  expect(date.x - (before.x + before.width)).toBeLessThan(60)
  expect(Math.abs(pill.x + pill.width - (card.x + card.width))).toBeLessThan(2)
  expect(pill.y).toBeGreaterThan(date.y - pill.height)
  expect(pill.y).toBeLessThan(date.y + date.height)
})

test("a phone's day is one column as before, today's Croatian date on one line", async ({
  page,
}) => {
  await page.setViewportSize({ width: 390, height: 844 })
  await signIn(page)
  expect((await page.request.patch('/api/v1/me', { data: { locale: 'hr' } })).ok()).toBe(true)
  try {
    const meal = await page.request.post('/api/v1/meals', {
      data: {
        description: 'Kajgana',
        items: [{ name: 'Kajgana', nutrients: { kcal: 320, protein: 20, carbs: 2, fat: 25 } }],
      },
    })
    expect(meal.ok()).toBe(true)
    await withTarget(page)
    await page.goto('/')

    // The date under "Danas", on a line of its own, and the user's mark
    // at the row's end; the day's type is the hero's.
    const title = page.getByRole('heading', { level: 1, name: 'Danas' })
    await expect(title).toBeVisible()
    const date = page.locator('header').filter({ has: title }).locator('p')
    expect(await linesOf(date)).toBe(1)
    const [heading, when, type, hero] = await Promise.all([
      boxOf(title),
      boxOf(date),
      boxOf(page.getByRole('button', { name: /^Tip dana: / })),
      boxOf(page.getByRole('region', { name: 'Energija i makroi' })),
    ])
    expect(when.y).toBeGreaterThanOrEqual(heading.y + heading.height - 1)
    expect(type.y).toBeGreaterThan(hero.y)
    expect(type.y + type.height).toBeLessThan(hero.y + hero.height)
    const mark = await boxOf(page.getByRole('link', { name: 'Postavke i račun' }))
    expect(mark.x).toBeGreaterThan(heading.x + heading.width)

    // The day, then its week; its meals' cards, the body and the
    // check-in, one under another, as wide.
    const [summary, week, meals, body, checkIn] = await Promise.all([
      boxOf(page.getByRole('region', { name: 'Energija i makroi' })),
      boxOf(page.getByRole('region', { name: 'Ovaj tjedan' })),
      boxOf(page.getByRole('region', { name: 'Obroci' })),
      boxOf(page.getByRole('region', { name: 'Tijelo danas' })),
      boxOf(page.getByRole('region', { name: 'Check-in' })),
    ])
    expect(week.y).toBeGreaterThan(summary.y + summary.height - 1)
    expect(meals.y).toBeGreaterThan(week.y + week.height)
    expect(body.y).toBeGreaterThan(meals.y + meals.height)
    expect(checkIn.y).toBeGreaterThan(body.y + body.height)
    for (const box of [summary, week, meals, body, checkIn]) {
      expect(box.x).toBe(summary.x)
      expect(box.width).toBe(summary.width)
    }
    // Its meals are cards, with no table's heads nor total.
    await expect(page.getByRole('article').filter({ hasText: 'Kajgana' })).toBeVisible()
    await expect(page.getByText('Zbroj', { exact: true })).toHaveCount(0)

    // A week with a day logged says its average in a line under its
    // days, which opens them: a week long past, which the other tests
    // leave alone (today's has no day over yet, nor a plan).
    const past = await page.request.post('/api/v1/meals', {
      data: {
        eaten_at: '2025-03-19T12:00:00Z',
        items: [{ name: 'Grah', nutrients: { kcal: 640, protein: 30, carbs: 80, fat: 18 } }],
      },
    })
    expect(past.ok()).toBe(true)
    await page.goto('/?date=2025-03-19')
    const line = page.getByRole('button', { name: 'Taj tjedan: prosjek 640 · 1 upisan dan' })
    await expect(line).toBeVisible()
    const [days, said, day] = await Promise.all([
      boxOf(page.getByRole('navigation', { name: 'Tjedan' })),
      boxOf(line),
      boxOf(page.getByRole('region', { name: 'Pojedeno' })),
    ])
    expect(said.y).toBeGreaterThan(days.y + days.height - 1)
    expect(days.y).toBeGreaterThan(day.y + day.height - 1)
    // A day before the plan is a column of what it ate.
    await expect(
      page
        .getByRole('navigation', { name: 'Tjedan' })
        .getByRole('link', { name: 'sri 19.: 640 kcal' }),
    ).toBeVisible()
    await line.click()
    const sheet = page.getByRole('dialog', { name: 'Taj tjedan' })
    await expect(sheet.getByRole('row', { name: /^sri 19/ })).toContainText('640')
  } finally {
    await page.request.patch('/api/v1/me', { data: { locale: 'en' } })
  }
})
