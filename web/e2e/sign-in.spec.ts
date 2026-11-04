import { expect, test } from '@playwright/test'

import { toSettings, user } from './user.ts'

test('a user signs in, moves between the sections and signs out', async ({ page }) => {
  await page.goto('/settings')
  await expect(page.getByRole('heading', { name: 'Sign in' })).toBeVisible()

  await page.getByLabel('Email').fill(user.email)
  await page.getByLabel('Password').fill(user.password)
  await page.getByRole('button', { name: 'Sign in' }).click()

  await expect(page.getByRole('heading', { name: 'Settings' })).toBeVisible()
  // Whose they are: the account's card, and signing out as whom.
  await expect(page.getByRole('main').getByText('Ana', { exact: true })).toBeVisible()
  await expect(page.getByRole('button', { name: 'Sign out' })).toHaveAccessibleDescription(
    user.email,
  )
  const sections = page.getByRole('navigation', { name: 'Sections' })
  await sections.getByRole('link', { name: 'Foods' }).click()
  await expect(page.getByRole('heading', { name: 'Foods' })).toBeVisible()
  await expect(page).toHaveURL('/foods')
  await toSettings(page)

  await page.getByRole('button', { name: 'Sign out' }).click()
  await expect(page.getByRole('heading', { name: 'Sign in' })).toBeVisible()
  await page.goto('/')
  await expect(page.getByRole('heading', { name: 'Sign in' })).toBeVisible()
})

test('a wrong password is refused', async ({ page }) => {
  await page.goto('/login')

  await page.getByLabel('Email').fill(user.email)
  await page.getByLabel('Password').fill('not the password')
  await page.getByRole('button', { name: 'Sign in' }).click()

  await expect(page.getByRole('alert')).toHaveText('Wrong email or password.')
  await expect(page).toHaveURL('/login')
})
