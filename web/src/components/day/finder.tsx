import { Tabs } from '@base-ui/react/tabs'
import { Check, Plus, Repeat2, ScanBarcode, Search, X } from 'lucide-react'
import { Fragment, useEffect, useId, useRef, useState, type RefObject } from 'react'
import { flushSync } from 'react-dom'

import { $api } from '@/api/client'
import { useFindFoods, useFrequentFoods } from '@/api/foods'
import { useRecentMeals } from '@/api/meals'
import { catalogFood, type Food, type Meal, type RecentMeal } from '@/api/types'
import {
  entryAmount,
  entryAmountLabel,
  entryValues,
  entryWrong,
  foodEntry,
  inUnit,
  isShare,
  sayWrong,
  unitGrams,
  unitsOf,
  type Entry,
  type FoodEntry,
  type Wrong,
} from '@/components/day/meal-entries'
import { longName, mealName } from '@/components/day/meal-slot'
import type { Making } from '@/components/day/new-entry'
import { pastMeals } from '@/components/day/past-meals'
import { suggestFoods, suggestMeals, type Moment } from '@/components/day/suggest'
import { entryUnitLabel, isRecipe } from '@/components/recipes/recipe-values'
import { portionServing, shareOf, wholeServing } from '@/components/recipes/share'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { useFormat } from '@/i18n/format'
import { useMessages } from '@/i18n/messages'
import { touchScreen, useDesktop } from '@/lib/media'
import { matches, parts } from '@/lib/search'
import { activeChip, chipClass, inputClass } from '@/lib/styles'
import { cn } from '@/lib/utils'

// The lists a meal's foods are found in.
const lists = ['all', 'recent', 'frequent', 'meals'] as const
type List = (typeof lists)[number]

function isList(value: unknown): value is List {
  return lists.some((l) => l === value)
}

// How many of the recent meals and foods the first list shows: the meals
// a search finds, and the foods with nothing searched for.
const firstMeals = 3
const firstFoods = 12

