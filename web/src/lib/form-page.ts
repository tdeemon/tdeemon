import type { ParsedLocation } from '@tanstack/react-router'

// formPages are the pages of a form, a food's or a recipe's, new or
// corrected, the profile, the limits and the notes for a coach: a task of
// its own, which a phone gives the whole screen, without the bar to leave
// it by.
const formPages = new Set([
  '/foods/new',
  '/foods/new-recipe',
  '/plan/profile',
  '/plan/limits',
  '/plan/notes',
])

// formPage tells a form's page at location.
export function formPage(location: Pick<ParsedLocation, 'pathname' | 'search'>): boolean {
  const path = location.pathname.replace(/\/$/, '')
  const search: Record<string, unknown> = location.search
  return formPages.has(path) || (path.startsWith('/foods/') && search.edit === true)
}
