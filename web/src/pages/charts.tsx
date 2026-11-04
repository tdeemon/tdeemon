import { keepPreviousData } from '@tanstack/react-query'
import { Link, useNavigate, useSearch } from '@tanstack/react-router'
import { ChartLine } from 'lucide-react'
import { useState, type ReactNode } from 'react'

import { $api } from '@/api/client'
import type { components } from '@/api/schema'
import { Legend, type Mark } from '@/components/charts/legend'
import {
  counted,
  eaten7,
  fromFirst,
  goalShown,
  lineOption,
  sleepOption,
  stepsOf,
  stepsOption,
  tdeeOption,
  weightOption,
  type Cited,
  type HistoryDay,
  type Point,
  type Words,
} from '@/components/charts/options'
import { useChartColors, type ChartColors } from '@/components/charts/palette'
import { useChartWords } from '@/components/charts/words'
import { LoadError } from '@/components/load-error'
import { ChartCard } from '@/components/progress/chart-card'
import { citesWeight } from '@/components/progress/cites'
import { CoachLine } from '@/components/progress/coach-line'
import type { DayRows } from '@/components/progress/day-table'
import { MetricTile, type Usual } from '@/components/progress/metric-tile'
import { phaseSaid } from '@/components/progress/said'
import { Skeleton } from '@/components/progress/skeleton'
import {
  byMonth,
  defaultSpan,
  firstDay,
  spans,
  weeksOf,
  type Span,
} from '@/components/progress/span'
import { card, entry } from '@/components/progress/styles'
import { VerdictHero } from '@/components/progress/verdict-hero'
import { WeeklyTable } from '@/components/progress/weekly-table'
import { SettingsAvatar } from '@/components/settings-avatar'
import { buttonVariants } from '@/components/ui/button'
import { InfoTip } from '@/components/ui/info-tip'
import { SegmentedControl } from '@/components/ui/segmented'
import { rounded, useFormat } from '@/i18n/format'
import { useMessages } from '@/i18n/messages'
import limits from '@/lib/limits.json'
import { useDesktop } from '@/lib/media'
import { cn } from '@/lib/utils'

type Phase = components['schemas']['PhaseBody']
type Plan = components['schemas']['PlanBody']

type Days = 30 | 90 | 365 | 'phase' | 'all'

// The address's days of each span; the default's is left out.
const searchOf: Record<Span, Days> = {
  '30': 30,
  phase: 'phase',
  '90': 90,
  '365': 365,
  all: 'all',
}

// spanOf is the span the address's days say, else the default.
function spanOf(days: Days | undefined, fallback: Span): Span {
  return spans.find((s) => searchOf[s] === days) ?? fallback
}

// drawn reports whether a day has anything a chart draws.
function drawn(d: HistoryDay): boolean {
  return [
    d.intake,
    d.weight_kg,
    d.trend_kg,
    d.tdee,
    d.sleep_h,
    d.resting_hr,
    d.hrv,
    d.body_fat_pct,
    d.steps,
  ].some((v) => v !== undefined)
}

// averageOf is the average of the values there are.
function averageOf(values: Point[]): number | undefined {
  const have = values.filter((v): v is number => v !== null)
  return have.length === 0 ? undefined : have.reduce((a, b) => a + b, 0) / have.length
}

// trimmed are a measurement's days and values from its first value on: a
// chart starts at its first data.
function trimmed(days: HistoryDay[], values: Point[]): { days: HistoryDay[]; values: Point[] } {
  const first = values.findIndex((v) => v !== null)
  return first <= 0 ? { days, values } : { days: days.slice(first), values: values.slice(first) }
}

