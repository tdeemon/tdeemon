import { useQuery } from '@tanstack/react-query'
import { House } from 'lucide-react'

import { householdQuery } from '@/api/household'
import { useFormat } from '@/i18n/format'
import { useMessages } from '@/i18n/messages'
import { cn } from '@/lib/utils'

// HouseholdMark is a food's card of the household that shares it: its
// members (Ana and you), and that a change reaches every one of them; of
// a recipe, that each logs their own portion. Nothing out of a household.
export function HouseholdMark({
  recipe = false,
  className,
}: {
  recipe?: boolean
  className?: string
}) {
  const t = useMessages()
  const f = useFormat()
  const household = useQuery(householdQuery)
  const members = household.data?.members ?? []
  if (members.length === 0) {
    return null
  }
  // The others first, the user last: Ana and you.
  const names = [
    ...members.filter((m) => m.you !== true).map((m) => m.name || m.email),
    t.foods.householdYou,
  ]
  return (
    <section className={cn('flex items-start gap-3', className)}>
      <House aria-hidden className="mt-0.5 size-5 shrink-0 text-muted-foreground" />
      <div className="min-w-0 flex-1">
        <h2 className="text-h3 break-words">{t.foods.household(f.list(names))}</h2>
        <p className="text-small text-muted-foreground">
          {t.foods.householdChange}
          {recipe && ` ${t.recipes.householdLog}`}
        </p>
      </div>
    </section>
  )
}
