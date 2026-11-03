import { expect, test } from '@playwright/test'

import { signIn } from './user.ts'

test('a food is made, logged, found, corrected, archived and brought back', async ({ page }) => {
  await signIn(page)
  await page
    .getByRole('navigation', { name: 'Sections' })
    .getByRole('link', { name: 'Foods' })
    .click()

  // The header's, or an empty list's own.
  await page.getByRole('link', { name: /^(New food|Add a food)$/ }).click()
  await page.getByLabel('Name', { exact: true }).fill('Buckwheat')
  // A slip of the energy, which the macros tell, and their energy taken.
  await page.getByLabel('Energy (kcal)').fill('34')
  await page.getByLabel('Protein (g)').fill('13.3')
  await page.getByLabel('Carbs (g)').fill('71.5')
  await page.getByLabel('Fat (g)', { exact: true }).fill('3.4')
  await expect(page.getByText('The macros make about 370 kcal, not 34')).toBeVisible()
  await page.getByRole('button', { name: 'Use 370 kcal' }).click()
  await expect(page.getByLabel('Energy (kcal)')).toHaveValue('370')
  await page.getByRole('button', { name: 'Add another name' }).click()
  await expect(page.getByLabel('Other name')).toBeFocused()
  await page.getByLabel('Other name').fill('kasha')
  await page.getByRole('button', { name: 'Save the food' }).click()
  await expect(page.getByRole('heading', { name: 'Buckwheat' })).toBeVisible()

  // Logged from its page: the meal to log opens on its amount.
  await page.getByRole('button', { name: 'Log it' }).click()
  const sheet = page.getByRole('dialog', { name: 'New meal' })
  await expect(sheet.getByLabel('Amount: Buckwheat')).toHaveValue('100')
  await sheet.getByRole('button', { name: 'Add: Buckwheat' }).click()
  await sheet.getByRole('button', { name: 'Log 1 item · 370 kcal' }).click()
  await expect(page.getByText(/^Logged: .* · 370 kcal$/)).toBeVisible()
  await expect(page.getByText(/^1× · last /)).toBeVisible()
  await expect(page.getByRole('button', { name: 'Log 100 g' })).toBeVisible()

  await page.getByRole('link', { name: 'Back to the foods' }).click()
  await page.getByLabel('Search the foods').fill('kash')
  await page
    .getByRole('list', { name: 'Found' })
    .getByRole('link', { name: /Buckwheat/ })
    .click()
  await page.getByRole('link', { name: 'Edit' }).click()
  await page.getByLabel('Energy (kcal)').fill('346')
  await page.getByRole('button', { name: 'Save the food' }).click()
  await expect(page.getByText('Saved: Buckwheat')).toBeVisible()
  await expect(page.getByText('kcal per 100 g')).toBeVisible()

  await page.getByRole('button', { name: 'Archive' }).click()
  await expect(page.getByText('Archived. Find it among the archived foods')).toBeVisible()
  await page.getByRole('link', { name: 'Back to the foods' }).click()
  await expect(page.getByRole('list', { name: 'Found' })).toHaveCount(0)
  await page.getByLabel('Search the foods').fill('')
  await page.getByRole('group', { name: 'Lists' }).getByText('Archive', { exact: true }).click()
  await expect(page.getByRole('radio', { name: 'Archive' })).toBeChecked()
  const archived = page.getByRole('list', { name: /^Archive/ })
  await expect(archived).toContainText('346 kcal')
  // A search of the archive is of the archived foods.
  await page.getByLabel('Search the foods').fill('buck')
  await expect(page.getByRole('list', { name: 'Found' })).toContainText('Buckwheat')
  await page.getByLabel('Search the foods').fill('')
  await archived.getByRole('link', { name: /Buckwheat/ }).click()
  await page.getByRole('button', { name: 'Bring back' }).click()
  await expect(page.getByRole('button', { name: 'Archive' })).toBeVisible()
})

