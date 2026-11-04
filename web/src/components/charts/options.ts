import type { components } from '@/api/schema'

import type { ChartColors } from './palette'

export type HistoryDay = components['schemas']['HistoryDayBody']

// Words is what the options take of the app's language: the series'
// names, what a tooltip says of a value, and how a date, a number and a
// percentage are written.
export interface Words {
  // A day on the axis: its date, short; over a year (months), its month
  // and year, at the first of each month only.
  axisDate: (date: string) => string
  months: boolean
  // A day in a tooltip: its weekday and date.
  day: (date: string) => string
  number: (n: number, digits?: number) => string
  percent: (n: number, digits?: number) => string
  // Hours of sleep, in hours and minutes: 6 h 52.
  hm: (hours: number) => string
  names: {
    weighIn: string
    trend: string
    goal: string
    tdee: string
    interval: string
    tdeeBand: string
    proposal: string
    intake: string
    target: string
    gap: string
    average: string
    rhr: string
    hrv: string
    steps: string
    bodyFat: string
    // The nights by their length, which colours their bars.
    long: string
    middle: string
    short: string
  }
  // A value in a tooltip, its number written already.
  tips: {
    kg: (kg: string) => string
    trend: (kg: string) => string
    tdee: (kcal: string, half: string) => string
    eaten: (kcal: string) => string
    target: (kcal: string) => string
    average: (hours: string) => string
    bpm: (bpm: string) => string
    ms: (ms: string) => string
    steps: (steps: string, count: number) => string
  }
}

// An option is ECharts', as the charts give it to setOption.
export type Option = Record<string, unknown>

// A value of a series: null where the day has none, a gap.
export type Point = number | null

function value<T>(v: T | undefined): T | null {
  return v ?? null
}

// fromFirst are the days from the first that has a value of a chart's
// (has) on: a chart starts at its first data, not at its span's first
// day, which would squeeze its values into a strip. Days of none are as
// they are.
export function fromFirst(days: HistoryDay[], has: (day: HistoryDay) => boolean): HistoryDay[] {
  const first = days.findIndex(has)
  return first <= 0 ? days : days.slice(first)
}

// Cited is what a coach's proposal cites of a chart: the days it was
// drawn from (window), shaded, and a line on the last day, the proposal
// waiting now, which label names.
export interface Cited {
  window?: { from: string; to: string } | undefined
  label: string
}

// citedSeries is a proposal's marks on a chart of days, in the coach's
// colour: a series of no values of its own, so no tooltip or legend
// says it. A window outside the days is not shaded.
function citedSeries(days: HistoryDay[], c: ChartColors, cited: Cited): Option {
  const first = days[0]?.date
  const last = days.at(-1)?.date
  const from =
    cited.window && first !== undefined && cited.window.from < first ? first : cited.window?.from
  const to = cited.window && last !== undefined && cited.window.to > last ? last : cited.window?.to
  return {
    name: 'proposal',
    type: 'line',
    silent: true,
    showSymbol: false,
    lineStyle: { opacity: 0 },
    data: days.map(() => null),
    ...(from !== undefined && to !== undefined && from <= to
      ? {
          markArea: {
            silent: true,
            itemStyle: { color: c.coach, opacity: 0.1 },
            data: [[{ xAxis: from }, { xAxis: to }]],
          },
        }
      : {}),
    ...(last === undefined
      ? {}
      : {
          markLine: {
            symbol: 'none',
            silent: true,
            lineStyle: { color: c.coach, type: 'dashed', width: 1.5 },
            label: {
              color: c.coach,
              fontWeight: 600,
              position: 'end',
              align: 'right',
              formatter: cited.label,
            },
            data: [{ xAxis: last }],
          },
        }),
  }
}

// some are the values of a series that a day has.
function some(values: Point[]): number[] {
  return values.filter((v): v is number => v !== null)
}

// indexOf is the day a tooltip is of, by its place on the axis.
function indexOf(params: unknown): number | undefined {
  const first: unknown = Array.isArray(params) ? params[0] : params
  return typeof first === 'object' &&
    first !== null &&
    'dataIndex' in first &&
    typeof first.dataIndex === 'number'
    ? first.dataIndex
    : undefined
}