// Finder finds what goes in a meal being logged: the foods of the user's
// a search (query) finds, the catalog's after them, or those eaten lately
// or most, and the meals of before. Each food says what its + adds, the
// amount it was eaten in last (or its default) and its kcal; a tap on
// the food opens it in its row for another amount. The first list is
// the meal of the day's (at, suggestMeals and suggestFoods): with
// nothing searched for, its meals as cards on top, repeated in a tap,
// then the foods, none of them a card's again; the other meals are in
// Meals. A food in the meal already (chosen) has its + checked, and a
// tap on it takes it out again (onUnchoose). A new food, an estimate (its
// kcal chip) or a barcode is the sheet's next step (onMake, with the
// button that asks for it). onAdd takes an entry, checked; onRepeat a
// meal to log again. search is the field the sheet's focus goes to. With
// stay, the search stays as it was once a food is added, for the next of
// the same results, its words selected: a keystroke types anew. For a
// recipe's ingredients (ingredient), it finds foods alone, never the
// recipe itself (exclude), and makes no estimate: an ingredient is a
// food. A food it opens with (initial) is picked, first in the list
// until it is added, its amount's field amount.
export function Finder({
  search,
  query,
  onQuery,
  at,
  onAdd,
  onRepeat,
  onMake,
  ingredient,
  initial,
  amount,
  chosen,
  onUnchoose,
  onLog,
  stay = false,
}: {
  search: RefObject<HTMLInputElement | null>
  query: string
  onQuery: (query: string) => void
  at: Moment
  onAdd: (entry: Entry) => void
  onRepeat?: (meal: Meal) => void
  onMake: (making: Making, button: HTMLElement) => void
  ingredient?: { exclude?: string | undefined }
  initial?: Food | undefined
  amount?: RefObject<HTMLInputElement | null>
  chosen?: ReadonlySet<string>
  onUnchoose?: (food: Food) => void
  onLog?: (entry: Entry) => void
  stay?: boolean
}) {
  const t = useMessages()
  const f = useFormat()
  const [list, setList] = useState<List>('all')
  // The food picked, with its amount, which opens in its row.
  const [picked, setPicked] = useState<FoodEntry | undefined>(() =>
    initial === undefined ? undefined : foodEntry(initial, f.field),
  )
  const [pickedWrong, setPickedWrong] = useState<Wrong | undefined>(undefined)
  const results = useRef<HTMLDivElement>(null)
  // A food added since the search was typed in: its words are selected
  // as it takes the focus, so the next search replaces them.
  const stale = useRef(false)
  // Two cards side by side on a phone, three on a desktop's sheet.
  const cards = useDesktop() ? 3 : 2
  const found = useFindFoods(query)
  const recentFoods = $api.useQuery('get', '/api/v1/foods/list', {
    params: { query: { order: 'use', limit: 50 } },
  })
  const frequentFoods = useFrequentFoods(list === 'frequent')
  const recentMeals = useRecentMeals(ingredient === undefined)
  const q = query.trim()
  // A search's foods of the catalog, after the user's: logged, theirs.
  const fromCatalog = q === '' ? [] : (found.data?.catalog ?? []).map(catalogFood)
  const catalog = new Set(fromCatalog.map((food) => food.id))
  const shownLists = ingredient === undefined ? lists : lists.filter((l) => l !== 'meals')
  const repeat = (meal: Meal) => {
    onRepeat?.(meal)
  }
  // A food the finder never offers: the recipe its ingredients are of.
  const offered = (food: Food) => food.id !== ingredient?.exclude

  // eaten are the foods of a list eaten before, that the search matches.
  const eaten = (foods: Food[] | undefined) =>
    (foods ?? []).filter(
      (food) =>
        offered(food) &&
        food.last_use !== undefined &&
        matches(q, [food.name, food.brand, ...(food.aliases ?? [])]),
    )
  // The meals the search matches, by their words or their items' names
  // and brands, each once: of the meals of the same foods, the latest it
  // matches; those eaten at the meal of the day lately first.
  const ranked = suggestMeals(
    pastMeals(
      (ingredient === undefined ? (recentMeals.data?.meals ?? []) : []).filter((m) =>
        matches(q, [m.description, ...m.items.flatMap((i) => [i.name, i.brand])]),
      ),
    ),
    at,
  )
  // Nothing searched for, the first meals are cards, on top of the first
  // list: those of the meal of the day, then the others the day has not
  // had. Meals lists them all.
  const suggested = q === '' ? ranked.meals.slice(0, Math.min(ranked.fresh, cards)) : []
  const meals = ranked.meals
  // A card of one food says that food: it is not listed again.
  const carded = new Set(
    suggested.flatMap((m) => {
      const [only, ...rest] = m.items
      return only?.food_id !== undefined && rest.length === 0 ? [only.food_id] : []
    }),
  )
  // The food it opened with, while it is picked and nothing is searched
  // for: first, whether it was eaten lately or not.
  const pinned = initial !== undefined && picked?.food.id === initial.id ? [picked.food] : []
  const shown: Record<List, { foods: Food[]; meals: Meal[]; pending: boolean }> = {
    all:
      q === ''
        ? {
            foods: [
              ...pinned,
              ...suggestFoods(eaten(recentFoods.data?.foods), recentMeals.data?.meals ?? [], at)
                .filter((food) => !pinned.some((p) => p.id === food.id) && !carded.has(food.id))
                .slice(0, firstFoods),
            ],
            meals: [],
            pending: recentFoods.isPending || (ingredient === undefined && recentMeals.isPending),
          }
        : {
            foods: [...(found.data?.foods ?? []).filter(offered), ...fromCatalog],
            meals: meals.slice(0, firstMeals),
            pending: found.isPending,
          },
    recent: { foods: eaten(recentFoods.data?.foods), meals: [], pending: recentFoods.isPending },
    frequent: {
      foods: eaten(frequentFoods.data?.foods),
      meals: [],
      pending: frequentFoods.isPending,
    },
    meals: { foods: [], meals, pending: recentMeals.isPending },
  }

  // entryOf is a food's entry, of the catalog's food as one.
  const entryOf = (food: Food): FoodEntry => {
    const entry = foodEntry(food, f.field)
    return catalog.has(food.id) ? { ...entry, catalog: true } : entry
  }

  function pick(food: Food) {
    setPicked(entryOf(food))
    setPickedWrong(undefined)
  }

  // add adds an entry checked. From its amount's row (opened), the
  // focus goes on: with stay, to the search, its words selected, but on
  // a touch screen, whose keyboard would cover the lists, to the food's
  // + again.
  function add(entry: FoodEntry, opened = false) {
    const wrong = entryWrong(entry, f.parse)
    if (wrong !== undefined) {
      setPicked(entry)
      setPickedWrong(wrong)
      return
    }
    if (!stay) {
      onAdd(entry)
      setPicked(undefined)
      setPickedWrong(undefined)
      return
    }
    stale.current = true
    flushSync(() => {
      onAdd(entry)
      setPicked(undefined)
      setPickedWrong(undefined)
    })
    if (!opened) {
      return
    }
    if (touchScreen()) {
      results.current
        ?.querySelector<HTMLElement>(`[data-add="${CSS.escape(entry.food.id)}"]`)
        ?.focus()
    } else {
      search.current?.focus()
      search.current?.select()
    }
  }

  const pickedRow = (food: Food) =>
    picked?.food.id === food.id ? (
      <Amount
        entry={picked}
        field={amount}
        wrong={pickedWrong}
        onChange={(e) => {
          setPicked(e)
          setPickedWrong(undefined)
        }}
        onAdd={(e) => {
          add(e, true)
        }}
        onLog={
          onLog === undefined
            ? undefined
            : (e) => {
                const wrong = entryWrong(e, f.parse)
                if (wrong !== undefined) {
                  setPickedWrong(wrong)
                  return
                }
                setPicked(undefined)
                onLog(e)
              }
        }
      />
    ) : undefined

  const resultsOf = (l: List) => (
    <Results
      {...shown[l]}
      query={q}
      timeZone={at.timeZone}
      // The cards have said what there is.
      quiet={l === 'all' && suggested.length > 0}
      picked={picked?.food.id}
      pinned={pinned[0]?.id}
      pickedRow={pickedRow}
      onPick={(food) => {
        if (picked?.food.id === food.id) {
          setPicked(undefined)
        } else {
          pick(food)
        }
      }}
      onAdd={(food) => {
        if (chosen?.has(food.id) === true && onUnchoose !== undefined) {
          onUnchoose(food)
        } else {
          add(entryOf(food))
        }
      }}
      catalog={catalog}
      chosen={chosen}
      onRepeat={repeat}
      ingredient={ingredient !== undefined}
    />
  )

  return (
    <div ref={results} className="space-y-3">
      {/* The search, and beside it a barcode's: the camera, or its digits. */}
      <div className="flex h-12 items-center rounded-lg border border-control-border bg-card pr-0.5 focus-within:border-ring focus-within:ring-1 focus-within:ring-ring">
        <label className="flex h-full min-w-0 flex-1 items-center gap-2.5 pl-3.5">
          <Search aria-hidden className="size-4.5 shrink-0 text-muted-foreground" />
          <span className="sr-only">{t.log.search}</span>
          <input
            ref={search}
            type="search"
            value={query}
            placeholder={t.log.findHint}
            autoComplete="off"
            className="h-full min-w-0 flex-1 bg-transparent text-base outline-none [&::-webkit-search-cancel-button]:hidden"
            onChange={(e) => {
              stale.current = false
              onQuery(e.currentTarget.value)
            }}
            onFocus={(e) => {
              if (stale.current) {
                stale.current = false
                e.currentTarget.select()
              }
            }}
            onKeyDown={(e) => {
              // Enter opens the first food found, for its amount; it sends
              // nothing.
              if (e.key === 'Enter' && !e.metaKey && !e.ctrlKey) {
                e.preventDefault()
                const first = shown[list].foods[0]
                if (first !== undefined) {
                  pick(first)
                }
              }
            }}
          />
        </label>
        {query !== '' && (
          <Button
            variant="ghost"
            size="icon"
            className="shrink-0 text-muted-foreground"
            aria-label={t.log.clear}
            onClick={() => {
              stale.current = false
              onQuery('')
              search.current?.focus()
            }}
          >
            <X aria-hidden />
          </Button>
        )}
        <Button
          variant="ghost"
          size="icon"
          className="shrink-0"
          aria-label={t.scan.title}
          onClick={(e) => {
            onMake('scan', e.currentTarget)
          }}
        >
          <ScanBarcode aria-hidden />
        </Button>
      </div>
      <Tabs.Root
        value={list}
        onValueChange={(value) => {
          if (isList(value)) {
            setList(value)
          }
        }}
      >
        {/* The lists, and an estimate's kcal at the end of their row. */}
        <div className="flex flex-wrap gap-x-1.5 gap-y-3">
          <Tabs.List aria-label={t.log.lists} className="flex flex-wrap gap-x-1.5 gap-y-3">
            {shownLists.map((l) => (
              <Tabs.Tab key={l} value={l} className={cn(chipClass, activeChip)}>
                {l === list && <Check aria-hidden className="size-4 shrink-0" />}
                {l === 'all' && ingredient === undefined ? t.log.forSlot[at.slot] : t.log.list[l]}
              </Tabs.Tab>
            ))}
          </Tabs.List>
          {ingredient === undefined && (
            <button
              type="button"
              aria-label={t.log.quickKcal}
              className={cn(
                chipClass,
                'hover:bg-muted focus-visible:ring-2 focus-visible:ring-ring focus-visible:ring-offset-2 focus-visible:ring-offset-background',
              )}
              onClick={(e) => {
                onMake('estimate', e.currentTarget)
              }}
            >
              <Plus aria-hidden className="size-4 shrink-0" />
              kcal
            </button>
          )}
        </div>
        {shownLists.map((l) => (
          <Tabs.Panel
            key={l}
            value={l}
            className="space-y-3 rounded-md pt-3 outline-none focus-visible:ring-2 focus-visible:ring-ring"
          >
            {l === 'all' && suggested.length > 0 && (
              <>
                <ul
                  aria-label={t.log.picks}
                  className={cn('grid gap-2', cards === 3 ? 'grid-cols-3' : 'grid-cols-2')}
                >
                  {suggested.map((meal) => (
                    <Suggestion key={meal.id} meal={meal} onRepeat={repeat} />
                  ))}
                </ul>
                {shown.all.foods.length > 0 && (
                  <h3 className="text-caption font-semibold tracking-wide text-muted-foreground uppercase">
                    {t.log.foodsAndRecipes}
                  </h3>
                )}
              </>
            )}
            {resultsOf(l)}
          </Tabs.Panel>
        ))}
      </Tabs.Root>
      <Button
        variant="outline"
        className="h-auto min-h-11 w-full border-dashed py-2 whitespace-normal"
        onClick={(e) => {
          onMake('food', e.currentTarget)
        }}
      >
        <Plus aria-hidden />
        {t.log.newFood}
      </Button>
    </div>
  )
}

