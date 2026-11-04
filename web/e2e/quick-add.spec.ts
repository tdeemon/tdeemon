import { expect, test, type Page } from '@playwright/test'

import { signIn } from './user.ts'

test('a meal eaten at this hour before is a card on top of the first list, and logged again in two taps', async ({
  page,
}) => {
  await signIn(page)
  // Meals logged as a model would, through the API: one two days before,
  // now (the day before is the day's test's), and one just now.
  const logged = await Promise.all(
    [
      { name: 'Chickpea curry', at: Date.now() - 2 * 86_400_000 },
      { name: 'Lentil soup', at: Date.now() },
    ].map(async ({ name, at }) => {
      const meal = await page.request.post('/api/v1/meals', {
        data: {
          description: name,
          eaten_at: new Date(at).toISOString(),
          items: [{ name, nutrients: { kcal: 540, protein: 22, carbs: 70, fat: 18 } }],
        },
      })
      return meal.ok()
    }),
  )
  expect(logged).toEqual([true, true])
  await page.goto('/')
  const curries = page.getByRole('article').filter({ hasText: 'Chickpea curry' })
  await expect(page.getByRole('article').filter({ hasText: 'Lentil soup' })).toHaveCount(1)
  await expect(curries).toHaveCount(0)

  // One: the bar's +. Two: the card's Repeat, on top of the first list;
  // the meal just logged is no card.
  await page
    .getByRole('navigation', { name: 'Sections' })
    .getByRole('button', { name: 'Log a meal' })
    .click()
  const sheet = page.getByRole('dialog', { name: /^(Breakfast|Lunch|Snack|Dinner)$/ })
  const cards = sheet.getByRole('list', { name: 'Meals to repeat' })
  await expect(cards.getByRole('button', { name: 'Repeat: Lentil soup' })).toHaveCount(0)
  await cards.getByRole('button', { name: 'Repeat: Chickpea curry' }).click()

  await expect(page.getByText('Repeated: Chickpea curry · 540 kcal')).toBeVisible()
  await expect(sheet).toBeHidden()
  await expect(curries).toHaveCount(1)
})

test("the bar's + logs a meal over the foods, which stay, its toast the way to today", async ({
  page,
}) => {
  await signIn(page)
  await page.goto('/foods')
  await expect(page.getByRole('heading', { name: 'Foods' })).toBeVisible()
  await page
    .getByRole('navigation', { name: 'Sections' })
    .getByRole('button', { name: 'Log a meal' })
    .click()
  const sheet = page.getByRole('dialog', { name: /^(Breakfast|Lunch|Snack|Dinner)$/ })
  // Its kcal alone, logged from the estimate's step: the meal holds
  // nothing else.
  await sheet.getByRole('button', { name: 'An estimate in kcal' }).click()
  await sheet.getByLabel('Energy (kcal)').fill('120')
  await sheet.getByLabel('What it is').fill('Flat white')
  // Enter presses its primary button, as the eye sees it: Log.
  await expect(sheet.getByRole('button', { name: /^Log \w+ · 120 kcal$/ })).toBeVisible()
  await sheet.getByLabel('What it is').press('Enter')

  await expect(page.getByText(/^Logged: Flat white · 120 kcal/)).toBeVisible()
  await expect(sheet).toBeHidden()
  await expect(page).toHaveURL('/foods')
  await expect(page.getByRole('heading', { name: 'Foods' })).toBeVisible()
  await page.getByRole('link', { name: 'See it today' }).click()
  await expect(page).toHaveURL('/')
  await expect(page.getByRole('article').filter({ hasText: 'Flat white' })).not.toHaveCount(0)
})

test('on a phone the keyboard waits for a tap on the search', async ({ page }) => {
  await signIn(page)
  await page
    .getByRole('navigation', { name: 'Sections' })
    .getByRole('button', { name: 'Log a meal' })
    .click()
  const sheet = page.getByRole('dialog', { name: /^(Breakfast|Lunch|Snack|Dinner)$/ })
  const search = sheet.getByLabel('Find a food or a meal')
  await expect(sheet).toBeFocused()
  await expect(search).not.toBeFocused()

  await search.tap()
  await expect(search).toBeFocused()
})