// tableOf are a chart's days as a table's rows, the latest first: each
// day's cells (cells), those of none written as a dash, and no day of
// none at all.
function tableOf(
  days: HistoryDay[],
  w: Words,
  columns: string[],
  cells: (day: HistoryDay, i: number) => (string | undefined)[],
): DayRows {
  return {
    columns,
    rows: days
      .flatMap((d, i) => {
        const of = cells(d, i)
        return of.every((c) => c === undefined)
          ? []
          : [{ key: d.date, date: w.day(d.date), cells: of.map((c) => c ?? '—') }]
      })
      .toReversed(),
  }
}

// ChartsPage is the progress over a period up to today (30 days to a
// year, the phase's, or all), by default the phase's, or all while the
// phase is young: first the TDEE measured and how the phase goes, as
// calibration judges it; then the weight and its trend, and the TDEE
// against what was eaten, each from its first data, and a proposal that
// waits marked on the chart it cites; sleep, the heart, the steps and the
// body fat in small, each a whole chart a tap away; and the weeks in a
// table, which says the charts in text. Each chart is a table of its days
// a tap away too.
export function ChartsPage() {
  const t = useMessages()
  const { days: asked } = useSearch({ from: '/app/progress' })
  const navigate = useNavigate({ from: '/progress' })
  const me = $api.useQuery('get', '/api/v1/me')
  const plan = $api.useQuery('get', '/api/v1/plan')
  const desktop = useDesktop()
  const today = me.data?.today
  const phase = me.data?.phase
  const noPhase = me.data !== undefined && phase === undefined
  const fallback = today === undefined ? 'all' : defaultSpan(today, phase?.start)
  // The phase's span is there while the user is in one.
  const chosen = spanOf(asked, fallback)
  const span = chosen === 'phase' && noPhase ? fallback : chosen
  const from = today === undefined ? undefined : firstDay(span, today, phase?.start)
  const history = $api.useQuery(
    'get',
    '/api/v1/history',
    { params: { query: { from: from ?? '' } } },
    { enabled: from !== undefined, placeholderData: keepPreviousData },
  )
  // The first day of data, where the charts start.
  const first = history.data?.days.find(drawn)?.date ?? from
  const words = useChartWords(first !== undefined && today !== undefined && byMonth(first, today))
  const colors = useChartColors()

  const hero =
    today === undefined ? (
      !me.isError && <HeroSkeleton />
    ) : (
      <VerdictHero phase={phase} today={today} />
    )

  // What came stays while asking for it again fails: the bar over the
  // page says since when.
  let body
  let charts: Charts | undefined
  if (me.data === undefined && me.isError) {
    body = (
      <LoadError title={t.loadFailed.progress} error={me.error} onRetry={() => void me.refetch()} />
    )
  } else if (history.data === undefined && history.isError) {
    body = (
      <LoadError
        title={t.loadFailed.progress}
        error={history.error}
        onRetry={() => void history.refetch()}
      />
    )
  } else if (!history.data || today === undefined || first === undefined) {
    body = <ProgressSkeleton />
  } else if (history.data.days.every((d) => !drawn(d))) {
    body = <Empty />
  } else {
    charts = {
      // From the first day of data on: a chart of none has no more.
      days: fromFirst(history.data.days, drawn),
      words,
      colors,
      phase,
      weeks: weeksOf(first, today),
      today,
      plan: plan.data,
    }
  }

  return (
    <section className="space-y-3 lg:space-y-4">
      <header className="flex flex-col gap-3 lg:flex-row lg:items-center lg:justify-between">
        <div className="flex items-center justify-between gap-2">
          <div className="min-w-0">
            <h1 className="text-title">{t.charts.title}</h1>
            {phase !== undefined && today !== undefined && (
              <p className="text-small text-muted-foreground">{phaseSaid(phase, today, t)}</p>
            )}
          </div>
          <SettingsAvatar className="-my-1.5" />
        </div>
        <SegmentedControl
          legend={t.charts.span}
          hideLegend
          className="lg:w-104"
          options={spans
            .filter((s) => s !== 'phase' || !noPhase)
            .map((s) => ({
              value: s,
              label: (
                <>
                  <span aria-hidden>{t.charts.spans[s]}</span>
                  <span className="sr-only">{t.charts.spansSaid[s]}</span>
                </>
              ),
            }))}
          value={span}
          onChange={(s) => {
            void navigate({
              search: s === fallback ? {} : { days: searchOf[s] },
              replace: true,
            })
          }}
        />
      </header>
      <Progress
        hero={hero}
        charts={charts}
        fallback={body}
        busy={history.isPlaceholderData}
        desktop={desktop}
      />
    </section>
  )
}