// Suggestion is a meal eaten lately at the meal of the day being logged,
// as a card: its name, its kcal and items, and Repeat, which logs it
// again.
function Suggestion({ meal, onRepeat }: { meal: RecentMeal; onRepeat: (meal: Meal) => void }) {
  const t = useMessages()
  const f = useFormat()
  const name = mealName(meal, t)

  return (
    <li className="flex min-w-0 flex-col gap-2 rounded-lg border bg-card p-3">
      <span className="line-clamp-2 font-semibold wrap-break-word">{name}</span>
      <span className="text-caption text-muted-foreground">
        {[t.unit.kcal(f.number(meal.total.kcal)), t.meal.items(meal.items.length)].join(' · ')}
      </span>
      <Button
        variant="outline"
        size="sm"
        className="mt-auto w-full text-primary"
        aria-label={t.log.repeatOf(name)}
        onClick={() => {
          onRepeat(meal)
        }}
      >
        <Repeat2 aria-hidden />
        {t.log.repeat}
      </Button>
    </li>
  )
}

// Results are a list's foods and meals, or what there is to say when it
// has none, unless it is quiet: what there is was said already.
function Results({
  foods,
  meals,
  pending,
  query,
  timeZone,
  quiet,
  picked,
  pickedRow,
  onPick,
  onAdd,
  onRepeat,
  ingredient,
  pinned,
  catalog,
  chosen,
}: {
  foods: Food[]
  meals: Meal[]
  pending: boolean
  // What was searched for, trimmed.
  query: string
  timeZone: string
  quiet: boolean
  picked: string | undefined
  pickedRow: (food: Food) => React.ReactNode
  onPick: (food: Food) => void
  onAdd: (food: Food) => void
  onRepeat: (meal: Meal) => void
  // Of a recipe's ingredients, which are foods alone.
  ingredient: boolean
  // The food the finder opened with, first of all while it is picked.
  pinned?: string | undefined
  // The ids of the foods of the catalog among foods.
  catalog: Set<string>
  // The ids of the foods in the meal already.
  chosen: ReadonlySet<string> | undefined
}) {
  const t = useMessages()
  const searched = query !== ''
  if (foods.length === 0 && meals.length === 0) {
    if (quiet && !pending) {
      return null
    }
    return (
      <p className="py-2 text-sm text-muted-foreground">
        {pending
          ? t.loading
          : ingredient
            ? searched
              ? t.recipes.nothing
              : t.recipes.noneEaten
            : searched
              ? t.log.nothing
              : t.log.none}
      </p>
    )
  }
  const foodRows = foods.map((food) => (
    <FoodResult
      key={food.id}
      food={food}
      query={query}
      catalog={catalog.has(food.id)}
      chosen={chosen?.has(food.id)}
      open={picked === food.id}
      amount={pickedRow(food)}
      onPick={() => {
        onPick(food)
      }}
      onAdd={() => {
        onAdd(food)
      }}
    />
  ))
  return (
    <ul aria-label={t.log.results}>
      {/* The food it opened with first; the meals a search finds after
          the foods. */}
      {[
        ...foodRows.filter((r) => r.key === pinned),
        ...foodRows.filter((r) => r.key !== pinned),
        ...meals.map((meal) => (
          <MealResult
            key={meal.id}
            meal={meal}
            query={query}
            timeZone={timeZone}
            onRepeat={onRepeat}
          />
        )),
      ]}
    </ul>
  )
}