test.describe('on a desktop', () => {
  test.use({ viewport: { width: 1440, height: 900 }, isMobile: false, hasTouch: false })

  test('a side panel of 440 px, the search with the focus, opened by its key and logged by ⌘↵', async ({
    page,
  }) => {
    await signIn(page)
    await eaten(page, ['Kiwi'])
    await page.goto('/')
    await page.keyboard.press('n')
    const sheet = page.getByRole('dialog', { name: /^(Breakfast|Lunch|Snack|Dinner)$/ })
    const search = sheet.getByLabel('Find a food or a meal')
    await expect(search).toBeFocused()
    // Once it has slid in.
    await expect(async () => {
      const box = await sheet.boundingBox()
      expect(box?.width).toBe(440)
      expect((box?.x ?? 0) + (box?.width ?? 0)).toBe(1440)
    }).toPass()

    // Typed, Enter opens it for its amount, Enter adds it, and the search
    // has its words selected for the next.
    await search.fill('kiwi')
    await search.press('Enter')
    await sheet.getByLabel('Amount: Kiwi').press('Enter')
    await expect(search).toBeFocused()
    await search.press('ControlOrMeta+Enter')
    await expect(page.getByText(/^Logged: Kiwi · /)).toBeVisible()
    await expect(sheet).toBeHidden()
  })
})

// eaten makes foods of these names, 100 kcal a piece of 100 g, each eaten
// alone these days before at this time (3), so the first list has them.
async function eaten(page: Page, names: string[], days = [3]) {
  const logged = await Promise.all(
    names.map(async (name) => {
      const food = await page.request.post('/api/v1/foods', {
        data: {
          name,
          basis: '100g',
          source: 'label',
          nutrients: { kcal: 100, protein: 2, carbs: 20, fat: 1 },
        },
      })
      const saved: unknown = await food.json()
      const id =
        typeof saved === 'object' && saved !== null && 'id' in saved && typeof saved.id === 'string'
          ? saved.id
          : ''
      const meals = await Promise.all(
        days.map((ago) =>
          page.request.post('/api/v1/meals', {
            data: {
              eaten_at: new Date(Date.now() - ago * 86_400_000).toISOString(),
              items: [{ food_id: id, amount: 100 }],
            },
          }),
        ),
      )
      return meals.every((m) => m.ok())
    }),
  )
  expect(logged).toEqual(names.map(() => true))
}

test('a banana and a yogurt are logged in six actions, the tray naming the meal', async ({
  page,
}) => {
  await signIn(page)
  await eaten(page, ['Banana', 'Greek yogurt'])
  await page.goto('/')
  const actions: string[] = []
  const act = async (what: string, action: () => Promise<void>) => {
    actions.push(what)
    await action()
  }

  await act('open', async () => {
    await page
      .getByRole('navigation', { name: 'Sections' })
      .getByRole('button', { name: 'Log a meal' })
      .click()
  })
  const sheet = page.getByRole('dialog', { name: /^(Breakfast|Lunch|Snack|Dinner)$/ })
  const search = sheet.getByLabel('Find a food or a meal')
  // No tray, and so no button waiting disabled, until an item is in it.
  await expect(sheet.getByRole('button', { name: /^Log / })).toHaveCount(0)
  await act('type banana', () => search.fill('banana'))
  await act('add the banana', () => sheet.getByRole('button', { name: 'Add: Banana' }).click())
  // The search stays, the banana's + checked.
  await expect(sheet.getByRole('button', { name: 'Add: Banana' })).toHaveAttribute(
    'aria-pressed',
    'true',
  )
  await expect(sheet.getByText('1 item', { exact: true })).toBeVisible()
  await act('type yogurt', () => search.fill('yogurt'))
  await act('add the yogurt', () =>
    sheet.getByRole('button', { name: 'Add: Greek yogurt' }).click(),
  )
  await expect(sheet.getByText('2 items', { exact: true })).toBeVisible()
  await act('log', () => sheet.getByRole('button', { name: /^Log \w+ · 200 kcal$/ }).click())

  await expect(page.getByText(/^Logged: Banana and Greek yogurt · 200 kcal/)).toBeVisible()
  await expect(sheet).toBeHidden()
  expect(actions.length).toBeLessThanOrEqual(6)

  // Undo takes the meal back.
  await page.getByRole('button', { name: 'Undo' }).click()
  await expect(page.getByText('Undone: a meal logged.')).toBeVisible()
  await expect(
    page.getByRole('article').filter({ hasText: 'Banana and Greek yogurt' }),
  ).toHaveCount(0)
})

