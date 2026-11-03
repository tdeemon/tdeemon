import { Link } from '@tanstack/react-router'

import { $api } from '@/api/client'
import type { Day } from '@/api/types'
import { CalibrationRow } from '@/components/day/calibration-row'
import { DayPhase } from '@/components/day/day-phase'
import { DayTypeChip } from '@/components/day/day-type-chip'
import { EnergyRing } from '@/components/day/energy-ring'
import { kcalLeft, kcalStanding } from '@/components/day/left'
import { MacroBar, type Macro } from '@/components/day/macro-bar'
import { weekOf } from '@/components/day/week'
import { buttonVariants } from '@/components/ui/button'
import { InfoTip } from '@/components/ui/info-tip'
import { rounded, useFormat } from '@/i18n/format'
import { useMessages } from '@/i18n/messages'

const macros: readonly Macro[] = ['protein', 'carbs', 'fat']

// DaySummary is the day at a glance, its hero: what is left of its
// target in the ring, the one number of what is left on the day; beside
// it what was eaten and the target, the target's arithmetic (the TDEE
// calibration measures, which leads to the progress, less the planned
// deficit or plus the surplus, an ⓘ away from what the TDEE is), or
// until calibration has a TDEE, how far it is from a first; the day's
// type, a small picker, and the phase the day is in under it; then each
// macro against its target, beside the ring on a desktop. A day before
// that went over its target says it in a sentence, beside its TDEE, not
// in an error's colour. A day before any target shows what was eaten,
// and the way to a plan.
export function DaySummary({ day }: { day: Day }) {
  const t = useMessages()
  const f = useFormat()
  const { total, target } = day
  const summary = $api.useQuery('get', '/api/v1/summary')
  const me = $api.useQuery('get', '/api/v1/me')
  const today = me.data?.today
  const past = today !== undefined && day.date < today
  // A day before's TDEE is the one it had then, from the history of its
  // week, which the week has asked for; today's, the summary's.
  const history = $api.useQuery(
    'get',
    '/api/v1/history',
    { params: { query: weekOf(day.date, today ?? day.date) } },
    { enabled: past },
  )
  // Calibration on its way to a first TDEE, in the TDEE's place: now's,
  // so not of a day before.
  const calibrating = !past && summary.data?.tdee.measured === false ? summary.data.tdee : undefined
  // The day's type, and the phase under it.
  const kind = (
    <div className="flex flex-col items-start gap-2">
      <DayTypeChip key={`type ${day.date}`} day={day} />
      <DayPhase date={day.date} />
    </div>
  )

  if (target === undefined) {
    return (
      <section aria-labelledby="day-summary" className="space-y-3 rounded-lg border bg-card p-4">
        <h2 id="day-summary" className="text-small text-muted-foreground">
          {t.day.eaten}
        </h2>
        <p className="text-title">{t.unit.kcal(f.number(total.kcal))}</p>
        <p className="text-small text-muted-foreground">
          {macros.map((m) => `${t.day.short[m]} ${f.grams(total[m])}`).join(' · ')}
        </p>
        {calibrating !== undefined && (
          <dl className="space-y-1 text-small">
            <CalibrationRow tdee={calibrating} />
          </dl>
        )}
        {kind}
        <NoTarget date={day.date} />
      </section>
    )
  }

  const s = t.day.summary
  const tdee = past
    ? history.data?.days.find((d) => d.date === day.date)?.tdee
    : summary.data?.tdee.measured === true
      ? summary.data.tdee
      : undefined
  const measured = tdee?.kcal
  // What the TDEE leaves after what was eaten, both as shown; below 0
  // past it: said of a day before over its target, never as a number of
  // what is left.
  const left = measured === undefined ? undefined : kcalLeft(total.kcal, measured)
  const over = -kcalLeft(total.kcal, target.kcal)

  return (
    <section
      aria-labelledby="day-summary"
      className="space-y-4 rounded-lg border bg-card p-4 lg:p-5"
    >
      <h2 id="day-summary" className="sr-only">
        {s.title}
      </h2>
      <div className="grid grid-cols-[auto_minmax(0,1fr)] items-center gap-x-4 gap-y-4 lg:gap-x-7">
        <div className="lg:row-span-2">
          <EnergyRing eaten={total.kcal} target={target.kcal} past={past} />
        </div>
        <div className="flex min-w-0 flex-col items-start gap-2 lg:flex-row lg:flex-wrap lg:items-end lg:justify-between lg:gap-x-6">
          <div className="space-y-1.5">
            <dl className="flex gap-5 lg:gap-7">
              <div>
                <dt className="text-caption text-muted-foreground">{t.day.eaten}</dt>
                <dd className="text-h3 font-semibold tabular-nums lg:text-h2">
                  {f.number(total.kcal)}
                </dd>
              </div>
              <div>
                <dt className="text-caption text-muted-foreground">{s.target}</dt>
                <dd className="text-h3 font-semibold tabular-nums lg:text-h2">
                  {f.number(target.kcal)}
                </dd>
              </div>
            </dl>
            {measured !== undefined && (
              <Equation target={target.kcal} tdee={measured} trusted={tdee?.trusted === true} />
            )}
            {calibrating !== undefined && (
              <dl className="space-y-0.5 text-small">
                <CalibrationRow tdee={calibrating} />
              </dl>
            )}
          </div>
          {kind}
        </div>
        <div className="col-span-2 space-y-3 lg:col-span-1 lg:col-start-2 lg:grid lg:grid-cols-3 lg:gap-6 lg:space-y-0">
          {macros.map((m) => (
            <MacroBar key={m} macro={m} eaten={total[m]} target={target[m]} />
          ))}
        </div>
      </div>
      {past && left !== undefined && kcalStanding(total.kcal, target.kcal) === 'over' && (
        <p className="text-small">
          {left < 0
            ? s.overOverTdee(f.number(over), f.number(-left))
            : s.overUnderTdee(f.number(over), f.number(left))}
        </p>
      )}
    </section>
  )
}

