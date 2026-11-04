import { mealColumns } from '@/components/day/meal-columns'
import { Skeleton } from '@/components/progress/skeleton'
import { useFormat } from '@/i18n/format'
import { useMessages } from '@/i18n/messages'
import { cn } from '@/lib/utils'

// The seven days of a week, and the three meals the skeleton has.
const week = [0, 1, 2, 3, 4, 5, 6]
const meals = [0, 1, 2]

// DaySkeleton stands in for a day under its header while what it shows
// comes (DayPage), in its shape: on a phone the hero, its ring, numbers
// and macros, the week under it, and three meals' rows; from a tablet's
// 768 px, the hero and the week across, the body and the check-in beside
// the meals; on a desktop (columns)
// the hero and the meals' table, and beside them the week, the body and
// the check-in. A screen reader hears that it loads.
export function DaySkeleton({ columns }: { columns: boolean }) {
  const t = useMessages()
  const weekCard = (
    <div className="space-y-3 rounded-lg border bg-card p-4">
      <Skeleton className="h-5.5 w-28" />
      <WeekDays />
      <Skeleton className="h-11" />
    </div>
  )
  const body = (
    <div className="space-y-3 rounded-lg border bg-card p-4">
      <Skeleton className="h-5.5 w-28" />
      <div className="grid grid-cols-2 gap-2">
        <Skeleton className="h-18" />
        <Skeleton className="h-18" />
      </div>
    </div>
  )
  const checkIn = (
    <div className="space-y-3 rounded-lg border bg-card p-4">
      <Skeleton className="h-5.5 w-24" />
      <Skeleton className="h-11" />
    </div>
  )

  return (
    <div aria-busy className="space-y-4">
      <output className="sr-only">{t.loading}</output>
      <div className="grid grid-cols-1 gap-4 md:grid-cols-2 md:items-start lg:grid-cols-12">
        {!columns && (
          <div className="space-y-4 md:col-span-2">
            <Hero />
            {weekCard}
          </div>
        )}
        <div className="space-y-4 lg:col-span-7 xl:col-span-8">
          {columns && <Hero />}
          <div className="space-y-2">
            <Skeleton className="h-6.5 w-24" />
            {columns ? <MealTable /> : <MealCards />}
          </div>
        </div>
        {columns ? (
          <div className="space-y-4 lg:col-span-5 xl:col-span-4">
            {weekCard}
            {body}
            {checkIn}
          </div>
        ) : (
          // A phone's come after the meals, the day's skeleton enough.
          <div className="hidden space-y-4 md:block">
            {body}
            {checkIn}
          </div>
        )}
      </div>
    </div>
  )
}

// WeekDays are the week's days: each its column, its weekday and its
// number.
function WeekDays() {
  return (
    <div className="grid grid-cols-7 gap-1.5 lg:gap-2">
      {week.map((d) => (
        <div key={d} className="flex flex-col items-center gap-1.5 px-0.5 pt-1 pb-0.5">
          <Skeleton className="h-16 w-full rounded-md lg:h-24" />
          <Skeleton className="h-4 w-6" />
          <Skeleton className="h-4 w-4" />
        </div>
      ))}
    </div>
  )
}

// Hero is the day's hero: the ring, the numbers, the arithmetic, the
// day's type and the phase beside it, and the macros' bars, beside the
// ring too on a desktop.
function Hero() {
  return (
    <div className="grid grid-cols-[auto_minmax(0,1fr)] items-center gap-4 rounded-lg border bg-card p-4 lg:gap-x-7 lg:p-5">
      <Skeleton className="size-33 shrink-0 rounded-full lg:row-span-2 lg:size-42" />
      <div className="space-y-2">
        <div className="flex gap-5">
          <Skeleton className="h-10 w-14" />
          <Skeleton className="h-10 w-14" />
        </div>
        <Skeleton className="h-8 w-full max-w-60" />
        <Skeleton className="h-8 w-24" />
        <Skeleton className="h-4 w-28" />
      </div>
      <div className="col-span-2 space-y-3 lg:col-span-1 lg:col-start-2 lg:grid lg:grid-cols-3 lg:gap-6 lg:space-y-0">
        {[0, 1, 2].map((i) => (
          <div key={i} className="space-y-1.5">
            <Skeleton className="h-4.5 w-28" />
            <Skeleton className="h-2 rounded-full" />
          </div>
        ))}
      </div>
    </div>
  )
}

// MealCards are a phone's meals: rows of one card, the time, the name
// and the kcal, the macros under them.
function MealCards() {
  const f = useFormat()

  return (
    <div className="divide-y rounded-lg border bg-card">
      {meals.map((m) => (
        <div
          key={m}
          className={cn(
            'grid items-center gap-x-2.5 gap-y-1.5 px-4 py-3.5',
            f.hour12
              ? 'grid-cols-[3.75rem_minmax(0,1fr)_auto]'
              : 'grid-cols-[2.5rem_minmax(0,1fr)_auto]',
          )}
        >
          <Skeleton className="h-4 w-full" />
          <Skeleton className="h-5.5 w-3/5" />
          <Skeleton className="h-5.5 w-16" />
          <Skeleton className="col-span-2 col-start-2 h-4 w-4/5" />
        </div>
      ))}
    </div>
  )
}

// MealTable is a desktop's meals, a table: its heads, a row a meal, and
// the day's total.
function MealTable() {
  const f = useFormat()
  const grid = cn('grid items-center', mealColumns(f.hour12))

  return (
    <div className="rounded-lg border bg-card">
      <div className={cn(grid, 'border-b py-2')}>
        <Skeleton className="col-span-2 h-4 w-20" />
      </div>
      {meals.map((m) => (
        <div key={m} className={cn(grid, 'min-h-14 border-b py-3')}>
          <Skeleton className="h-4 w-full" />
          <Skeleton className="h-5.5 w-3/5" />
          <Skeleton className="h-5.5 w-full" />
          <Skeleton className="h-4.5 w-full" />
          <Skeleton className="h-4.5 w-full" />
          <Skeleton className="h-4.5 w-full" />
        </div>
      ))}
      <div className={cn(grid, 'py-3')}>
        <Skeleton className="col-span-2 h-5.5 w-16" />
        <Skeleton className="h-5.5 w-full" />
      </div>
    </div>
  )
}
