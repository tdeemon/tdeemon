import { expect, test } from 'vitest'

import { formPage } from '@/lib/form-page'

// at tells a form's page at pathname and search.
function at(pathname: string, search: Record<string, unknown> = {}): boolean {
  return formPage({ pathname, search })
}

test("a form's page is a food's or a recipe's, new or corrected, the profile, the limits or the notes", () => {
  expect(at('/foods/new')).toBe(true)
  expect(at('/foods/new-recipe')).toBe(true)
  expect(at('/foods/0193c3a5-0000-7000-8000-0000000000a1', { edit: true })).toBe(true)
  expect(at('/plan/profile')).toBe(true)
  expect(at('/plan/limits/')).toBe(true)
  expect(at('/plan/notes')).toBe(true)

  // A food read, the lists, the sections and the settings are none.
  expect(at('/foods/0193c3a5-0000-7000-8000-0000000000a1')).toBe(false)
  expect(at('/foods', { edit: true })).toBe(false)
  expect(at('/')).toBe(false)
  expect(at('/plan')).toBe(false)
  expect(at('/plan/autopilot')).toBe(false)
  expect(at('/settings/account')).toBe(false)
})