// nameOf is a food's name, with its brand, which tells two of one name
// apart.
function nameOf(food: Food): string {
  return food.brand ? `${food.name} · ${food.brand}` : food.name
}

// Marked is a text with what the search for query matched in it marked,
// as the start of a word.
function Marked({ text, query }: { text: string; query: string }) {
  return parts(query, text).map((p) =>
    p.matched ? (
      <mark key={p.at} className="rounded-xs bg-highlight text-inherit">
        {p.text}
      </mark>
    ) : (
      <Fragment key={p.at}>{p.text}</Fragment>
    ),
  )
}

// FoodResult is a food found, what the search matched in its name
// marked, marked a recipe, an estimate or the catalog's when it is, and
// what its + adds: the amount it was eaten in last, or its default, its
// kcal and protein (≈ the grams of a recipe not weighed). A tap opens it
// for another amount (amount); its + adds it, checked while it is in
// the meal (chosen), when a tap takes it out again.
function FoodResult({
  food,
  query,
  catalog,
  chosen,
  open,
  amount,
  onPick,
  onAdd,
}: {
  food: Food
  query: string
  catalog: boolean
  // Undefined where a food is added and never taken out again here.
  chosen: boolean | undefined
  open: boolean
  amount: React.ReactNode
  onPick: () => void
  onAdd: () => void
}) {
  const t = useMessages()
  const f = useFormat()
  const id = useId()
  // What its + adds.
  const adds = foodEntry(food, f.field)
  const values = entryValues(adds, f.parse)
  const about = isRecipe(food) && food.recipe?.yield_g === undefined && adds.unit === 'g'

  return (
    <li className={cn('rounded-lg', open && 'bg-surface-2/60')}>
      <div className="flex items-center gap-2">
        <button
          type="button"
          aria-expanded={open}
          aria-controls={`${id}-amount`}
          className="flex min-h-14 min-w-0 flex-1 flex-col justify-center rounded-lg px-2 py-2 text-left outline-none hover:bg-muted focus-visible:ring-2 focus-visible:ring-ring"
          onClick={onPick}
        >
          <span className="flex min-w-0 items-center gap-1.5">
            <span className="truncate font-semibold">
              <Marked text={food.name} query={query} />
              {food.brand && (
                <span className="font-medium text-muted-foreground">
                  {' · '}
                  <Marked text={food.brand} query={query} />
                </span>
              )}
            </span>
            {isRecipe(food) && <Badge tone="primary">{t.recipes.mark}</Badge>}
            {food.source === 'estimate' && <Badge tone="warning">{t.meal.estimate}</Badge>}
            {catalog && <Badge>{t.log.catalog}</Badge>}
          </span>
          <span className="block text-small text-muted-foreground">
            {[
              `${about ? '≈ ' : ''}${entryAmountLabel(adds, t, f)}`,
              ...(values === undefined
                ? []
                : [
                    t.unit.kcal(f.number(values.kcal)),
                    `${t.unit.g(f.grams(values.protein))} ${t.day.short.protein}`,
                  ]),
            ].join(' · ')}
          </span>
        </button>
        {!open && (
          <Button
            variant={chosen ? 'default' : 'outline'}
            size="icon"
            data-add={food.id}
            className={cn('shrink-0 rounded-lg', !chosen && 'text-primary')}
            aria-label={t.log.addOf(nameOf(food))}
            aria-pressed={chosen}
            onClick={onAdd}
          >
            {chosen ? <Check aria-hidden /> : <Plus aria-hidden />}
          </Button>
        )}
      </div>
      {open && (
        <div id={`${id}-amount`} className="px-2 pb-3">
          {amount}
        </div>
      )}
    </li>
  )
}

