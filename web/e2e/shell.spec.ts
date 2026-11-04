import { expect, test } from '@playwright/test'

import { signIn } from './user.ts'

test('a phone has the bar at the bottom, a tablet the rail, a desktop the sidebar, and the keys work there', async ({
  page,
}) => {
  await signIn(page)
  const sections = page.getByRole('navigation', { name: 'Sections' })
  const mark = page.getByRole('main').getByRole('link', { name: 'Settings and account' })

  // A phone: the bar's sections, Log a meal in its middle; the settings
  // the user's mark's, in the page's header.
  const viewport = page.viewportSize()
  const bar = await sections.boundingBox()
  expect(bar && viewport && Math.round(bar.y + bar.height)).toBe(viewport?.height)
  await expect(sections.getByRole('link', { name: 'Progress' })).toBeVisible()
  await expect(sections.getByRole('link', { name: 'Settings' })).toBeHidden()
  await expect(mark).toBeVisible()
  await sections.getByRole('link', { name: 'Foods' }).click()
  await sections.getByRole('button', { name: 'Log a meal' }).click()
  const sheet = page.getByRole('dialog', { name: /^(Breakfast|Lunch|Snack|Dinner)$/ })
  await expect(sheet).toBeVisible()
  // A phone's keyboard waits for a tap on the search; the foods stay
  // under it.
  await expect(sheet).toBeFocused()
  await expect(page).toHaveURL('/foods')
  // The bar steps down out of the way of the sheet, and back after it
  // (under the sheet, it is hidden from a screen reader).
  const under = page.getByRole('navigation', { name: 'Sections', includeHidden: true })
  await expect
    .poll(async () => Math.round((await under.boundingBox())?.y ?? 0), {
      message: 'the bar under the screen',
    })
    .toBe(viewport?.height)
  await page.keyboard.press('Escape')
  await expect(sheet).toBeHidden()
  await expect
    .poll(async () => Math.round((await sections.boundingBox())?.y ?? 0), {
      message: 'the bar back',
    })
    .toBe(bar && Math.round(bar.y))
  await mark.click()
  await expect(page).toHaveURL('/settings')
  await expect(page.getByRole('heading', { name: 'Settings', level: 1 })).toBeVisible()

  // A tablet, upright and on its side: the rail, 72 px at the left, Log a
  // meal first and the settings at its foot; the user's mark in the
  // header is the rail's. The day is in two columns, the body beside
  // the meals.
  const tablet = async (width: number) => {
    await page.setViewportSize({ width, height: 900 })
    await sections.getByRole('link', { name: 'Today' }).click()
    const rail = await sections.boundingBox()
    expect(rail && [rail.x, rail.width, rail.height]).toEqual([0, 72, 900])
    await expect(sections.getByRole('button', { name: 'Log a meal' })).toBeVisible()
    await expect(sections.getByRole('link', { name: 'Settings' })).toBeVisible()
    await expect(sections.getByRole('button', { name: 'Search…' })).toBeHidden()
    await expect(sections.getByRole('button', { name: 'Dark theme' })).toBeHidden()
    await expect(mark).toBeHidden()
    const meals = await page.getByRole('heading', { name: 'Meals', level: 2 }).boundingBox()
    const body = await page.getByRole('region', { name: 'Body today' }).boundingBox()
    expect(meals && body && body.x - meals.x > 250).toBe(true)
  }
  await tablet(768)
  await tablet(1024)

  // A desktop: the sidebar, 240 px at the left, the way to search at its
  // top, the settings at its foot.
  await page.setViewportSize({ width: 1440, height: 900 })
  const side = await sections.boundingBox()
  expect(side && [side.x, side.width, side.height]).toEqual([0, 240, 900])
  await expect(sections.getByRole('link', { name: 'Settings' })).toBeVisible()
  await expect(sections.getByRole('button', { name: 'Dark theme' })).toBeVisible()
  await expect(sections.getByRole('button', { name: 'Search…' })).toBeVisible()

  await sections.getByRole('link', { name: 'Today' }).click()
  await page.getByRole('heading', { name: 'Today', exact: true }).click()
  await page.keyboard.press('/')
  await expect(page.getByRole('searchbox', { name: 'Search the foods' })).toBeFocused()
  await page.keyboard.press('Escape')
  await page.getByRole('heading', { name: 'Foods' }).click()
  await page.keyboard.press('?')
  const dialog = page.getByRole('dialog', { name: 'Keyboard shortcuts' })
  await expect(dialog).toBeVisible()
  await page.keyboard.press('Escape')
  await expect(dialog).toBeHidden()
})

