import { expect, test } from '@playwright/test'

import { signIn } from './user.ts'

// The command palette on a desktop: ⌘K or Ctrl+K from any screen, its
// rows by the keyboard, and typing on the day.
test.use({ viewport: { width: 1440, height: 900 }, isMobile: false, hasTouch: false })

test('Ctrl+K opens the palette from every section, keeps the focus in it, and Escape gives it back', async ({
  page,
}) => {
  await signIn(page)
  const sections = page.getByRole('navigation', { name: 'Sections' })
  // opensOn opens the palette on a section's page, and closes it.
  const opensOn = async (path: string, name: string) => {
    await page.goto(path)
    await expect(page.getByRole('heading', { name, level: 1, exact: true })).toBeVisible()
    const link = sections.getByRole('link', { name: 'Foods' })
    await link.focus()

    await page.keyboard.press('Control+k')
    const palette = page.getByRole('dialog', { name: 'Search and commands' })
    const field = palette.getByRole('combobox')
    await expect(field).toBeFocused()
    await expect(palette.getByRole('listbox')).toBeVisible()
    await expect(palette.getByRole('option', { name: 'Log a meal' })).toHaveAttribute(
      'aria-keyshortcuts',
      'N',
    )
    // Tab, and Shift+Tab, stay in the palette: its field is its one stop.
    await page.keyboard.press('Tab')
    await expect(field).toBeFocused()
    await page.keyboard.press('Shift+Tab')
    await expect(field).toBeFocused()
    await page.keyboard.press('Escape')
    await expect(palette).toBeHidden()
    await expect(link).toBeFocused()
  }
  await opensOn('/', 'Today')
  await opensOn('/foods', 'Foods')
  await opensOn('/progress', 'Progress')
  await opensOn('/plan', 'Plan')
  await opensOn('/settings/appearance', 'Appearance')
})

test('"weigh" and Enter open the weigh-in; the arrows choose a row, which the count says', async ({
  page,
}) => {
  await signIn(page)
  await page.goto('/foods')
  await expect(page.getByRole('heading', { name: 'Foods', level: 1 })).toBeVisible()

  await page
    .getByRole('navigation', { name: 'Sections' })
    .getByRole('button', { name: 'Search…' })
    .click()
  const palette = page.getByRole('dialog', { name: 'Search and commands' })
  const field = palette.getByRole('combobox')
  await expect(field).toBeFocused()

  // ↓ moves the active row, which the field names.
  const second = palette.getByRole('option').nth(1)
  await page.keyboard.press('ArrowDown')
  await expect(second).toHaveAttribute('aria-selected', 'true')
  await expect(field).toHaveAttribute(
    'aria-activedescendant',
    (await second.getAttribute('id')) ?? '',
  )

  await page.keyboard.type('weigh')
  await expect(palette.getByRole('status')).toHaveText('1 result')
  await page.keyboard.press('Enter')
  await expect(palette).toBeHidden()
  const sheet = page.getByRole('dialog', { name: 'Log a weigh-in' })
  await expect(sheet.getByLabel('Weight')).toBeFocused()
  await page.keyboard.press('Escape')
  await expect(sheet).toBeHidden()
})

test('typing on the day opens the palette with it; in a field, or with a shortcut, it does not', async ({
  page,
}) => {
  await signIn(page)
  await page.goto('/')
  await expect(page.getByRole('heading', { name: 'Today', exact: true })).toBeVisible()
  const palette = page.getByRole('dialog', { name: 'Search and commands' })

  // G, then D, is a section's: the day again, no palette.
  await page.keyboard.press('g')
  await page.keyboard.press('d')
  await expect(palette).toBeHidden()

  await page.keyboard.type('yest')
  await expect(palette.getByRole('combobox')).toHaveValue('yest')
  await expect(palette.getByRole('option', { name: /^Yesterday · / })).toBeVisible()
  await page.keyboard.press('Enter')
  await expect(page).toHaveURL(/\?date=\d{4}-\d{2}-\d{2}$/)

  // A field's keys are its own, Ctrl+K among them.
  await page.goto('/foods')
  const search = page.getByRole('searchbox', { name: 'Search the foods' })
  await search.fill('')
  await search.press('o')
  await search.press('Control+k')
  await expect(palette).toBeHidden()
  await expect(search).toHaveValue('o')
})