// tipOf is what a tooltip says of a day: its weekday and date, and its
// values, in a line.
export function tipOf(day: HistoryDay, w: Words, values: string[]): string {
  return [w.day(day.date), ...values].join(' · ')
}

// Base is what makes a chart's base: how its axis of values writes a
// number, what a tooltip says of a day (the i-th of the days), and the
// axis's reach.
interface Base {
  axis: (n: number) => string
  tip: (day: HistoryDay, i: number) => string[]
  reach?: Option
  // The room right of the grid, for a line's label at its end, and above
  // it, for a proposal's label over its line.
  right?: number
  top?: number
}

// base is what every chart shares: the days along the bottom, one axis of
// values whose labels are written as the language writes numbers, the
// grid's lines and the text in the theme's colours, and a tooltip by
// day. Its legend is its card's (Legend), which wraps on a phone as
// ECharts' would not.
function base(days: HistoryDay[], w: Words, c: ChartColors, of: Base): Option {
  return {
    backgroundColor: 'transparent',
    animation: false,
    // The font is the page's, which the chart is given when it is drawn
    // (Chart): ECharts measures text by a font's name.
    textStyle: { color: c.muted },
    // The axes' labels fit in, however wide their numbers: the margins
    // are the bounds of the grid with its labels.
    grid: {
      left: 4,
      right: of.right ?? 12,
      top: of.top ?? 12,
      bottom: 4,
      outerBoundsMode: 'same',
      outerBoundsContain: 'axisLabel',
    },
    tooltip: {
      trigger: 'axis',
      // Inside the chart, so a phone's page does not scroll sideways.
      confine: true,
      backgroundColor: c.surface,
      borderColor: c.grid,
      textStyle: { color: c.text, fontSize: 13 },
      extraCssText: 'white-space: normal; max-width: 280px;',
      formatter: (params: unknown) => {
        const i = indexOf(params)
        const day = i === undefined ? undefined : days[i]
        return day === undefined || i === undefined ? '' : tipOf(day, w, of.tip(day, i))
      },
    },
    xAxis: {
      type: 'category',
      data: days.map((d) => d.date),
      axisLine: { lineStyle: { color: c.grid } },
      axisTick: { show: false },
      axisLabel: {
        color: c.muted,
        hideOverlap: true,
        formatter: (date: string) => w.axisDate(date),
        // A year's axis names each month once, at its first day.
        ...(w.months ? { interval: (_: number, date: string) => date.endsWith('-01') } : {}),
      },
    },
    yAxis: {
      type: 'value',
      splitLine: { lineStyle: { color: c.grid } },
      axisLabel: { color: c.muted, formatter: of.axis },
      ...of.reach,
    },
  }
}

// line is a series of one line, its dots shown when there are too few
// to draw a line between.
function line(name: string, color: string, data: Point[], extra: Option = {}): Option {
  return {
    name,
    type: 'line',
    showSymbol: some(data).length < 2,
    connectNulls: true,
    lineStyle: { color, width: 2 },
    itemStyle: { color },
    data,
    ...extra,
  }
}

// counted reports whether a day counts in calibration's averages: over
// (today is open, what it has eaten not all yet) and not left out.
export function counted(day: HistoryDay): boolean {
  return day.status === 'complete'
}

// The steps an axis of weights goes by, the least that takes 6 at most.
const axisSteps = [0.2, 0.5, 1, 2, 5, 10, 20, 50, 100]

// reachOf is an axis that holds values, from a step below them to one
// above, so its ends are labels as its steps are; a value alone, a kilo
// each way.
export function reachOf(values: number[]): Option {
  if (values.length === 0) {
    return { scale: true }
  }
  let lo = Math.min(...values)
  let hi = Math.max(...values)
  if (lo === hi) {
    lo -= 1
    hi += 1
  }
  const step = axisSteps.find((s) => Math.ceil(hi / s) - Math.floor(lo / s) <= 6) ?? 500
  return { min: Math.floor(lo / step) * step, max: Math.ceil(hi / step) * step, interval: step }
}