test("a phone's form has the whole screen, without the bar of sections", async ({ page }) => {
  await signIn(page)
  const sections = page.getByRole('navigation', { name: 'Sections' })

  const form = async (path: string) => {
    await page.goto(path)
    await expect(page.getByRole('heading', { level: 1 })).toBeVisible()
    await expect(sections).toBeHidden()
  }
  const food = await page.request.post('/api/v1/foods', {
    data: {
      name: `Kefir ${String(Date.now())}`,
      basis: '100g',
      source: 'label',
      nutrients: { kcal: 60, protein: 3.4, carbs: 4.5, fat: 3.2 },
    },
  })
  expect(food.ok()).toBe(true)
  const body: unknown = await food.json()
  const id = typeof body === 'object' && body !== null && 'id' in body ? String(body.id) : ''
  await form('/foods/new')
  await form('/foods/new-recipe')
  await form(`/foods/${id}?edit=true`)
  await form('/plan/profile')
  await form('/plan/limits')
  await form('/plan/notes')
  // Back on a list, the bar is back.
  await page.getByRole('main').getByRole('link', { name: 'Plan' }).click()
  await expect(page).toHaveURL('/plan')
  await expect(sections).toBeVisible()
})

test('the addresses of before lead where their pages are now', async ({ page }) => {
  await signIn(page)
  await page.goto('/charts?days=30')
  await expect(page).toHaveURL('/progress?days=30')
  await expect(page.getByRole('heading', { name: 'Progress' })).toBeVisible()
  await page.goto('/settings/plan')
  await expect(page).toHaveURL('/plan')
  await expect(page.getByRole('heading', { name: 'Plan', level: 1 })).toBeVisible()
  // The settings are a list now, each a row to its page, which leads
  // back to them.
  await page.goto('/settings')
  await expect(page.getByRole('heading', { name: 'Settings', level: 1 })).toBeVisible()
  await page.getByRole('link', { name: 'Devices and AI coaches' }).click()
  await expect(page).toHaveURL('/settings/devices')
  await page.getByRole('main').getByRole('link', { name: 'Settings' }).click()
  await expect(page).toHaveURL('/settings')
})

test('a window too low for the rail keeps the bar, and the settings a tap away', async ({
  page,
}) => {
  await signIn(page)
  const sections = page.getByRole('navigation', { name: 'Sections' })
  const mark = page.getByRole('main').getByRole('link', { name: 'Settings and account' })

  // A phone on its side, and a desktop zoomed in to 200 %: wide, but not
  // high enough for the rail's sections and settings.
  const low = async (width: number, height: number) => {
    await page.setViewportSize({ width, height })
    await page.goto('/plan')
    await expect(page.getByRole('heading', { name: 'Plan', level: 1 })).toBeVisible()
    const bar = await sections.boundingBox()
    expect(bar && Math.round(bar.y + bar.height)).toBe(height)
    expect(bar?.width).toBe(width)
    await mark.click()
    await expect(page).toHaveURL('/settings')
  }
  await low(844, 390)
  await low(640, 384)

  // A desktop's window made low: another day's header, a desktop's, the
  // foods beside their list, and the progress have the mark too, to the
  // settings' first page beside their list.
  await page.setViewportSize({ width: 1440, height: 400 })
  const reaches = async (path: string) => {
    await page.goto(path)
    await mark.click()
    await expect(page).toHaveURL('/settings/appearance')
  }
  await page.goto('/')
  await page.getByRole('link', { name: 'The day before' }).click()
  await mark.click()
  await expect(page).toHaveURL('/settings/appearance')
  await reaches('/foods')
  await reaches('/progress')
})