// approx marks a number about so, when it is.
const approx = (on: boolean) => (on ? '≈ ' : '')

// The shares of a recipe its chips give, of the whole dish.
const shares: readonly [string, number][] = [
  ['1/2', 1 / 2],
  ['1/3', 1 / 3],
  ['1/4', 1 / 4],
]

// Amount is a food picked, in an amount to add: typed, in a unit of the
// food's, which works the amount out anew, or one of its servings in a
// tap; with what it brings. A recipe is given in a portion (when
// divided), a share of the whole dish or grams, its chips a portion, ½,
// ⅓ or ¼ of it, and 1/…, which types 1/ for the denominator to follow; a
// share is typed as a fraction (1/10) or a number. Grams of a recipe not
// weighed are about so (≈), its portions and shares never. field is the
// amount's field, which takes the focus as it comes.
export function Amount({
  entry,
  field: given,
  wrong,
  onChange,
  onAdd,
  onLog,
}: {
  entry: FoodEntry
  field?: RefObject<HTMLInputElement | null> | undefined
  wrong: Wrong | undefined
  onChange: (entry: FoodEntry) => void
  onAdd: (entry: FoodEntry) => void
  // ⌘↵ or Ctrl+↵: the meal logged with it.
  onLog?: ((entry: FoodEntry) => void) | undefined
}) {
  const t = useMessages()
  const f = useFormat()
  const id = useId()
  const own = useRef<HTMLInputElement>(null)
  const field = given ?? own
  useEffect(() => {
    field.current?.focus()
  }, [field])
  const food = entry.food
  const name = nameOf(food)
  const values = entryValues(entry, f.parse)
  const n = entryAmount(entry, f.parse)
  const recipe = isRecipe(food)
  // A recipe not weighed: its grams stand for the raw ingredients'.
  const unweighed = recipe && food.recipe?.yield_g === undefined
  const portionUnit = `serving:${portionServing}`
  const shareUnit = `serving:${wholeServing}`
  const portionGrams = recipe ? unitGrams(food, portionUnit) : undefined
  // A food by the portion takes no serving; a recipe's are its chips.
  const servings = food.basis === 'portion' || recipe ? [] : (food.servings ?? []).slice(0, 3)
  const unitOfServing = (grams: number) =>
    food.basis === '100ml' ? t.unit.ml(f.number(grams)) : t.unit.g(f.number(grams))
  // A unit and an amount in a tap: what a unit chosen worked out is no
  // more.
  const set = (unit: string, amount: string) => {
    const { exact: _was, ...rest } = entry
    onChange({ ...rest, unit, amount })
  }
  const pressed = (unit: string, of: number) =>
    entry.unit === unit && n !== undefined && Math.abs(n - of) < 1e-9
  // The grams a recipe's portion or share weighs, said after what it
  // brings.
  const grams =
    recipe && entry.unit !== 'g' && n !== undefined ? unitGrams(food, entry.unit) : undefined
  // A share waiting for its denominator: 1/.
  const waiting = isShare(food, entry.unit) && /\/\s*$/.test(entry.amount)

  return (
    <div data-amount="" className="space-y-2">
      <div className="flex flex-wrap items-center gap-2">
        <input
          ref={field}
          aria-label={t.log.amount(name)}
          aria-invalid={wrong !== undefined}
          aria-describedby={wrong ? `${id}-wrong` : undefined}
          value={entry.amount}
          // A recipe's share is a fraction, whose / a keyboard of digits lacks.
          inputMode={recipe ? 'text' : 'decimal'}
          autoComplete="off"
          className={cn(inputClass, 'w-24 tabular-nums')}
          onChange={(e) => {
            // Typed: what a unit chosen worked out is no more.
            const { exact: _typed, ...rest } = entry
            onChange({ ...rest, amount: e.currentTarget.value })
          }}
          onKeyDown={(e) => {
            if (e.key !== 'Enter') {
              return
            }
            e.preventDefault()
            if ((e.metaKey || e.ctrlKey) && onLog !== undefined) {
              onLog(entry)
            } else {
              onAdd(entry)
            }
          }}
        />
        <select
          aria-label={t.log.unit(name)}
          value={entry.unit}
          className={cn(inputClass, 'w-auto max-w-36')}
          onChange={(e) => {
            // The amount is worked out anew in the unit chosen, so the item
            // weighs what it did.
            onChange(inUnit(entry, e.currentTarget.value, f.parse, f.field))
          }}
        >
          {unitsOf(food).map((u) => (
            <option key={u} value={u}>
              {entryUnitLabel(food, u, n ?? Number.NaN, t)}
            </option>
          ))}
        </select>
        <Button
          size="icon"
          className="ml-auto"
          aria-label={t.log.addOf(name)}
          onClick={() => {
            onAdd(entry)
          }}
        >
          <Check aria-hidden />
        </Button>
      </div>
      {recipe && (
        <fieldset aria-label={t.recipes.quick} className="flex flex-wrap gap-1.5">
          {portionGrams !== undefined && (
            <Button
              variant="outline"
              size="sm"
              aria-pressed={pressed(portionUnit, 1)}
              onClick={() => {
                set(portionUnit, f.field(1))
              }}
            >
              {t.log.serving(
                t.recipes.onePortion,
                `${approx(unweighed)}${t.unit.g(f.number(portionGrams))}`,
              )}
            </Button>
          )}
          {shares.map(([typed, of]) => (
            <Button
              key={typed}
              variant="outline"
              size="sm"
              className="min-w-11"
              aria-label={t.recipes.ofRecipe(shareOf(of, f.number))}
              aria-pressed={pressed(shareUnit, of)}
              onClick={() => {
                set(shareUnit, typed)
              }}
            >
              {shareOf(of, f.number)}
            </Button>
          ))}
          <Button
            variant="outline"
            size="sm"
            aria-label={t.recipes.denominator}
            onClick={() => {
              flushSync(() => {
                set(shareUnit, '1/')
              })
              const input = field.current
              input?.focus()
              input?.setSelectionRange(2, 2)
            }}
          >
            1/…
          </Button>
        </fieldset>
      )}
      {servings.length > 0 && (
        <div className="flex flex-wrap gap-1.5">
          {servings.map((s) => (
            <Button
              key={s.name}
              variant="outline"
              size="sm"
              onClick={() => {
                set(`serving:${s.name}`, f.field(1))
              }}
            >
              {t.log.serving(s.name, unitOfServing(s.amount))}
            </Button>
          ))}
        </div>
      )}
      <p aria-live="polite" className="text-caption text-muted-foreground">
        {values !== undefined
          ? [
              `= ${approx(unweighed && entry.unit === 'g')}${t.unit.kcal(f.number(values.kcal))}`,
              `${t.day.short.protein} ${f.grams(values.protein)}`,
              `${t.day.short.carbs} ${f.grams(values.carbs)}`,
              `${t.day.short.fat} ${f.grams(values.fat)}`,
              ...(grams === undefined || n === undefined
                ? []
                : [`${approx(unweighed)}${t.unit.g(f.number(n * grams))}`]),
            ].join(' · ')
          : waiting && t.recipes.denominatorHint}
      </p>
      {unweighed && entry.unit === 'g' && (
        <p className="text-caption text-muted-foreground">≈ {t.recipes.gramsApprox}</p>
      )}
      {wrong && (
        <p id={`${id}-wrong`} role="alert" className="text-sm text-destructive">
          {sayWrong(wrong, t, f.number)}
        </p>
      )}
    </div>
  )
}