// weightsOf are the weigh-ins and the trend weights of the days.
function weightsOf(days: HistoryDay[]): number[] {
  return days.flatMap((d) => [d.weight_kg, d.trend_kg]).filter((v) => v !== undefined)
}

// goalShown reports whether the weight chart draws a goal: within the
// axis the weights reach, from a step below them to one above. One
// further off would squeeze the weights into a strip; the card says it
// in words instead.
export function goalShown(days: HistoryDay[], goalKg: number | undefined): boolean {
  const kg = weightsOf(days)
  const reach = reachOf(kg)
  return (
    goalKg !== undefined &&
    typeof reach.min === 'number' &&
    typeof reach.max === 'number' &&
    reach.min <= goalKg &&
    goalKg <= reach.max
  )
}

// weightOption is the weigh-ins as faint dots under the trend weight's
// line, its last value written at its end, with the phase's goal as a
// dashed line when it is near them (goalShown), and a proposal that
// cites the weight (cited). The axis reaches the weights alone.
export function weightOption(
  days: HistoryDay[],
  w: Words,
  c: ChartColors,
  goalKg?: number,
  cited?: Cited,
): Option {
  const goal = goalShown(days, goalKg) ? goalKg : undefined
  return {
    ...base(days, w, c, {
      axis: (n) => w.number(n, 1),
      tip: (d) => [
        ...(d.weight_kg === undefined ? [] : [w.tips.kg(w.number(d.weight_kg, 1))]),
        ...(d.trend_kg === undefined ? [] : [w.tips.trend(w.number(d.trend_kg, 1))]),
      ],
      reach: reachOf(weightsOf(days)),
      right: 44,
      ...(cited === undefined ? {} : { top: 28 }),
    }),
    series: [
      {
        name: w.names.weighIn,
        type: 'scatter',
        symbolSize: 7,
        itemStyle: { color: c.neutral, opacity: 0.45 },
        data: days.map((d): Point => value(d.weight_kg)),
      },
      line(
        w.names.trend,
        c.primary,
        days.map((d): Point => value(d.trend_kg)),
        {
          lineStyle: { color: c.primary, width: 2.5 },
          endLabel: {
            show: true,
            color: c.text,
            fontWeight: 600,
            formatter: (p: { value?: unknown }) =>
              typeof p.value === 'number' ? w.number(p.value, 1) : '',
          },
          ...(goal === undefined
            ? {}
            : {
                markLine: {
                  symbol: 'none',
                  silent: true,
                  lineStyle: { color: c.warning, type: 'dashed', width: 1.5 },
                  label: {
                    color: c.warning,
                    position: 'insideEndTop',
                    formatter: `${w.names.goal} ${w.number(goal, 1)}`,
                  },
                  data: [{ yAxis: goal }],
                },
              }),
        },
      ),
      ...(cited === undefined ? [] : [citedSeries(days, c, cited)]),
    ],
  }
}

// eaten7 is what was eaten of the days calibration counts, each day's 7
// days' average up to it.
export function eaten7(days: HistoryDay[]): Point[] {
  return average7(days.map((d): Point => (counted(d) ? value(d.intake?.kcal) : null)))
}

// Layers is what the TDEE chart draws past its first two layers, the
// TDEE in its band and what was eaten: the space between the TDEE and
// the 7 days' average of what was eaten, and the target.
export interface Layers {
  more?: boolean | undefined
  cited?: Cited | undefined
}

