import { Link } from '@tanstack/react-router'

import { $api } from '@/api/client'
import type { components } from '@/api/schema'
import { kcalStanding } from '@/components/day/left'
import { weekDates, weekOf } from '@/components/day/week'
import { useFormat } from '@/i18n/format'
import { useMessages } from '@/i18n/messages'
import { cn } from '@/lib/utils'

type HistoryDay = components['schemas']['HistoryDayBody']

// isOver tells a day past its target by more than the week takes in its
// stride (kcalStanding).
function isOver(intake: number | undefined, target: number | undefined): boolean {
  return intake !== undefined && target !== undefined && kcalStanding(intake, target) === 'over'
}

// Bar is a day's intake as a column, against the week's most (scale): its
// height what was eaten, so a day before any target shows too, and its
// target a tick across it. Past its target by more than the week takes
// in its stride, the column is the brand's up to the tick and, above it,
// narrower in the over colour, so it shows without the colour too.
// Today's is outlined, and lighter while it is open: it fills as the day
// is logged.
function Bar({
  day,
  scale,
  today,
}: {
  day: HistoryDay | undefined
  scale: number
  today: boolean
}) {
  const intake = day?.intake?.kcal
  const target = day?.target?.kcal
  const over = isOver(intake, target)
  const below = over ? target : intake
  const share = (kcal: number) => `${String((kcal / scale) * 100)}%`

  return (
    <span
      aria-hidden
      className={cn(
        'relative block h-16 w-full rounded-md bg-surface-2 lg:h-24',
        today && 'outline-2 outline-offset-2 outline-foreground',
      )}
    >
      {below !== undefined && below > 0 && (
        <span
          className={cn(
            'absolute inset-x-0 bottom-0 min-h-0.5 rounded-md bg-primary',
            today && 'opacity-60',
          )}
          style={{ height: share(below) }}
        />
      )}
      {over && intake !== undefined && below !== undefined && (
        <span
          className="absolute inset-x-1.5 rounded-t-md bg-over"
          style={{ bottom: share(below), height: share(intake - below) }}
        />
      )}
      {target !== undefined && (
        <span
          data-target
          className="absolute -inset-x-1 h-0.5 translate-y-1/2 rounded-full bg-foreground/60"
          style={{ bottom: share(target) }}
        />
      )}
    </span>
  )
}

// WeekBars is the calendar week of the day shown, Monday to Sunday, as
// the progress's weeks are, at every width: each day a link to its day,
// a column of what it ate with its target a tick across (Bar), today
// outlined, the day shown its weekday marked; a day to come is no link,
// as the arrows stop at today, and has nothing to show. Each says it in
// words, an over target's too.
export function WeekBars({ date, today }: { date: string; today: string }) {
  const t = useMessages()
  const f = useFormat()
  const query = weekOf(date, today)
  const history = $api.useQuery(
    'get',
    '/api/v1/history',
    { params: { query } },
    // A week to come has no history yet.
    { enabled: query.from <= query.to },
  )
  const byDate = new Map(history.data?.days.map((d) => [d.date, d]))
  const dates = weekDates(date)
  const scale = Math.max(
    1,
    ...dates.map((d) =>
      Math.max(byDate.get(d)?.intake?.kcal ?? 0, byDate.get(d)?.target?.kcal ?? 0),
    ),
  )
  const w = t.day.week

  function said(day: HistoryDay | undefined): string {
    const intake = day?.intake?.kcal
    if (intake === undefined) {
      return w.nothing
    }
    const target = day?.target?.kcal
    if (target === undefined) {
      return w.eaten(f.number(intake))
    }
    return (isOver(intake, target) ? w.eatenOver : w.eatenOf)(f.number(intake), f.number(target))
  }

  const cell =
    'group flex flex-col items-center gap-1.5 rounded-md px-0.5 pt-1 pb-0.5 text-muted-foreground'

  return (
    <nav aria-label={w.label}>
      <ol className="grid grid-cols-7 gap-1.5 lg:gap-2">
        {dates.map((d) => {
          const day = byDate.get(d)
          // Named as it reads, its weekday and day, so a voice says what
          // it sees.
          const name = `${f.weekday(d)} ${f.dayOfMonth(d)}`
          const face = (
            <>
              <Bar day={day} scale={scale} today={d === today} />
              {/* A space after the weekday, which a column does not show,
                  and the text reads "Tue 22", as the name. */}
              <span
                aria-hidden
                className="flex flex-col items-center rounded-sm px-1 text-caption leading-tight group-aria-[current]:bg-primary-soft group-aria-[current]:font-semibold group-aria-[current]:text-primary"
              >
                <span>{f.weekday(d)} </span>
                <span className="font-semibold text-foreground group-aria-[current]:text-primary">
                  {f.dayOfMonth(d)}
                </span>
              </span>
            </>
          )
          return (
            <li key={d}>
              {d > today ? (
                // A day to come, the day shown only by an address.
                <span aria-current={d === date ? 'date' : undefined} className={cell}>
                  <span className="sr-only">{w.day(name, w.toCome)}</span>
                  {face}
                </span>
              ) : (
                <Link
                  to="/"
                  // Today's link is / itself.
                  search={d === today ? {} : { date: d }}
                  activeOptions={{ exact: true }}
                  aria-label={w.day(name, said(day))}
                  className={cn(
                    cell,
                    // 44 px wide to a finger at 390 px too: an ::after
                    // over half the gap either side.
                    "relative outline-none after:absolute after:-inset-x-0.75 after:inset-y-0 after:content-[''] hover:bg-muted focus-visible:ring-2 focus-visible:ring-ring",
                  )}
                >
                  {face}
                </Link>
              )}
            </li>
          )
        })}
      </ol>
    </nav>
  )
}