test('the first screen says nothing twice: a meal of one food is a card, not a row too', async ({
  page,
}) => {
  await signIn(page)
  // Eaten at this time the two days before: first of the cards.
  await eaten(page, ['Plum'], [1, 2])
  await page.goto('/')
  await page
    .getByRole('navigation', { name: 'Sections' })
    .getByRole('button', { name: 'Log a meal' })
    .click()
  const sheet = page.getByRole('dialog', { name: /^(Breakfast|Lunch|Snack|Dinner)$/ })
  await expect(
    sheet
      .getByRole('list', { name: 'Meals to repeat' })
      .getByRole('button', { name: 'Repeat: Plum' }),
  ).toBeVisible()
  await expect(sheet.getByRole('button', { name: 'Add: Plum' })).toHaveCount(0)
})

test('a food not found is made in a step of its own, and goes into the meal logged', async ({
  page,
}) => {
  await signIn(page)
  await page
    .getByRole('navigation', { name: 'Sections' })
    .getByRole('button', { name: 'Log a meal' })
    .click()
  const sheet = page.getByRole('dialog', { name: /^(Breakfast|Lunch|Snack|Dinner)$/ })
  await sheet.getByLabel('Find a food or a meal').fill('kefir')
  await expect(
    sheet.getByText('Nothing matches. Add an estimate, or make a new food.'),
  ).toBeVisible()
  await sheet.getByRole('button', { name: 'New food' }).click()

  // In the search's place, with one button: the meal has no tray yet.
  await expect(sheet.getByRole('button', { name: 'Back to search' })).toBeVisible()
  await expect(sheet.getByRole('button', { name: /^Log / })).toBeHidden()
  await expect(sheet.getByLabel('Name', { exact: true })).toHaveValue('kefir')
  await sheet.getByLabel('Name', { exact: true }).fill('Kefir')
  await sheet.getByText('per 100 ml').click()
  await sheet.getByLabel('Energy (kcal)').fill('41')
  await sheet.getByLabel('Protein (g)').fill('3.4')
  await sheet.getByLabel('Carbs (g)').fill('4')
  await sheet.getByLabel('Fat (g)', { exact: true }).fill('1.5')
  await sheet.getByLabel('Eaten now (ml)').fill('250')
  await sheet.getByRole('button', { name: 'Add to meal' }).click()

  await sheet.getByRole('button', { name: 'Edit amounts' }).click()
  await expect(sheet.getByRole('list', { name: 'In the meal' })).toContainText(
    'Kefir · 250 ml · 103 kcal',
  )
  await expect(sheet.getByLabel('Find a food or a meal')).toHaveValue('')
  await sheet.getByRole('button', { name: /^Log \w+ · 103 kcal$/ }).click()
  await expect(page.getByText(/^Logged: Kefir · 103 kcal/)).toBeVisible()
  await expect(sheet).toBeHidden()
  await expect(page.getByRole('article').filter({ hasText: 'Kefir' })).toHaveCount(1)
})

test("the day's chips scroll on their own, not the page", async ({ page }) => {
  await signIn(page)
  // Foods of long names, each eaten: chips wider than a phone together.
  const names = ['Greek yoghurt with honey', 'Wholegrain sourdough', 'Smoked salmon slices']
  const logged = await Promise.all(
    names.map(async (name) => {
      const food = await page.request.post('/api/v1/foods', {
        data: {
          name,
          basis: '100g',
          source: 'label',
          nutrients: { kcal: 200, protein: 10, carbs: 20, fat: 8 },
        },
      })
      const saved: unknown = await food.json()
      const id =
        typeof saved === 'object' && saved !== null && 'id' in saved && typeof saved.id === 'string'
          ? saved.id
          : ''
      const meal = await page.request.post('/api/v1/meals', {
        data: { items: [{ food_id: id, amount: 150 }] },
      })
      return meal.ok()
    }),
  )
  expect(logged).toEqual([true, true, true])
  await page.goto('/')
  await expect(page.getByRole('region', { name: 'Quick add' })).toBeVisible()

  // A string: the tests' types know no DOM.
  const width = await page.evaluate<number>('document.documentElement.scrollWidth')
  expect(width).toBe(page.viewportSize()?.width)
})