test("a food's servings of long names fit a phone's width", async ({ page }) => {
  await page.setViewportSize({ width: 390, height: 844 })
  await signIn(page)
  // Two servings, so segments, each of a name longer than a third of it.
  const food = await page.request.post('/api/v1/foods', {
    data: {
      name: 'Rye loaf',
      basis: '100g',
      source: 'label',
      nutrients: { kcal: 250, protein: 8, carbs: 48, fat: 2 },
      servings: [
        { name: 'a very thick slice of home-made bread', amount: 65 },
        { name: 'half a loaf of rye bread', amount: 250 },
      ],
    },
  })
  const saved: unknown = await food.json()
  const id =
    typeof saved === 'object' && saved !== null && 'id' in saved && typeof saved.id === 'string'
      ? saved.id
      : ''
  await page.goto(`/foods/${id}`)
  const half = page.getByRole('radio', { name: 'half a loaf of rye bread (250 g)' })
  await expect(half).not.toBeChecked()

  // The segments share the card's width, their names cut short: the page
  // does not scroll sideways. A string: the tests' types know no DOM.
  const width = await page.evaluate<number>('document.documentElement.scrollWidth')
  expect(width).toBe(390)
  await page.getByText('half a loaf of rye bread (250 g)', { exact: true }).click()
  await expect(half).toBeChecked()
  await expect(page.getByRole('cell', { name: '625 kcal' })).toBeVisible()
})

test("a desktop's foods and food scroll on their own, the food's end in reach and its title in view", async ({
  page,
}) => {
  await page.setViewportSize({ width: 1440, height: 900 })
  await signIn(page)
  const tag = String(Date.now())
  // A recipe of many ingredients, taller than the screen.
  const made = await Promise.all(
    Array.from({ length: 14 }, (_, i) =>
      page.request.post('/api/v1/foods', {
        data: {
          name: `Ingredient ${String(i)} ${tag}`,
          basis: '100g',
          source: 'label',
          nutrients: { kcal: 100 + i, protein: 5, carbs: 10, fat: 3 },
        },
      }),
    ),
  )
  const ids = await Promise.all(
    made.map(async (answer) => {
      const body: unknown = await answer.json()
      return typeof body === 'object' && body !== null && 'id' in body ? String(body.id) : ''
    }),
  )
  const answer = await page.request.post('/api/v1/recipes/new', {
    data: {
      name: `Stew ${tag}`,
      items: ids.map((id) => ({ food_id: id, amount: 100, unit: 'g' })),
      yield_g: 1200,
      portions: 4,
    },
  })
  const saved: unknown = await answer.json()
  const food =
    typeof saved === 'object' && saved !== null && 'food' in saved ? saved.food : undefined
  const id = typeof food === 'object' && food !== null && 'id' in food ? String(food.id) : ''
  await page.goto(`/foods/${id}`)
  const title = page.getByRole('heading', { name: `Stew ${tag}`, level: 1 })
  await expect(title).toBeVisible()

  // The page itself does not scroll: each pane does.
  const height = await page.evaluate<number>('document.documentElement.scrollHeight')
  expect(height).toBeLessThanOrEqual(900)
  const end = page.getByText('Values from')
  await end.scrollIntoViewIfNeeded()
  await expect(end).toBeInViewport()
  // The title stays at the top of its pane.
  await expect(title).toBeInViewport()
  // The list beside it, its search at the top of its own pane.
  await expect(page.getByLabel('Search the foods')).toBeInViewport()

  // What it brings is across both columns: its amounts' segments whole.
  const amounts = page.getByRole('group', { name: 'Values for' }).locator('label span')
  await expect(amounts).toHaveText(['1 portion', '100 g', 'Whole recipe'])
  const cut = await amounts.evaluateAll((spans) =>
    spans.filter((span) => span.scrollWidth > span.clientWidth).map((span) => span.textContent),
  )
  expect(cut).toEqual([])

  // A control the focus comes back to, far above, scrolls clear of the
  // title that stays at the pane's top.
  // The pane is scrolled so it is just under the title's top, in the
  // pane's sight but hidden.
  await page.getByRole('button', { name: 'Share' }).focus()
  const edit = page.getByRole('link', { name: 'Edit the recipe' })
  await edit.evaluate((link) => {
    const pane = link.closest('.overflow-y-auto')
    if (pane !== null) {
      pane.scrollTop += link.getBoundingClientRect().top - pane.getBoundingClientRect().top - 8
    }
  })
  await page.keyboard.press('Shift+Tab')
  await expect(edit).toBeFocused()
  const below = await edit.boundingBox()
  const header = await title.locator('xpath=..').boundingBox()
  expect(below?.y ?? 0).toBeGreaterThanOrEqual((header?.y ?? 0) + (header?.height ?? 0))
})