// balanceOf is what was eaten, the last 7 days' average, against the TDEE
// of the last day with both, in a sentence: about so many kcal under or
// over it, to 50, and the weight that moves a week, at the server's kcal
// a kg (limits.json).
function balanceOf(
  days: HistoryDay[],
  t: ReturnType<typeof useMessages>,
  f: ReturnType<typeof useFormat>,
): string | undefined {
  const eaten = eaten7(days)
  for (let i = days.length - 1; i >= 0; i--) {
    const tdee = days[i]?.tdee?.kcal
    const kcal = eaten[i]
    if (tdee === undefined || kcal === null || kcal === undefined) {
      continue
    }
    const gap = Math.round((kcal - tdee) / 50) * 50
    const kg = rounded((gap * 7) / limits.plan.kcalPerKg, 1)
    if (kg === 0) {
      return t.charts.balance.even
    }
    return gap < 0
      ? t.charts.balance.under(f.number(-gap), f.signed(kg, 1))
      : t.charts.balance.over(f.number(gap), f.signed(kg, 1))
  }
  return undefined
}

// Charts are what the charts are drawn of, once the days have come.
interface Charts {
  days: HistoryDay[]
  words: Words
  colors: ChartColors
  phase: Phase | undefined
  weeks: { from: string; to: string; count: number }
  today: string
  plan: Plan | undefined
}

// Progress is the hero beside the charts of the days, the measurements in
// small and the table of the weeks: on a desktop the hero beside the
// TDEE, the weight beside the measurements; on a phone one after the
// other, the weight first. Until the charts come (or when they cannot),
// fallback stands in their place, the hero where it stays, so it is not
// drawn again when they come.
function Progress({
  hero,
  charts,
  fallback,
  busy,
  desktop,
}: {
  hero: ReactNode
  charts: Charts | undefined
  fallback: ReactNode
  busy: boolean
  desktop: boolean
}) {
  const cards = useCards(charts)

  return (
    <div
      aria-busy={busy}
      className={cn('space-y-3 transition-opacity lg:space-y-4', busy && 'opacity-60')}
    >
      {desktop ? (
        <>
          <div className="grid grid-cols-[380px_minmax(0,1fr)] items-start gap-4">
            {hero}
            {cards?.tdee ?? fallback}
          </div>
          {cards !== undefined && (
            <>
              <div className="grid grid-cols-2 items-start gap-4">
                {cards.weight}
                {cards.measurements}
              </div>
              <div className="grid grid-cols-2 items-start gap-4">{cards.weekly}</div>
            </>
          )}
        </>
      ) : (
        <>
          {hero}
          {cards === undefined ? (
            fallback
          ) : (
            <>
              {cards.weight}
              {cards.tdee}
              {cards.measurements}
              {cards.weekly}
            </>
          )}
        </>
      )}
    </div>
  )
}

