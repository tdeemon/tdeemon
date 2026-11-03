import { Menu } from '@base-ui/react/menu'
import { Link, useLocation, useNavigate, useSearch } from '@tanstack/react-router'
import { Check, ChevronDown, CookingPot, Plus, Search } from 'lucide-react'
import { useEffect, useId, useRef, useState, type ReactNode } from 'react'

import { $api } from '@/api/client'
import { useFindFoods, useFindOfList, useFoodList, type FoodFilter } from '@/api/foods'
import { eatenThisWeek } from '@/components/foods/eaten'
import { FoodRow } from '@/components/foods/food-row'
import { LoadError } from '@/components/load-error'
import { SettingsAvatar } from '@/components/settings-avatar'
import { Button, buttonVariants } from '@/components/ui/button'
import { ChipGroup } from '@/components/ui/chip-group'
import { useFormat } from '@/i18n/format'
import { useMessages } from '@/i18n/messages'
import { cn } from '@/lib/utils'
import type { FoodsSearch } from '@/router'

const filters = ['inUse', 'recipes', 'estimates', 'archived'] as const

// The orders of the list, the first when the address names none.
const orders = ['frequent', 'use', 'name'] as const
type Order = (typeof orders)[number]

// A list's card, which its rows divide.
const card = 'divide-y overflow-hidden rounded-lg bg-card shadow-card'

// FoodBrowser is the user's foods: a search of them, or a list (those in
// use, the recipes, the estimates in use, the archived ones, as chips,
// the one chosen with how many foods it has), by how many of the latest
// meals had them (those eaten this week first), by when they were eaten
// last or by name (a menu beside the chips), a page at a time; and a way
// to add a food or a recipe, each a button of its own (on a phone the
// list's own: a recipe in the recipes, else a food). A search is of the
// list chosen. What it shows is in the address (q, filter, order), which
// a food opened from it keeps, so the list beside a food, and back from
// it, is as it was. With detail (a desktop's), the list and the detail
// are two panes under the title, each scrolled on its own, the page not.
export function FoodBrowser({ detail }: { detail?: ReactNode }) {
  const t = useMessages()
  const f = useFormat()
  const id = useId()
  const search = useSearch({ from: '/app/foods' })
  const { q = '', filter = 'inUse', order = 'frequent' } = search
  // The address's own path stays: a food open beside the list stays open.
  const navigate = useNavigate()
  // What is typed: the address follows it, but the field is not taken
  // from it as it is typed, which would move the caret to the end. A q
  // the field did not write (a link, back and forward) is put in it; one
  // it wrote comes later than the typing, and is passed over, with those
  // written before it, which it replaced (some never come: a navigation
  // that waits on the network gives way to the next).
  const [typed, setTyped] = useState(q)
  const [written, setWritten] = useState<readonly string[]>([])
  const [seenQ, setSeenQ] = useState(q)
  if (q !== seenQ) {
    setSeenQ(q)
    const at = written.indexOf(q)
    if (at >= 0) {
      setWritten(written.slice(at + 1))
    } else {
      setTyped(q)
    }
  }
  const searching = typed.trim() !== ''
  // The search takes the focus when the shortcut / brought the page.
  const field = useRef<HTMLInputElement>(null)
  const find = useLocation({ select: (l) => l.state.find === true })
  useEffect(() => {
    if (find) {
      field.current?.focus()
    }
  }, [find])
  // The list as the address has it, which a food opened from it keeps.
  const from: FoodsSearch = {
    q: search.q,
    filter: search.filter,
    order: search.order,
  }
  // How many foods the list chosen has, once it came (the list's own
  // request), a search of it too.
  const total = useFoodList(filter, order).data?.pages.at(-1)?.total

  // The search and the list chosen.
  const list = (
    <>
      <label className="flex h-12 items-center gap-2.5 rounded-lg border border-input bg-card px-3.5 focus-within:border-ring focus-within:ring-1 focus-within:ring-ring">
        <Search aria-hidden className="size-4.5 shrink-0 text-muted-foreground" />
        <span className="sr-only">{t.foods.search}</span>
        <input
          ref={field}
          type="search"
          // The shortcut / finds it.
          data-find
          value={typed}
          placeholder={filter === 'recipes' ? t.recipes.searchHint : t.foods.searchHint}
          autoComplete="off"
          className="h-full min-w-0 flex-1 bg-transparent text-base outline-none"
          onChange={(event) => {
            const now = event.currentTarget.value
            setTyped(now)
            setWritten((w) => [...w, now])
            void navigate({
              to: '.',
              search: (s: FoodsSearch) => ({ ...s, q: now === '' ? undefined : now }),
              replace: true,
            })
          }}
        />
      </label>
      {/* The search too is of the list chosen, and ordered by how well a
          food matches. Rows of chips apart, a chip's target clear of the
          next row's. */}
      <div className="flex flex-wrap items-center gap-x-2 gap-y-3">
        <ChipGroup
          legend={t.foods.lists}
          options={filters.map((value) => {
            const name = listName(value, t)
            return {
              value,
              label:
                value === filter && total !== undefined
                  ? t.foods.counted(name, f.number(total))
                  : name,
            }
          })}
          value={filter}
          onChange={(chosen) => {
            void navigate({
              to: '.',
              search: (s: FoodsSearch) => ({
                ...s,
                filter: chosen === 'inUse' ? undefined : chosen,
              }),
            })
          }}
        />
        {!searching && (
          <OrderMenu
            order={order}
            onChange={(chosen) => {
              void navigate({
                to: '.',
                search: (s: FoodsSearch) => ({
                  ...s,
                  order: chosen === 'frequent' ? undefined : chosen,
                }),
              })
            }}
          />
        )}
      </div>
      {searching ? (
        <Found query={typed} filter={filter} from={from} />
      ) : (
        <Listed filter={filter} order={order} from={from} />
      )}
    </>
  )

  return (
    <section
      aria-labelledby={`${id}-title`}
      className={
        detail === undefined
          ? 'min-w-0 space-y-3'
          : // The page's height, but the main's top padding: its bottom's
            // is the panes' own.
            '-mb-12 flex h-[calc(100dvh-2rem)] min-w-0 flex-col'
      }
    >
      <header
        className={cn(
          'flex items-center justify-between gap-2',
          detail !== undefined && 'border-b pb-4',
        )}
      >
        <h1 id={`${id}-title`} className="mr-auto text-title">
          {t.foods.title}
        </h1>
        <NewButtons filter={filter} total={total} from={from} />
        <SettingsAvatar className="-my-1.5 -ml-1" />
      </header>
      {detail === undefined ? (
        list
      ) : (
        <div className="grid min-h-0 flex-1 grid-cols-[minmax(0,25rem)_minmax(0,1fr)]">
          {/* Each pane its own scroll, which holds what is placed in it (a
              screen reader's words); its focus rings clear of its edge. */}
          <div className="relative min-h-0 space-y-3 overflow-y-auto border-r py-4 pr-5 pl-1">
            {list}
          </div>
          {/* The detail's title stays at its top: a control the focus
              comes to scrolls clear of it, a title of two lines too. */}
          <div className="@container relative min-h-0 scroll-pt-36 overflow-y-auto pt-4 pb-6 pl-8">
            {detail}
          </div>
        </div>
      )}
    </section>
  )
}

