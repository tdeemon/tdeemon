import { expect, test, type Browser, type Page } from '@playwright/test'

import { signIn } from './user.ts'

// newAccount is a user the e2e user, an admin, invites, who makes their
// account in a browser of their own: its page, signed in.
async function newAccount(admin: Page, browser: Browser, name: string, email: string) {
  const invite = await admin.request.post('/api/v1/invites', { data: { email, role: 'user' } })
  expect(invite.ok()).toBe(true)
  const made: unknown = await invite.json()
  const url = typeof made === 'object' && made !== null && 'url' in made ? String(made.url) : ''
  const context = await browser.newContext({ serviceWorkers: 'block' })
  const page = await context.newPage()
  await page.goto(url)
  await page.getByLabel('Name').fill(name)
  await page.getByLabel('Password').fill(`a password of ${name}'s own`)
  await page.getByRole('button', { name: 'Make the account' }).click()
  await expect(page.getByRole('heading', { name: 'Today', exact: true })).toBeVisible()
  return page
}

test('a household: invited, what joining does seen, joined, the kitchen shared, and left', async ({
  page,
  browser,
}) => {
  // Three users and a merge: longer than a test of one.
  test.setTimeout(90_000)
  const tag = String(Date.now())
  const miaEmail = `mia-${tag}@example.com`
  const leoEmail = `leo-${tag}@example.com`
  // Two new users, so the e2e user, whom the other tests share, keeps a
  // kitchen of her own.
  await signIn(page)
  const mia = await newAccount(page, browser, 'Mia', miaEmail)
  const leo = await newAccount(page, browser, 'Leo', leoEmail)
  // Her own food, not a label's, which the catalog would show the other
  // tests' searches.
  const drink = await mia.request.post('/api/v1/foods', {
    data: {
      name: `Oat drink ${tag}`,
      basis: '100ml',
      source: 'user',
      nutrients: { kcal: 60, protein: 3.2, carbs: 4, fat: 3 },
    },
  })
  expect(drink.ok()).toBe(true)

  // Mia invites Leo from her household's page.
  await mia.goto('/settings/household')
  await mia.getByRole('button', { name: 'Invite someone' }).click()
  const sheet = mia.getByRole('dialog', { name: 'Invite someone' })
  await sheet.getByLabel('Their email address').fill(leoEmail)
  await sheet.getByRole('button', { name: 'Invite' }).click()
  await expect(mia.getByText(`Invited ${leoEmail}`)).toBeVisible()
  await expect(
    mia.getByRole('region', { name: 'Invites sent' }).getByRole('heading', { name: 'Leo' }),
  ).toBeVisible()

  // Leo sees it from the settings, what joining does, and joins.
  await leo.goto('/settings')
  const row = leo.getByRole('link', { name: 'Household' })
  await expect(row).toHaveAccessibleDescription('An invite waits for your answer')
  await row.click()
  const received = leo.getByRole('region', { name: 'Invites for you' })
  await expect(received.getByText('Mia invites you into their household')).toBeVisible()
  await received.getByRole('button', { name: 'See what joining does' }).click()
  const preview = leo.getByRole('dialog', { name: 'What joining does' })
  await expect(preview.getByText('1 food')).toBeVisible()
  await preview.getByRole('button', { name: 'Join the household' }).click()
  await expect(leo.getByText('You joined Mia’s household')).toBeVisible()
  const members = leo.getByRole('region', { name: 'Members' })
  await expect(members.getByText(miaEmail)).toBeVisible()
  await expect(members.getByText(leoEmail)).toBeVisible()

  // Mia's drink is his too.
  await leo.goto('/foods')
  await expect(leo.getByRole('link', { name: new RegExp(`Oat drink ${tag}`) })).toBeVisible()

  // He leaves, keeping it; Mia is in no household then.
  await leo.goto('/settings/household')
  await leo.getByRole('button', { name: 'Leave the household' }).click()
  await leo.getByRole('button', { name: 'Leave', exact: true }).click()
  await expect(leo.getByText('You left the household')).toBeVisible()
  await expect(leo.getByText(/You are in no household/)).toBeVisible()
  await mia.goto('/settings/household')
  await expect(mia.getByText(/You are in no household/)).toBeVisible()
  await leo.goto('/foods')
  await expect(leo.getByRole('link', { name: new RegExp(`Oat drink ${tag}`) })).toBeVisible()
  await mia.context().close()
  await leo.context().close()
})