// useCards are the cards of the charts, once they have come: the weight,
// the TDEE (its more layers a switch of its own), the measurements in
// small and the weeks.
function useCards(
  charts: Charts | undefined,
): { weight: ReactNode; tdee: ReactNode; measurements: ReactNode; weekly: ReactNode } | undefined {
  const t = useMessages()
  const f = useFormat()
  const [more, setMore] = useState(false)
  if (charts === undefined) {
    return undefined
  }
  const { days, words, colors, phase, weeks, today, plan } = charts
  const said = t.charts.said
  const none = said.none
  const names = words.names
  const tips = words.tips

  // Each chart from its first data.
  const weightDays = fromFirst(days, (d) => d.weight_kg !== undefined || d.trend_kg !== undefined)
  const tdeeDays = fromFirst(
    days,
    (d) => d.tdee !== undefined || (counted(d) && d.intake !== undefined),
  )

  const trends = weightDays.flatMap((d) => (d.trend_kg === undefined ? [] : [d.trend_kg]))
  const firstTrend = trends[0]
  const lastTrend = trends.at(-1)
  const weighIns = weightDays.filter((d) => d.weight_kg !== undefined).length
  const lastTDEE = tdeeDays.findLast((d) => d.tdee !== undefined)?.tdee
  // The days logged, as calibration counts them: over, with a meal, not
  // left out. Today is open, its intake not all eaten yet.
  const logged = tdeeDays.filter((d) => counted(d) && d.intake !== undefined)
  const kcal = averageOf(logged.map((d) => d.intake?.kcal ?? null))

  // Weigh-ins and no trend are today's alone, which the trend takes once
  // the day is over.
  let weightLabel = none
  if (firstTrend !== undefined && lastTrend !== undefined) {
    weightLabel = said.weight(f.number(firstTrend, 1), f.number(lastTrend, 1), weighIns)
  } else if (weighIns > 0) {
    weightLabel = said.weighInsToday(weighIns)
  }
  const tdeeSaid = lastTDEE
    ? said.tdee(
        f.number(lastTDEE.kcal),
        f.number(lastTDEE.half_width),
        lastTDEE.trusted ? (lastTDEE.before_phase ? 'before' : 'trusted') : 'unsure',
      )
    : undefined
  const eatenSaid = kcal === undefined ? undefined : said.eaten(f.number(kcal), logged.length)
  const goal = phase?.goal_weight
  // A goal far off the weights is said, not drawn.
  const goalDrawn = goalShown(weightDays, goal)
  const weightNote = [
    firstTrend === undefined || lastTrend === undefined || trends.length < 2
      ? undefined
      : t.charts.change(f.signed(lastTrend - firstTrend, 1)),
    goal === undefined || goalDrawn ? undefined : t.charts.goal(f.number(goal, 1)),
  ].filter((s) => s !== undefined)

  // A proposal that waits, on the chart it cites: the weight's when it
  // changes the phases alone, else the TDEE's.
  const pending = plan?.pending
  const cited: Cited | undefined =
    pending === undefined ? undefined : { window: pending.window, label: names.proposal }
  const onWeight = pending !== undefined && citesWeight(pending)
  const coach = pending !== undefined && plan !== undefined && (
    <CoachLine proposal={pending} plan={plan} />
  )

  const tdeeMarks: Mark[] = [
    { name: names.tdeeBand, mark: 'lineBand', color: 'bg-chart-2' },
    { name: names.intake, mark: 'bar', color: 'bg-chart-5' },
    ...(more
      ? [
          { name: names.gap, mark: 'band' as const, color: 'bg-chart-5' },
          { name: names.target, mark: 'dashed' as const, color: 'border-warning' },
        ]
      : []),
  ]

  const weightCard = (
    <ChartCard
      title={t.charts.weight}
      info={
        <InfoTip label={t.info.trend.label} title={t.info.trend.title}>
          {t.info.trend.text}
        </InfoTip>
      }
      note={weightNote.length === 0 ? undefined : weightNote.join(' · ')}
      marks={[
        { name: names.weighIn, mark: 'dot', color: 'bg-chart-5' },
        { name: names.trend, mark: 'line', color: 'bg-chart-1' },
        ...(goalDrawn
          ? [{ name: names.goal, mark: 'dashed' as const, color: 'border-warning' }]
          : []),
      ]}
      option={weightOption(weightDays, words, colors, goal, onWeight ? cited : undefined)}
      label={weightLabel}
      table={tableOf(weightDays, words, [names.weighIn, names.trend], (d) => [
        d.weight_kg === undefined ? undefined : f.number(d.weight_kg, 1),
        d.trend_kg === undefined ? undefined : f.number(d.trend_kg, 1),
      ])}
      footer={onWeight && coach}
    />
  )
  const tdeeCard = (
    <ChartCard
      title={t.charts.tdee}
      note={kcal === undefined ? undefined : t.charts.eaten(f.number(kcal), logged.length)}
      lead={balanceOf(tdeeDays, t, f)}
      marks={tdeeMarks}
      option={tdeeOption(tdeeDays, words, colors, {
        more,
        cited: onWeight ? undefined : cited,
      })}
      label={[tdeeSaid, eatenSaid].filter((s) => s !== undefined).join(' ') || none}
      table={tableOf(tdeeDays, words, [names.tdee, names.intake, names.target], (d) => [
        d.tdee === undefined
          ? undefined
          : tips.tdee(f.number(d.tdee.kcal), f.number(d.tdee.half_width)).replace(/^TDEE /, ''),
        d.intake === undefined || !counted(d) ? undefined : f.number(d.intake.kcal),
        d.target === undefined ? undefined : f.number(d.target.kcal),
      ])}
      more={{ shown: more, onChange: setMore }}
      footer={!onWeight && coach}
    />
  )
  return {
    weight: weightCard,
    tdee: tdeeCard,
    measurements: <Measurements days={days} words={words} colors={colors} />,
    weekly: weeks.count > 0 && <WeeklyTable weeks={weeks} today={today} />,
  }
}