// NewButtons are the ways to add a food and a recipe, each a button of
// its own; the list's own first, the other beside it from a tablet up (a
// phone has it in its list). A list without foods has its own button,
// the one way there to add one.
function NewButtons({
  filter,
  total,
  from,
}: {
  filter: FoodFilter
  total: number | undefined
  from: FoodsSearch
}) {
  const t = useMessages()
  const recipes = filter === 'recipes'
  const food = (
    <Link
      to="/foods/new"
      search={from}
      className={buttonVariants({
        size: 'sm',
        variant: recipes ? 'outline' : 'default',
        className: recipes ? 'max-sm:hidden' : undefined,
      })}
    >
      <Plus aria-hidden />
      {t.foods.new}
    </Link>
  )
  const recipe = (
    <Link
      to="/foods/new-recipe"
      search={from}
      className={buttonVariants({
        size: 'sm',
        variant: recipes ? 'default' : 'outline',
        className: recipes ? undefined : 'max-sm:hidden',
      })}
    >
      <Plus aria-hidden />
      {t.recipes.new}
    </Link>
  )
  const empty = total === 0 && (recipes || filter === 'inUse')
  // The list's own last, nearest the hand.
  return (
    <>
      {recipes ? food : recipe}
      {!empty && (recipes ? recipe : food)}
    </>
  )
}