// tdeeOption is the TDEE measured up to each day, with its 80 % interval
// as a band, and what was eaten as faint bars; with more layers, the
// space between the TDEE and the 7 days' average of what was eaten
// shaded (a deficit, or a surplus), and the target as its 7 days'
// average, a dashed line: the days' own, a training day's and a rest
// day's in turn, would zigzag (a tooltip says the day's own). What was
// eaten is of the days calibration counts: today's, not over yet, would
// be a bar far below the others. A proposal that cites the intake is
// marked (cited).
export function tdeeOption(
  days: HistoryDay[],
  w: Words,
  c: ChartColors,
  layers: Layers = {},
): Option {
  const eaten = eaten7(days)
  // The gap's foot and its height, on the days that have both.
  const gap = days.map((d, i) => {
    const kcal = eaten[i]
    return d.tdee === undefined || kcal === null || kcal === undefined
      ? undefined
      : { foot: Math.min(d.tdee.kcal, kcal), height: Math.abs(d.tdee.kcal - kcal) }
  })
  const more = [
    {
      // The gap's foot, not drawn: the gap stacks on it.
      name: 'gapFoot',
      type: 'line',
      stack: 'gap',
      showSymbol: false,
      lineStyle: { opacity: 0 },
      data: gap.map((g): Point => g?.foot ?? null),
    },
    {
      name: w.names.gap,
      type: 'line',
      stack: 'gap',
      showSymbol: false,
      lineStyle: { opacity: 0 },
      itemStyle: { color: c.neutral },
      areaStyle: { color: c.neutral, opacity: 0.22 },
      data: gap.map((g): Point => g?.height ?? null),
    },
    line(w.names.target, c.warning, average7(days.map((d): Point => value(d.target?.kcal))), {
      showSymbol: false,
      lineStyle: { color: c.warning, type: 'dashed', width: 1.5 },
    }),
  ]
  return {
    ...base(days, w, c, {
      axis: (n) => w.number(n),
      tip: (d) => [
        ...(d.tdee ? [w.tips.tdee(w.number(d.tdee.kcal), w.number(d.tdee.half_width))] : []),
        ...(d.intake && counted(d) ? [w.tips.eaten(w.number(d.intake.kcal))] : []),
        ...(d.target ? [w.tips.target(w.number(d.target.kcal))] : []),
      ],
      reach: { scale: true },
      ...(layers.cited === undefined ? {} : { top: 28 }),
    }),
    series: [
      {
        name: w.names.intake,
        type: 'bar',
        barMaxWidth: 10,
        itemStyle: { color: c.neutral, opacity: 0.35, borderRadius: [2, 2, 0, 0] },
        data: days.map((d): Point => (counted(d) ? value(d.intake?.kcal) : null)),
      },
      {
        // The band's foot, not drawn: the band stacks on it.
        name: 'low',
        type: 'line',
        stack: 'interval',
        showSymbol: false,
        lineStyle: { opacity: 0 },
        data: days.map((d): Point => (d.tdee ? d.tdee.kcal - d.tdee.half_width : null)),
      },
      {
        name: w.names.interval,
        type: 'line',
        stack: 'interval',
        showSymbol: false,
        lineStyle: { opacity: 0 },
        itemStyle: { color: c.blue },
        areaStyle: { color: c.blue, opacity: 0.16 },
        data: days.map((d): Point => (d.tdee ? 2 * d.tdee.half_width : null)),
      },
      ...(layers.more === true ? more : []),
      line(
        w.names.tdee,
        c.blue,
        days.map((d): Point => value(d.tdee?.kcal)),
      ),
      ...(layers.cited === undefined ? [] : [citedSeries(days, c, layers.cited)]),
    ],
  }
}

// average7 is the average of each day's 7 days up to it that have a value.
export function average7(values: Point[]): Point[] {
  return values.map((_, i) => {
    const week = some(values.slice(Math.max(0, i - 6), i + 1))
    return week.length === 0
      ? null
      : Math.round((week.reduce((a, b) => a + b, 0) / week.length) * 100) / 100
  })
}

// weekly are the averages of each 7 days that have a value, the last 7
// up to the last day, the first maybe fewer.
export function weekly(values: Point[]): Point[] {
  const weeks: Point[] = []
  for (let end = values.length; end > 0; end -= 7) {
    const week = some(values.slice(Math.max(0, end - 7), end))
    weeks.unshift(week.length === 0 ? null : week.reduce((a, b) => a + b, 0) / week.length)
  }
  return weeks
}