// Equation is the target's arithmetic, in a line: "Target 2,150 = TDEE
// 2,679 − 529 deficit", or + a surplus, or the TDEE alone at maintenance;
// the TDEE about so much (≈) while calibration does not trust it, a link
// to the progress, where it is drawn, and an ⓘ away from what it is.
function Equation({ target, tdee, trusted }: { target: number; tdee: number; trusted: boolean }) {
  const t = useMessages()
  const f = useFormat()
  const e = t.day.summary.equation
  const gap = rounded(tdee) - rounded(target)

  return (
    <p className="text-caption text-muted-foreground">
      {e.target(f.number(target))} ={' '}
      <Link
        to="/progress"
        className="font-semibold text-foreground underline decoration-muted-foreground/50 underline-offset-2 outline-none hover:decoration-foreground focus-visible:ring-2 focus-visible:ring-ring"
      >
        {e.tdee(`${trusted ? '' : '≈'}${f.number(tdee)}`)}
      </Link>
      {gap > 0 && ` ${e.deficit(f.number(gap))}`}
      {gap < 0 && ` ${e.surplus(f.number(-gap))}`}{' '}
      <InfoTip label={t.info.tdee.label} title={t.info.tdee.title} className="align-text-bottom">
        {t.info.tdee.text}
      </InfoTip>
    </p>
  )
}

// NoTarget says why a day has no target, with the way to the plan: a day
// before the plan's first target, when the plan starts; a plan of phases
// without a target yet, that it gives none; else, before a plan, that a
// plan gives the day one.
function NoTarget({ date }: { date: string }) {
  const t = useMessages()
  const f = useFormat()
  const s = t.day.summary
  const plan = $api.useQuery('get', '/api/v1/plan')
  const first = plan.data?.targets[0]?.effective_from
  let words = s.noTarget
  let link = s.setPlan
  if (first !== undefined && date < first) {
    words = s.startsOn(f.short(first))
    link = s.seePlan
  } else if (first === undefined && (plan.data?.phases.length ?? 0) > 0) {
    words = s.noTargetYet
    link = s.seePlan
  }

  return (
    <div className="flex flex-wrap items-center justify-between gap-3 rounded-md bg-surface-2 p-3">
      <p className="text-small">{words}</p>
      <Link to="/plan" className={buttonVariants({ variant: 'outline', size: 'sm' })}>
        {link}
      </Link>
    </div>
  )
}
