import { expect, test } from '@playwright/test'

import { signIn, toSettings } from './user.ts'

test('the admin invites someone, whose link makes their account and signs it in', async ({
  page,
  browser,
}) => {
  const email = `nina-${String(Date.now())}@example.com`
  await signIn(page)
  await toSettings(page)
  await page.getByRole('link', { name: 'Users' }).click()
  await expect(page.getByRole('heading', { name: 'Users', level: 1 })).toBeVisible()

  await page.getByRole('button', { name: 'Invite someone' }).click()
  const sheet = page.getByRole('dialog', { name: 'Invite someone' })
  await sheet.getByLabel('Email').fill(email)
  await sheet.getByRole('button', { name: 'Make the invite link' }).click()
  const link = sheet.getByRole('group', { name: 'The invite link' }).locator('code')
  await expect(link).toContainText('/invite#')
  const url = await link.innerText()
  await sheet.getByRole('button', { name: 'Done' }).click()
  await expect(page.getByRole('region', { name: 'Invites' }).getByText(email)).toBeVisible()

  // Someone else, in a browser of their own.
  const theirs = await browser.newContext({ serviceWorkers: 'block' })
  const invited = await theirs.newPage()
  await invited.goto(url)
  await expect(invited.getByText(`Your account will sign in as ${email}.`)).toBeVisible()
  await invited.getByLabel('Name').fill('Nina')
  await invited.getByLabel('Password').fill('a password of her own')
  await invited.getByRole('button', { name: 'Make the account' }).click()
  await expect(invited.getByRole('heading', { name: 'Today', exact: true })).toBeVisible()
  await theirs.close()

  // The invite was taken: the admin sees the account, and the invite gone.
  await page.reload()
  await expect(page.getByRole('region', { name: 'Accounts' }).getByText(email)).toBeVisible()
  await expect(page.getByRole('region', { name: 'Invites' }).getByText(email)).toHaveCount(0)
})
