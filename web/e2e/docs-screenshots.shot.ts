import { expect, test, type Page } from '@playwright/test'

import { demo } from './user.ts'

// The docs' screenshots (make screenshots, playwright.screenshots.config.ts):
// each screen of the demo user's made-up data, saved to docs/screenshots
// as <device>-<screen>-<theme>.png. Nothing in them is anyone's.

const out = '../docs/screenshots'

type Theme = 'light' | 'dark'

const phone = {
  viewport: { width: 390, height: 844 },
  deviceScaleFactor: 2,
  isMobile: true,
  hasTouch: true,
}
const desktop = {
  viewport: { width: 1440, height: 900 },
  deviceScaleFactor: 1,
  isMobile: false,
  hasTouch: false,
}

// open signs the demo user in, in a theme and a language, and goes to path.
async function open(page: Page, theme: Theme, path = '/', locale: 'en' | 'hr' = 'en') {
  await page.emulateMedia({ colorScheme: theme, reducedMotion: 'reduce' })
  await page.addInitScript((t) => {
    localStorage.setItem('tdeemon.theme', t)
  }, theme)
  const signedIn = await page.request.post('/api/v1/auth/login', {
    data: { email: demo.email, password: demo.password },
  })
  expect(signedIn.ok()).toBe(true)
  const me = await page.request.patch('/api/v1/me', { data: { locale } })
  expect(me.ok()).toBe(true)
  await page.goto(path)
}

// settle waits for a screen to finish: no skeleton, no request, no
// animation of a chart.
async function settle(page: Page) {
  await page.waitForLoadState('networkidle')
  await expect(page.locator('[aria-busy="true"]')).toHaveCount(0)
  await page.waitForTimeout(800)
}

async function shot(page: Page, name: string, fullPage = false) {
  await settle(page)
  await page.screenshot({ path: `${out}/${name}.png`, fullPage })
}

const heading = (page: Page, name: string) =>
  expect(page.getByRole('heading', { name, level: 1, exact: true })).toBeVisible()

test.describe('desktop', () => {
  test.use(desktop)

  for (const theme of ['light', 'dark'] as const) {
    test(`the day, the progress and the plan, ${theme}`, async ({ page }) => {
      await open(page, theme)
      await heading(page, 'Today')
      await shot(page, `desktop-day-${theme}`)
      await page.goto('/progress')
      await heading(page, 'Progress')
      await shot(page, `desktop-progress-${theme}`)
      await page.goto('/plan')
      await heading(page, 'Plan')
      await shot(page, `desktop-plan-${theme}`)
    })
  }

  test('the foods and a recipe', async ({ page }) => {
    await open(page, 'light', '/foods')
    await heading(page, 'Foods')
    await page
      .getByRole('link', { name: /^Red lentil soup/ })
      .first()
      .click()
    await expect(page.getByRole('heading', { name: 'Red lentil soup' })).toBeVisible()
    await shot(page, 'desktop-foods-light')
  })

  test('the command palette', async ({ page }) => {
    await open(page, 'dark')
    await heading(page, 'Today')
    await settle(page)
    await page.keyboard.press('Control+k')
    const palette = page.getByRole('dialog', { name: 'Search and commands' })
    await palette.getByRole('combobox').fill('yog')
    await shot(page, 'desktop-palette-dark')
  })
})

test.describe('phone', () => {
  test.use(phone)

  for (const theme of ['light', 'dark'] as const) {
    test(`the day, ${theme}`, async ({ page }) => {
      await open(page, theme)
      await heading(page, 'Today')
      await shot(page, `phone-day-${theme}`)
    })
  }

  test('a meal logged', async ({ page }) => {
    await open(page, 'light')
    await heading(page, 'Today')
    await settle(page)
    await page
      .getByRole('navigation', { name: 'Sections' })
      .getByRole('button', { name: /Log a meal/ })
      .click()
    const sheet = page.getByRole('dialog', { name: /^(Breakfast|Lunch|Snack|Dinner)$/ })
    await sheet.getByLabel('Find a food or a meal').fill('yog')
    await expect(sheet.getByRole('button', { name: /^Greek yoghurt/ }).first()).toBeVisible()
    await shot(page, 'phone-log-light')
  })

  test('the progress', async ({ page }) => {
    await open(page, 'dark', '/progress')
    await heading(page, 'Progress')
    await shot(page, 'phone-progress-dark')
  })

  test('a proposal', async ({ page }) => {
    await open(page, 'light', '/plan')
    await heading(page, 'Plan')
    await shot(page, 'phone-plan-light')
  })

  test('a recipe', async ({ page }) => {
    await open(page, 'light', '/foods')
    await heading(page, 'Foods')
    await page
      .getByRole('link', { name: /^Red lentil soup/ })
      .first()
      .click()
    await expect(page.getByRole('heading', { name: 'Red lentil soup' })).toBeVisible()
    await shot(page, 'phone-recipe-light')
  })

  test('the autopilot', async ({ page }) => {
    await open(page, 'dark', '/plan/autopilot')
    await shot(page, 'phone-autopilot-dark')
  })

  test('the day in Croatian', async ({ page }) => {
    await open(page, 'dark', '/', 'hr')
    await heading(page, 'Danas')
    await shot(page, 'phone-day-hr-dark')
  })
})