// usualOf is how the last 7 days' average (now) stands against its base,
// as both are shown: by so much (by), which way, and whether that is
// worse, as recovery flags it.
function usualOf(
  now: number | undefined,
  base: number | undefined,
  by: (now: number, base: number) => { difference: number; said: string },
  worse: boolean,
  t: ReturnType<typeof useMessages>,
): Usual | undefined {
  if (now === undefined || base === undefined) {
    return undefined
  }
  const { difference, said } = by(now, base)
  if (difference === 0) {
    return { said: t.charts.asUsual, way: 'same', worse: false }
  }
  return { said: t.charts.vsUsual(said), way: difference > 0 ? 'up' : 'down', worse }
}

// Measurements are sleep, the resting heart rate, the HRV, the steps and
// the body fat in small, each that has a value in the days, from its
// first. Sleep, the heart rate and the HRV are the last 7 days' average,
// as recovery has them (GET /api/v1/summary), against the usual, the 21
// days before them, whose range a caption says under the line; until it
// has them, the days' average.
function Measurements({
  days,
  words,
  colors,
}: {
  days: HistoryDay[]
  words: Words
  colors: ChartColors
}) {
  const t = useMessages()
  const f = useFormat()
  const said = t.charts.said
  const tips = t.charts.tips
  const recovery = $api.useQuery('get', '/api/v1/summary').data?.recovery
  const flags = recovery?.flags ?? []

  const sleep = trimmed(
    days,
    days.map((d): Point => d.sleep_h ?? null),
  )
  const rhr = trimmed(
    days,
    days.map((d): Point => d.resting_hr ?? null),
  )
  const hrv = trimmed(
    days,
    days.map((d): Point => d.hrv ?? null),
  )
  const steps = trimmed(days, stepsOf(days))
  const fat = trimmed(
    days,
    days.map((d): Point => d.body_fat_pct ?? null),
  )
  const sleepAverage = averageOf(sleep.values)
  const rhrAverage = averageOf(rhr.values)
  const hrvAverage = averageOf(hrv.values)
  const stepsAverage = averageOf(steps.values)
  const lastFat = fat.values.findLast((v): v is number => v !== null)
  // A value of the last 7 days', else the days' average.
  const caption = (lastWeek: number | undefined) =>
    lastWeek === undefined ? t.charts.average : t.charts.lastWeek
  // The usual range, in words under the line.
  const usualSaid = (range: { low: number; high: number } | undefined) =>
    range === undefined ? undefined : t.charts.usual(f.number(range.low), f.number(range.high))
  // A measurement's days as a table of one column.
  const table = (
    of: { days: HistoryDay[]; values: Point[] },
    name: string,
    write: (v: number) => string,
  ) =>
    tableOf(of.days, words, [name], (_, i) => {
      const v = of.values[i]
      return [v === null || v === undefined ? undefined : write(v)]
    })

  const tiles = [
    sleepAverage !== undefined && (
      <MetricTile
        key="sleep"
        title={t.charts.sleep}
        value={words.hm(recovery?.sleep_h ?? sleepAverage)}
        caption={caption(recovery?.sleep_h)}
        said={said.sleep(words.hm(sleepAverage))}
        values={sleep.values}
        legend={<SleepLegend />}
        option={sleepOption(sleep.days, words, colors)}
        table={table(sleep, t.charts.sleep, words.hm)}
      />
    ),
    rhrAverage !== undefined && (
      <MetricTile
        key="rhr"
        title={t.charts.rhr}
        value={tips.bpm(f.number(recovery?.resting_hr ?? rhrAverage))}
        caption={caption(recovery?.resting_hr)}
        // So many bpm, as both are shown.
        usual={usualOf(
          recovery?.resting_hr,
          recovery?.resting_hr_base,
          (now, base) => {
            const difference = rounded(now) - rounded(base)
            return { difference, said: f.signed(difference) }
          },
          flags.includes('high_resting_hr'),
          t,
        )}
        said={said.rhr(f.number(rhrAverage))}
        values={rhr.values}
        band={recovery?.resting_hr_usual}
        bandSaid={usualSaid(recovery?.resting_hr_usual)}
        option={lineOption(rhr.days, words, colors, {
          name: words.names.rhr,
          color: colors.blue,
          values: rhr.values,
          axis: (n) => words.number(n),
          tip: (v) => tips.bpm(f.number(v)),
          usual: recovery?.resting_hr_usual,
        })}
        table={table(rhr, t.charts.rhr, (v) => tips.bpm(f.number(v)))}
      />
    ),
    hrvAverage !== undefined && (
      <MetricTile
        key="hrv"
        title={t.charts.hrv}
        value={tips.ms(f.number(recovery?.hrv ?? hrvAverage))}
        caption={caption(recovery?.hrv)}
        // A share of the base, as an HRV's own level is the person's.
        usual={usualOf(
          recovery?.hrv,
          recovery?.hrv_base,
          (now, base) => {
            const difference = rounded(((now - base) / base) * 100)
            return { difference, said: f.signedPercent(difference) }
          },
          flags.includes('low_hrv'),
          t,
        )}
        said={said.hrv(f.number(hrvAverage))}
        values={hrv.values}
        band={recovery?.hrv_usual}
        bandSaid={usualSaid(recovery?.hrv_usual)}
        option={lineOption(hrv.days, words, colors, {
          name: words.names.hrv,
          color: colors.blue,
          values: hrv.values,
          axis: (n) => words.number(n),
          tip: (v) => tips.ms(f.number(v)),
          usual: recovery?.hrv_usual,
        })}
        table={table(hrv, t.charts.hrv, (v) => tips.ms(f.number(v)))}
      />
    ),
    stepsAverage !== undefined && (
      <MetricTile
        key="steps"
        title={t.charts.steps}
        value={f.number(stepsAverage)}
        caption={t.charts.average}
        said={said.steps(f.number(stepsAverage))}
        values={steps.values}
        option={stepsOption(steps.days, words, colors)}
        table={table(steps, t.charts.steps, (v) => f.number(v))}
      />
    ),
    lastFat !== undefined && (
      <MetricTile
        key="fat"
        title={t.charts.bodyFat}
        value={f.percent(lastFat, 1)}
        caption={t.charts.latest}
        said={said.bodyFat(f.percent(lastFat, 1))}
        values={fat.values}
        option={lineOption(fat.days, words, colors, {
          name: words.names.bodyFat,
          color: colors.blue,
          values: fat.values,
          axis: (n) => words.percent(n),
          tip: (v) => f.percent(v, 1),
        })}
        table={table(fat, t.charts.bodyFat, (v) => f.percent(v, 1))}
      />
    ),
  ].filter((tile) => tile !== false)

  if (tiles.length === 0) {
    return null
  }
  return (
    <section aria-labelledby="progress-body" className="space-y-2">
      <h2 id="progress-body" className="text-h3">
        {t.charts.body}
      </h2>
      <ul className="grid grid-cols-2 gap-3 sm:grid-cols-3 lg:grid-cols-2 lg:gap-4 xl:grid-cols-3">
        {tiles}
      </ul>
    </section>
  )
}