// OrderMenu is the order of a list, as a menu whose button says the one
// chosen, in short.
function OrderMenu({ order, onChange }: { order: Order; onChange: (order: Order) => void }) {
  const t = useMessages()

  return (
    <Menu.Root>
      <Menu.Trigger
        aria-label={t.foods.order(t.foods.orderShort[order])}
        className="relative ml-auto flex h-8 shrink-0 items-center gap-0.5 rounded-md px-1.5 text-small font-semibold whitespace-nowrap text-primary outline-none after:absolute after:inset-x-0 after:-inset-y-1.5 after:content-[''] hover:bg-muted focus-visible:ring-2 focus-visible:ring-ring"
      >
        {t.foods.orderShort[order]}
        <ChevronDown aria-hidden className="size-3.5" />
      </Menu.Trigger>
      <Menu.Portal>
        <Menu.Positioner sideOffset={6} align="end" className="z-40">
          <Menu.Popup className="min-w-48 rounded-lg border bg-card p-1 shadow-sheet outline-none">
            <Menu.RadioGroup
              value={order}
              onValueChange={(value: Order) => {
                if (value !== order) {
                  onChange(value)
                }
              }}
            >
              {orders.map((o) => (
                <Menu.RadioItem
                  key={o}
                  value={o}
                  closeOnClick
                  className="flex h-11 cursor-default items-center gap-2 rounded-md pr-3 pl-2 text-sm outline-none select-none data-highlighted:bg-muted"
                >
                  <span className="flex size-4 items-center justify-center">
                    <Menu.RadioItemIndicator>
                      <Check aria-hidden className="size-4" />
                    </Menu.RadioItemIndicator>
                  </span>
                  {t.foods.orders[o]}
                </Menu.RadioItem>
              ))}
            </Menu.RadioGroup>
          </Menu.Popup>
        </Menu.Positioner>
      </Menu.Portal>
    </Menu.Root>
  )
}

// listName is a list's name, as its chip says it.
function listName(filter: FoodFilter, t: ReturnType<typeof useMessages>): string {
  return filter === 'recipes' ? t.recipes.list : t.foods.filter[filter]
}

// Found is what a search of a list finds: of the archived foods, or of
// those in use, all, their estimates or their recipes.
function Found({ query, filter, from }: { query: string; filter: FoodFilter; from: FoodsSearch }) {
  const t = useMessages()
  const own = filter === 'estimates' || filter === 'recipes' ? filter : undefined
  // The estimates and the recipes are searched in their own list: a
  // search of all the foods finds 50 at most, which may leave them out.
  const all = useFindFoods(own === undefined ? query : '', filter === 'archived')
  const listed = useFindOfList(own ?? 'estimates', own === undefined ? '' : query)
  const found = own === undefined ? all : listed

  if (found.isPending) {
    return <Skeleton />
  }
  // A search that came stays while asking for it again fails.
  if (found.data === undefined) {
    return (
      <LoadError
        title={t.loadFailed.foods}
        error={found.error}
        onRetry={() => void found.refetch()}
      />
    )
  }
  const foods = own === undefined ? (all.data?.foods ?? []) : (listed.data ?? [])
  if (foods.length === 0) {
    return filter === 'recipes' ? (
      <Empty words={t.recipes.noMatch} from={from} recipe />
    ) : (
      <Empty words={t.foods.noMatch} from={from} />
    )
  }
  return (
    <ul aria-label={t.foods.found} className={card}>
      {foods.map((food) => (
        <FoodRow key={food.id} food={food} from={from} />
      ))}
    </ul>
  )
}

