import { expect, test } from 'vitest'

import { theirFood, type Ingredient } from './recipes'

const slice: Ingredient = {
  food_id: '0193c3a5-0000-7000-8000-0000000000b1',
  name: 'Bread',
  quantity: { servings: 2, serving: 'slice' },
  amount: 80,
  unit: 'g',
  nutrients: { kcal: 200, protein: 7, carbs: 38, fat: 2 },
  source: 'label',
  theirs: true,
}

test("theirFood is of the first row's values, with every row's serving", () => {
  const cup: Ingredient = { ...slice, quantity: { servings: 1, serving: 'cup' }, amount: 120 }
  const f = theirFood([slice, cup])
  expect(f.basis).toBe('100g')
  expect(f.nutrients.kcal).toBe(250)
  expect(f.servings).toEqual([
    { name: 'slice', amount: 40 },
    { name: 'cup', amount: 120 },
  ])
})

test("theirFood of portions keeps a portion's grams", () => {
  const f = theirFood([
    { ...slice, quantity: { portions: 2 }, amount: 2, unit: 'portion', portion_grams: 150 },
  ])
  expect(f.basis).toBe('portion')
  expect(f.nutrients.kcal).toBe(100)
  expect(f.portion_grams).toBe(150)
  expect(f.servings).toBeUndefined()
})
