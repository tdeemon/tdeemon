import { Legend } from '@/components/charts/legend'
import { weekDates } from '@/components/day/week'
import { WeekBars } from '@/components/day/week-bars'
import { WeekLine } from '@/components/day/week-line'
import { useFormat } from '@/i18n/format'
import { useMessages } from '@/i18n/messages'

// WeekCard is the day's week, at every width: its days as columns of what
// each ate, its target a tick across them and today outlined (WeekBars),
// what the marks mean, and the week in a line, which opens its days and
// what the days left can eat (WeekLine). Under the hero on a phone, a
// tablet's too; beside the day on a desktop.
export function WeekCard({ date, today }: { date: string; today: string }) {
  const t = useMessages()
  const f = useFormat()
  const dates = weekDates(date)
  const w = t.day.week

  return (
    <section aria-labelledby="day-week" className="space-y-3 rounded-lg border bg-card p-4">
      <div className="flex items-baseline justify-between gap-2">
        <h2 id="day-week" className="text-h3">
          {dates.includes(today) ? w.thisWeek : w.thatWeek}
        </h2>
        <span className="text-caption text-muted-foreground">
          {w.span(f.short(dates[0] ?? date), f.short(dates[6] ?? date))}
        </span>
      </div>
      <WeekBars date={date} today={today} />
      <Legend
        marks={[
          { name: w.legendTarget, mark: 'line', color: 'bg-foreground/60' },
          ...(dates.includes(today)
            ? [{ name: w.legendToday, mark: 'outline' as const, color: 'border-foreground' }]
            : []),
        ]}
      />
      <WeekLine date={date} today={today} />
    </section>
  )
}
