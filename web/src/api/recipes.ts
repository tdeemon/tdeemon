import { useQueries, useQueryClient } from '@tanstack/react-query'

import { inputOf } from '@/components/day/meal-entries'

import { $api } from './client'
import { statusOf } from './errors'
import { keep } from './foods'
import type { components } from './schema'
import type { Food } from './types'

export type Ingredient = components['schemas']['IngredientBody']
export type RecipeBody = components['schemas']['RecipeBody']
export type RecipeInput = components['schemas']['UpsertRecipeInputBody']
export type RecipeWarning = components['schemas']['RecipeWarningBody']

// useSaveRecipe saves a recipe of the user's, corrected or cooked again
// (upsert_recipe): its food, with the recipe, is kept, and the lists and
// the searches asked again.
export function useSaveRecipe() {
  const queryClient = useQueryClient()
  return $api.useMutation('post', '/api/v1/recipes', {
    onSuccess: (answer) => keep(queryClient, answer.food),
  })
}

// useCreateRecipe makes a new recipe; a food of its name, archived too,
// is in the way (409), not corrected.
export function useCreateRecipe() {
  const queryClient = useQueryClient()
  return $api.useMutation('post', '/api/v1/recipes/new', {
    onSuccess: (answer) => keep(queryClient, answer.food),
  })
}

// ingredientInput is an ingredient as a recipe saved again sends it: its
// food in the quantity it was given, which takes the food's values now.
// (inputOf reads an item of a meal, which an ingredient is but for its
// id, which it does not read.)
export function ingredientInput(ingredient: Ingredient) {
  return inputOf({ ...ingredient, id: ingredient.food_id })
}

// recipeInput is a recipe's food as upsert_recipe takes it to save it
// again as it is: by its id, its ingredients in the quantities they were
// given, which take their foods' values now, and archived as it is.
export function recipeInput(food: Food): RecipeInput {
  const recipe = food.recipe
  return {
    id: food.id,
    name: food.name,
    ...(food.aliases === undefined || food.aliases.length === 0 ? {} : { aliases: food.aliases }),
    items: (recipe?.items ?? []).map(ingredientInput),
    ...(recipe?.yield_g === undefined ? {} : { yield_g: recipe.yield_g }),
    ...(recipe?.portions === undefined ? {} : { portions: recipe.portions }),
    ...(recipe?.pot_g === undefined ? {} : { pot_g: recipe.pot_g }),
    ...(food.note ? { note: food.note } : {}),
    ...(food.archived ? { archived: true } : {}),
  }
}

// theirFood is the food a recipe's ingredients of one food stand for,
// when it is none of the user's to ask for: another member's, of a shared
// recipe (theirs), or one that keeps its values (a recipe the user left).
// Of the first's values per 100 g or 100 ml, or per portion with its
// portion's grams, and the servings their quantities name, so each
// amount is given as it was.
export function theirFood(ingredients: readonly [Ingredient, ...Ingredient[]]): Food {
  const first = ingredients[0]
  const { amount, unit } = first
  const per = unit === 'portion' ? 1 / amount : 100 / amount
  const n = first.nutrients
  const servings = new Map<string, number>()
  for (const i of ingredients) {
    const q = i.quantity
    if (q.servings !== undefined && q.servings > 0 && i.unit !== 'portion') {
      servings.set(q.serving ?? 'serving', i.amount / q.servings)
    }
  }
  return {
    id: first.food_id,
    name: first.name,
    ...(first.brand ? { brand: first.brand } : {}),
    basis: unit === 'portion' ? 'portion' : unit === 'ml' ? '100ml' : '100g',
    nutrients: {
      kcal: n.kcal * per,
      protein: n.protein * per,
      carbs: n.carbs * per,
      fat: n.fat * per,
    },
    ...(unit === 'portion' && first.portion_grams !== undefined
      ? { portion_grams: first.portion_grams }
      : {}),
    ...(servings.size > 0
      ? { servings: [...servings].map(([name, grams]) => ({ name, amount: grams })) }
      : {}),
    source: first.source,
    archived: false,
  }
}

// rowsOf are items' rows by their food's id.
function rowsOf(items: readonly Ingredient[]) {
  const rows = new Map<string, [Ingredient, ...Ingredient[]]>()
  for (const i of items) {
    const had = rows.get(i.food_id)
    if (had === undefined) {
      rows.set(i.food_id, [i])
    } else {
      had.push(i)
    }
  }
  return rows
}

// useIngredientFoods are the foods of a recipe's ingredients as they are
// now, which the recipe's editor gives amounts of: all of them once each
// came (foods), else whether one is still on its way (pending) or
// failed (error, and retry to ask again). Another member's, of a shared
// recipe, is none to ask for: theirFood stands for it.
export function useIngredientFoods(items: readonly Ingredient[]) {
  const rows = rowsOf(items)
  const ids = [...new Set(items.filter((i) => i.theirs !== true).map((i) => i.food_id))]
  const theirs = [...rowsOf(items.filter((i) => i.theirs === true)).values()]
  return useQueries({
    queries: ids.map((id) =>
      $api.queryOptions('get', '/api/v1/foods/{id}', { params: { path: { id } } }),
    ),
    combine: (results) => {
      const byId = new Map<string, Food>()
      for (const group of theirs) {
        byId.set(group[0].food_id, theirFood(group))
      }
      // A food of the user's no longer theirs to ask for (404: a recipe
      // they left) stands for itself as the recipe keeps it.
      const gone = (i: number) => statusOf(results[i]?.error) === 404
      results.forEach((r, i) => {
        const id = ids[i]
        const group = id === undefined ? undefined : rows.get(id)
        if (r.data !== undefined) {
          byId.set(r.data.id, r.data)
        } else if (gone(i) && group !== undefined) {
          byId.set(group[0].food_id, theirFood(group))
        }
      })
      const failed = results.find((r, i) => r.data === undefined && r.isError && !gone(i))
      return {
        foods: results.every((r, i) => r.data !== undefined || gone(i)) ? byId : undefined,
        error: failed?.error,
        retry: () => {
          for (const r of results) {
            if (r.isError) {
              void r.refetch()
            }
          }
        },
      }
    },
  })
}
