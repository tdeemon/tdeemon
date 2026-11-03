import { Link } from '@tanstack/react-router'
import { ChevronRight } from 'lucide-react'

import type { Food } from '@/api/types'
import { Badge } from '@/components/ui/badge'
import { useFormat } from '@/i18n/format'
import { useMessages } from '@/i18n/messages'
import type { FoodsSearch } from '@/router'

// FoodRow is a food in a list: its name and brand, a mark only where it
// says something (a recipe, an estimate), and its energy and protein for
// its basis (120 kcal · 23 g P /100 g); a link to the food, marked while
// it is the one open beside the list. A recipe divided in portions gives
// a portion's. Where its energy comes from is its page's.
export function FoodRow({ food, from }: { food: Food; from: FoodsSearch }) {
  const t = useMessages()
  const f = useFormat()
  const portion = food.recipe?.portion
  const n = portion ?? food.nutrients
  const per = portion ? t.recipes.perPortionShort : t.foods.perShort[food.basis]
  const perSaid = portion ? t.recipes.perPortion : t.foods.per[food.basis]
  const kcal = f.number(n.kcal)
  const protein = f.grams(n.protein)

  return (
    <li>
      <Link
        to="/foods/$id"
        params={{ id: food.id }}
        search={from}
        // Open whatever the list beside it, and read or edited.
        activeOptions={{ includeSearch: false }}
        className="flex min-h-14 items-center gap-3 px-4 py-2 outline-none hover:bg-muted focus-visible:ring-2 focus-visible:ring-ring focus-visible:ring-inset data-[status=active]:bg-primary-soft"
      >
        <span className="min-w-0 flex-1">
          <span className="flex min-w-0 items-center gap-1.5">
            <span className="truncate font-semibold">
              {food.name}
              {food.brand && (
                <span className="font-medium text-muted-foreground"> · {food.brand}</span>
              )}
            </span>
            {food.source === 'recipe' && <Badge tone="primary">{t.recipes.mark}</Badge>}
            {food.source === 'estimate' && <Badge tone="warning">{t.meal.estimate}</Badge>}
          </span>
          <span className="block truncate text-small text-muted-foreground">
            <span aria-hidden>
              {t.foods.density(kcal, protein)} {per}
            </span>
            <span className="sr-only">{t.foods.densitySaid(kcal, protein, perSaid)}</span>
          </span>
        </span>
        <ChevronRight aria-hidden className="size-4 shrink-0 text-muted-foreground" />
      </Link>
    </li>
  )
}
