import { expect, test } from 'vitest'

import { plural, pluralIn } from '@/i18n/plural'

test("a count takes its language's form", () => {
  const minutes = { one: 'minutu', few: 'minute', other: 'minuta' }
  expect([1, 2, 5, 11, 21, 22, 25].map((n) => plural('hr', n, minutes))).toEqual([
    'minutu',
    'minute',
    'minuta',
    'minuta',
    'minutu',
    'minute',
    'minuta',
  ])
  expect([1, 2].map((n) => plural('en', n, { one: 'minute', other: 'minutes' }))).toEqual([
    'minute',
    'minutes',
  ])
})

test('a form the word lacks is its other', () => {
  expect(plural('hr', 2, { other: 'puta' })).toBe('puta')
})

test('a counted word takes its case', () => {
  const portion = {
    nominative: { one: 'porcija', few: 'porcije', other: 'porcija' },
    accusative: { one: 'porciju', few: 'porcije', other: 'porcija' },
  }
  expect([1, 2, 5, 21].map((n) => pluralIn('hr', n, 'accusative', portion))).toEqual([
    'porciju',
    'porcije',
    'porcija',
    'porciju',
  ])
  expect([1, 2, 5].map((n) => pluralIn('hr', n, 'nominative', portion))).toEqual([
    'porcija',
    'porcije',
    'porcija',
  ])
  // A word that keeps its form in a case has its nominative alone.
  const recipe = { nominative: { one: 'recept', few: 'recepta', other: 'recepata' } }
  expect(pluralIn('hr', 1, 'accusative', recipe)).toBe('recept')
})
