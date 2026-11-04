import { expect, test, type APIResponse } from '@playwright/test'

import { signIn } from './user.ts'

// idOf is the id an answer of a food, or a recipe's save, names.
async function idOf(answer: APIResponse): Promise<string> {
  expect(answer.ok()).toBe(true)
  const body: unknown = await answer.json()
  const food = typeof body === 'object' && body !== null && 'food' in body ? body.food : body
  return typeof food === 'object' && food !== null && 'id' in food && typeof food.id === 'string'
    ? food.id
    : ''
}

test('a recipe shared with another user is theirs at once: its members, its ingredients, a portion logged', async ({
  page,
  browser,
}) => {
  // Two users, an invite and a share: longer than a test of one.
  test.setTimeout(90_000)
  const tag = String(Date.now())
  const email = `mia-${tag}@example.com`
  await signIn(page)

  // Mia, invited, makes her account in a browser of her own.
  const invite = await page.request.post('/api/v1/invites', { data: { email, role: 'user' } })
  expect(invite.ok()).toBe(true)
  const made: unknown = await invite.json()
  const url = typeof made === 'object' && made !== null && 'url' in made ? String(made.url) : ''
  const theirs = await browser.newContext({ serviceWorkers: 'block' })
  const mia = await theirs.newPage()
  await mia.goto(url)
  await mia.getByLabel('Name').fill('Mia')
  await mia.getByLabel('Password').fill('a password of her own')
  await mia.getByRole('button', { name: 'Make the account' }).click()
  await expect(mia.getByRole('heading', { name: 'Today', exact: true })).toBeVisible()

  // Ana's goulash, of her beef, shared with Mia from its page.
  const beef = await idOf(
    await page.request.post('/api/v1/foods', {
      data: {
        name: `Beef ${tag}`,
        basis: '100g',
        source: 'label',
        nutrients: { kcal: 250, protein: 26, carbs: 0, fat: 17 },
      },
    }),
  )
  const goulash = await idOf(
    await page.request.post('/api/v1/recipes/new', {
      data: {
        name: `Goulash ${tag}`,
        items: [{ food_id: beef, amount: 600, unit: 'g' }],
        yield_g: 1000,
        portions: 4,
      },
    }),
  )
  await page.goto(`/foods/${goulash}`)
  const recipe = page.getByRole('article', { name: `Goulash ${tag}` })
  await recipe.getByRole('button', { name: 'Share' }).click()
  const sheet = page.getByRole('dialog', { name: 'Share the recipe' })
  await sheet.getByLabel('Their email address').fill(email)
  await sheet.getByRole('button', { name: 'Share' }).click()
  await expect(page.getByText(`Shared with ${email}`)).toBeVisible()
  await expect(recipe.getByRole('list', { name: 'Shared with' }).getByText('Mia')).toBeVisible()

  // Mia has it: Ana among its members, her beef Ana's, none of Mia's.
  await mia.goto('/foods?filter=recipes')
  await mia.getByRole('link', { name: new RegExp(`Goulash ${tag}`) }).click()
  const hers = mia.getByRole('article', { name: `Goulash ${tag}` })
  await expect(hers.getByRole('list', { name: 'Shared with' }).getByText('Ana')).toBeVisible()
  await expect(hers.getByRole('list', { name: 'Ingredients' }).getByText('theirs')).toBeVisible()

  // A portion logged, as any recipe of hers.
  await hers.getByRole('button', { name: /^Log/ }).click()
  const meal = mia.getByRole('dialog', { name: /^(Breakfast|Lunch|Snack|Dinner)$/ })
  await meal.getByRole('button', { name: `Add: Goulash ${tag}` }).click()
  await meal.getByRole('button', { name: /^Log \w+ · / }).click()
  await expect(mia.getByText(/^Logged: /)).toBeVisible()
  await theirs.close()
})