// SleepLegend says what the colours of the nights' bars of the whole
// chart mean, and its 7 days' average.
function SleepLegend() {
  const names = useMessages().charts.names

  return (
    <Legend
      marks={[
        { name: names.long, mark: 'bar', color: 'bg-chart-2' },
        { name: names.middle, mark: 'bar', color: 'bg-chart-3' },
        { name: names.short, mark: 'bar', color: 'bg-destructive' },
        { name: names.average, mark: 'dashed', color: 'border-chart-5' },
      ]}
    />
  )
}

// Empty is the page of days with nothing logged, and the first thing to do.
function Empty() {
  const t = useMessages()

  return (
    <div className={cn(card, 'flex flex-col items-center gap-3 py-10 text-center', entry)}>
      <ChartLine aria-hidden className="size-8 text-muted-foreground" />
      <p className="text-h3">{t.charts.none}</p>
      <p className="max-w-sm text-small text-muted-foreground">{t.charts.noneHint}</p>
      <Link to="/" search={{ log: true }} className={buttonVariants()}>
        {t.charts.logMeal}
      </Link>
    </div>
  )
}

// HeroSkeleton stands in the hero's place until the app knows the day.
function HeroSkeleton() {
  return (
    <div className={cn(card, 'space-y-3 lg:p-6')}>
      <Skeleton className="h-4 w-40" />
      <Skeleton className="h-8 w-48" />
      <Skeleton className="h-5 w-full max-w-md" />
    </div>
  )
}