// MealResult is a meal of before, which a tap logs again: its name, when
// it was eaten, how many items and its kcal, what the search matched in
// its name marked.
function MealResult({
  meal,
  query,
  timeZone,
  onRepeat,
}: {
  meal: Meal
  query: string
  timeZone: string
  onRepeat: (meal: Meal) => void
}) {
  const t = useMessages()
  const f = useFormat()
  const name = mealName(meal, t)

  return (
    <li className="flex min-h-14 items-center gap-2 px-2 py-2">
      <span className="min-w-0 flex-1">
        <span className="block truncate font-semibold">
          <Marked text={name} query={query} />
        </span>
        {longName(meal.description) && (
          <span className="block truncate text-small text-muted-foreground">
            <Marked text={meal.description} query={query} />
          </span>
        )}
        <span className="block text-small text-muted-foreground">
          {t.log.mealLine(
            `${f.day(meal.eaten_at, timeZone)} ${f.time(meal.eaten_at, timeZone)}`,
            t.meal.items(meal.items.length),
            t.unit.kcal(f.number(meal.total.kcal)),
          )}
        </span>
      </span>
      <Button
        variant="outline"
        size="sm"
        className="shrink-0 text-primary"
        aria-label={t.log.repeatOf(name)}
        onClick={() => {
          onRepeat(meal)
        }}
      >
        <Repeat2 aria-hidden />
        {t.log.repeat}
      </Button>
    </li>
  )
}
