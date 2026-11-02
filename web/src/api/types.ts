import type { components } from './schema'

// The API's types the app passes around, by their names in the app.
export type Day = components['schemas']['DayBody']
export type Meal = components['schemas']['MealBody']
// A meal of before, with when it was eaten each time.
export type RecentMeal = components['schemas']['RecentMealBody']
export type Eaten = components['schemas']['EatenBody']
export type Item = components['schemas']['ItemBody']
export type Metric = components['schemas']['MetricBody']
export type Nutrients = components['schemas']['NutrientsBody']
export type Warning = components['schemas']['WarningBody']
export type Food = components['schemas']['FoodBody']
// A food of the instance's catalog: no one's, its id the catalog's.
export type CatalogFood = components['schemas']['CatalogFoodBody']

// catalogFood is a food of the catalog as the app shows a food: in use,
// never eaten; an entry of it is logged by its catalog_id (FoodEntry).
export function catalogFood(food: CatalogFood): Food {
  return { ...food, archived: false }
}