export type Night = 'long' | 'middle' | 'short'

// nightOf is a night by its length: 7 hours and more long, 6 to 7 middle,
// less short.
export function nightOf(hours: number): Night {
  if (hours >= 7) {
    return 'long'
  }
  return hours >= 6 ? 'middle' : 'short'
}

// sleepColors are the nights' colours by their length.
export function sleepColors(c: ChartColors): Record<Night, string> {
  return { long: c.blue, middle: c.amber, short: c.bad }
}

// sleepOption is each night's sleep as a bar, coloured by its length (a
// series for each length, which a tooltip and the legend name), and the
// 7 days' average as a dashed line.
export function sleepOption(days: HistoryDay[], w: Words, c: ChartColors): Option {
  const hours = days.map((d): Point => value(d.sleep_h))
  const average = average7(hours)
  const colors = sleepColors(c)
  const nights = (length: Night): Option => ({
    name: w.names[length],
    type: 'bar',
    stack: 'sleep',
    barMaxWidth: 14,
    itemStyle: { color: colors[length], borderRadius: [3, 3, 0, 0] },
    data: hours.map((h) => (h !== null && nightOf(h) === length ? h : null)),
  })
  return {
    ...base(days, w, c, {
      axis: (n) => w.number(n, 1),
      tip: (d, i) => {
        const mean = average[i]
        return [
          ...(d.sleep_h === undefined ? [] : [w.hm(d.sleep_h)]),
          ...(mean === null || mean === undefined ? [] : [w.tips.average(w.hm(mean))]),
        ]
      },
      reach: { min: 0 },
    }),
    series: [
      nights('long'),
      nights('middle'),
      nights('short'),
      line(w.names.average, c.neutral, average, {
        showSymbol: false,
        lineStyle: { color: c.neutral, type: 'dashed', width: 1.5 },
      }),
    ],
  }
}

// lineOption is one measurement as a line on one axis: the resting heart
// rate, the HRV, the body fat, over its usual range when it has one.
// axis writes its axis's numbers, and tip says a day's value.
export function lineOption(
  days: HistoryDay[],
  w: Words,
  c: ChartColors,
  of: {
    name: string
    color: string
    values: Point[]
    axis: (n: number) => string
    tip: (v: number) => string
    // The usual range, shaded behind the line.
    usual?: { low: number; high: number } | undefined
  },
): Option {
  const usual = of.usual
  return {
    ...base(days, w, c, {
      axis: of.axis,
      tip: (_, i) => {
        const v = of.values[i]
        return v === null || v === undefined ? [] : [of.tip(v)]
      },
      reach: { scale: true },
    }),
    series: [
      line(
        of.name,
        of.color,
        of.values,
        usual === undefined
          ? {}
          : {
              markArea: {
                silent: true,
                itemStyle: { color: c.neutral, opacity: 0.16 },
                data: [[{ yAxis: usual.low }, { yAxis: usual.high }]],
              },
            },
      ),
    ],
  }
}

// stepsOf are the days' steps as calibration counts them: an open day's
// still grow, a day left out counts for nothing, and 0 steps are none (a
// phone switched off).
export function stepsOf(days: HistoryDay[]): Point[] {
  return days.map((d) => (counted(d) && (d.steps ?? 0) > 0 ? value(d.steps) : null))
}

// stepsOption is each day's steps as a bar, on one axis.
export function stepsOption(days: HistoryDay[], w: Words, c: ChartColors): Option {
  const steps = stepsOf(days)
  return {
    ...base(days, w, c, {
      axis: (n) => w.number(n),
      tip: (_, i) => {
        const v = steps[i]
        return v === null || v === undefined ? [] : [w.tips.steps(w.number(v), v)]
      },
      reach: { min: 0 },
    }),
    series: [
      {
        name: w.names.steps,
        type: 'bar',
        barMaxWidth: 14,
        itemStyle: { color: c.blue, borderRadius: [3, 3, 0, 0] },
        data: steps,
      },
    ],
  }
}