// Listed is a list of the foods, with more to show while there are. By
// how many meals had them, those eaten in the last 7 days come first, as
// this week's.
function Listed({ filter, order, from }: { filter: FoodFilter; order: Order; from: FoodsSearch }) {
  const t = useMessages()
  const list = useFoodList(filter, order)
  const today = $api.useQuery('get', '/api/v1/me').data?.today

  if (list.isPending) {
    return <Skeleton />
  }
  if (list.isLoadingError) {
    return (
      <LoadError
        title={t.loadFailed.foods}
        error={list.error}
        onRetry={() => void list.refetch()}
      />
    )
  }
  // A food may come on two pages, when the list changed between them.
  const seen = new Set<string>()
  const foods = list.data.pages
    .flatMap((p) => p.foods)
    .filter((food) => !seen.has(food.id) && Boolean(seen.add(food.id)))
  const total = list.data.pages.at(-1)?.total ?? foods.length
  if (foods.length === 0) {
    if (filter === 'recipes') {
      return <NoRecipes from={from} />
    }
    return (
      <Empty
        words={
          { inUse: t.foods.none, archived: t.foods.noneArchived, estimates: t.foods.noneEstimates }[
            filter
          ]
        }
        from={from}
        add={filter === 'inUse'}
      />
    )
  }
  const name = listName(filter, t)
  const groups =
    order === 'frequent' && today !== undefined
      ? [
          { title: t.foods.thisWeek, foods: foods.filter((food) => eatenThisWeek(food, today)) },
          { title: t.foods.earlier, foods: foods.filter((food) => !eatenThisWeek(food, today)) },
        ].filter((g) => g.foods.length > 0)
      : [{ title: undefined, foods }]
  return (
    <div className="space-y-3">
      {groups.map((group) => (
        <section key={group.title ?? 'all'} className="space-y-1.5">
          {group.title !== undefined && (
            <h2 className="px-1 text-small font-semibold text-muted-foreground">{group.title}</h2>
          )}
          <ul
            aria-label={group.title === undefined ? name : `${name}: ${group.title}`}
            className={card}
          >
            {group.foods.map((food) => (
              <FoodRow key={food.id} food={food} from={from} />
            ))}
          </ul>
        </section>
      ))}
      <p className="text-center text-small text-muted-foreground">
        {t.foods.shown(foods.length, total)}
      </p>
      {list.hasNextPage && (
        <Button
          variant="outline"
          className="w-full"
          disabled={list.isFetchingNextPage}
          onClick={() => void list.fetchNextPage()}
        >
          {list.isFetchingNextPage ? t.loading : t.foods.more}
        </Button>
      )}
      {list.isFetchNextPageError && (
        <LoadError error={list.error} onRetry={() => void list.fetchNextPage()} />
      )}
    </div>
  )
}

// Empty is what a list or a search without foods says, with the way to
// add one, or a recipe.
function Empty({
  words,
  from,
  add = true,
  recipe = false,
}: {
  words: string
  from: FoodsSearch
  add?: boolean
  recipe?: boolean
}) {
  const t = useMessages()

  return (
    <div className="space-y-3 rounded-lg bg-card p-4 text-center shadow-card lg:p-6">
      <p className="text-muted-foreground">{words}</p>
      {add && (
        <Link
          to={recipe ? '/foods/new-recipe' : '/foods/new'}
          search={from}
          className={buttonVariants()}
        >
          <Plus aria-hidden />
          {recipe ? t.recipes.new : t.foods.add}
        </Link>
      )}
    </div>
  )
}

// NoRecipes is the recipes' list before there is one: what a recipe is
// for, and the way to write one.
function NoRecipes({ from }: { from: FoodsSearch }) {
  const t = useMessages()

  return (
    <div className="flex flex-col items-center gap-3 rounded-lg bg-card p-6 text-center shadow-card">
      <span
        aria-hidden
        className="grid size-14 place-items-center rounded-md bg-primary-soft text-primary"
      >
        <CookingPot className="size-6" />
      </span>
      <div className="max-w-sm space-y-1.5">
        <h2 className="text-h3">{t.recipes.none}</h2>
        <p className="text-muted-foreground">{t.recipes.noneWhy}</p>
        <p className="text-small text-muted-foreground">{t.recipes.noneCoach}</p>
      </div>
      <Link to="/foods/new-recipe" search={from} className={buttonVariants()}>
        <Plus aria-hidden />
        {t.recipes.new}
      </Link>
    </div>
  )
}

// Skeleton is a list on its way: rows of a food's shape, so what comes
// takes their place without moving the page.
function Skeleton() {
  const t = useMessages()

  return (
    <div>
      <output className="sr-only">{t.loading}</output>
      <ul aria-hidden className={card}>
        {[0, 1, 2, 3, 4].map((i) => (
          <li key={i} className="flex min-h-14 items-center gap-3 px-4 py-2">
            <span className="flex-1 space-y-2">
              <span className="block h-3.5 w-2/3 animate-pulse rounded-sm bg-surface-2 motion-reduce:animate-none" />
              <span className="block h-2.5 w-1/2 animate-pulse rounded-sm bg-surface-2 motion-reduce:animate-none" />
            </span>
            <span className="h-3.5 w-14 animate-pulse rounded-sm bg-surface-2 motion-reduce:animate-none" />
          </li>
        ))}
      </ul>
    </div>
  )
}