// ProgressSkeleton stands in the charts' place, in their shape, while
// their days come.
function ProgressSkeleton() {
  const t = useMessages()

  return (
    <div aria-busy className="space-y-3 lg:space-y-4">
      <output className="sr-only">{t.loading}</output>
      <div className="grid gap-3 lg:grid-cols-2 lg:gap-4">
        {[0, 1].map((i) => (
          <div key={i} className={cn(card, 'space-y-3')}>
            <div className="space-y-1.5">
              <Skeleton className="h-5 w-32" />
              <Skeleton className="h-4 w-48" />
            </div>
            <Skeleton className="h-4 w-56" />
            <Skeleton className="h-60" />
          </div>
        ))}
      </div>
      <div className="grid gap-3 lg:grid-cols-2 lg:items-start lg:gap-4">
        <div className={cn(card, 'space-y-3')}>
          <Skeleton className="h-5 w-24" />
          {[0, 1, 2, 3].map((i) => (
            <Skeleton key={i} className="h-7" />
          ))}
        </div>
        <div className="grid grid-cols-2 gap-3 sm:grid-cols-3 lg:grid-cols-2 lg:gap-4 xl:grid-cols-3">
          {[0, 1, 2].map((i) => (
            <Skeleton key={i} className="h-36 rounded-lg" />
          ))}
        </div>
      </div>
    </div>
  )
}
