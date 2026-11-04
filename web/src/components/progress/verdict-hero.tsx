import { CircleAlert, CircleCheck, type LucideIcon } from 'lucide-react'
import { useId } from 'react'

import { $api } from '@/api/client'
import type { components } from '@/api/schema'
import { LoadError } from '@/components/load-error'
import { CalibrationCard } from '@/components/progress/calibration-card'
import { PlanRow } from '@/components/progress/plan-row'
import { doubtsSaid, recoverySaid } from '@/components/progress/said'
import { Skeleton } from '@/components/progress/skeleton'
import { card, entry } from '@/components/progress/styles'
import { InfoTip } from '@/components/ui/info-tip'
import type { Messages } from '@/i18n/en'
import { addDays, daysBetween, useFormat } from '@/i18n/format'
import { useMessages } from '@/i18n/messages'
import { cn } from '@/lib/utils'

type Verdict = components['schemas']['VerdictBody']
type Tdee = components['schemas']['TdeeBody']
type Phase = components['schemas']['PhaseBody']
type Said = Messages['charts']['verdict']
type Format = ReturnType<typeof useFormat>

// Rated is a verdict on the phase: on track, or faster or slower than
// its range.
type Rated = 'on_target' | 'fast' | 'slow'

// ratedOf is the verdict's, when it has one. Below the range is a faster
// loss or a slower gain than planned, above it the other way; a phase
// that keeps the weight (a rate of 0) moves too fast either way.
function ratedOf(v: Verdict): Rated | undefined {
  if (v.rate === 'none' || v.target_pct_week === undefined) {
    return undefined
  }
  if (v.rate === 'on_target') {
    return 'on_target'
  }
  if (v.target_pct_week === 0) {
    return 'fast'
  }
  return (v.rate === 'below') === v.target_pct_week < 0 ? 'fast' : 'slow'
}

// The verdict's look: on track the brand's, off the range the
// warning's; its headline and icon say which, never the colour alone.
const looks: Record<Rated, { icon: LucideIcon; box: string; tint: string }> = {
  on_target: { icon: CircleCheck, box: 'bg-primary-soft', tint: 'text-primary' },
  fast: { icon: CircleAlert, box: 'bg-warning-soft', tint: 'text-warning' },
  slow: { icon: CircleAlert, box: 'bg-warning-soft', tint: 'text-warning' },
}

// VerdictHero is the page's one fact that matters most, from the summary
// (GET /api/v1/summary): the TDEE measured, ± its interval and whether it
// is reliable, and the sum it comes of (what was eaten, and the energy
// the trend weight's change stored or released); under it the verdict on
// the phase, or, while the phase is new, a bar to the day of its first,
// or the way to a plan without a phase or a target to judge by. Until
// there is a TDEE, calibration's card stands in its place. The TDEE, its
// trust and the verdict are each an ⓘ away from what they are.
export function VerdictHero({ phase, today }: { phase: Phase | undefined; today: string }) {
  const t = useMessages()
  const f = useFormat()
  const id = useId()
  const summary = $api.useQuery('get', '/api/v1/summary')
  const h = t.charts.hero
  const v = t.charts.verdict
  const i = t.info

  if (summary.data === undefined && summary.isError) {
    return (
      <section aria-label={h.title} className={card}>
        <LoadError error={summary.error} onRetry={() => void summary.refetch()} />
      </section>
    )
  }
  if (!summary.data) {
    return (
      <section aria-label={h.title} aria-busy className={cn(card, 'space-y-3 lg:p-6')}>
        <output className="sr-only">{t.loading}</output>
        <Skeleton className="h-4 w-40" />
        <Skeleton className="h-12 w-48" />
        <Skeleton className="h-10 w-full" />
        <Skeleton className="h-5 w-64" />
      </section>
    )
  }

  const { verdict, tdee, recovery } = summary.data
  let missing: 'no_phase' | 'no_target' | undefined
  if (verdict.reasons.includes('no_phase')) {
    missing = 'no_phase'
  } else if (verdict.reasons.includes('no_target')) {
    missing = 'no_target'
  }
  if (!tdee.measured || tdee.kcal === undefined || tdee.half_width === undefined) {
    return <CalibrationCard summary={summary.data} missing={missing} />
  }

  let trust = h.unsure
  if (tdee.trusted) {
    trust = tdee.before_phase ? h.before : h.reliable
  }
  const rated = ratedOf(verdict)
  const recovered = recoverySaid(recovery, t, f)

  let judged
  if (rated !== undefined) {
    const look = looks[rated]
    const Icon = look.icon
    const headlines: Record<Rated, string> = {
      on_target: v.onTarget,
      fast: v.fast,
      slow: v.slow,
    }
    judged = (
      <div className={cn('flex items-start gap-3 rounded-md p-3', look.box)}>
        <Icon aria-hidden className={cn('mt-0.5 size-5 shrink-0', look.tint)} />
        <div className="min-w-0 space-y-0.5">
          <p className="flex items-center gap-2 text-h3">
            {headlines[rated]}
            <InfoTip label={i.verdict.label} title={i.verdict.title}>
              {i.verdict.text}
            </InfoTip>
          </p>
          <p className="text-small">{rateSaid(verdict, rated === 'on_target', v, f)}</p>
        </div>
      </div>
    )
  } else if (missing !== undefined) {
    judged = <PlanRow missing={missing} />
  } else if (verdict.reasons.includes('no_tdee')) {
    judged = <p className="max-w-prose text-small">{v.reasons.no_tdee}</p>
  } else if (tdee.before_phase || verdict.reasons.includes('before_phase')) {
    judged = <FirstVerdict tdee={tdee} phase={phase} today={today} />
  }

  return (
    <section aria-labelledby={id} className={cn(card, 'flex flex-col gap-4 lg:p-6', entry)}>
      <div>
        <div className="flex flex-wrap items-center gap-x-2 text-caption text-muted-foreground">
          <h2 id={id} className="text-caption">
            {h.title}
          </h2>
          <InfoTip label={i.tdee.label} title={i.tdee.title}>
            {i.tdee.text}
          </InfoTip>
          <span className={cn(tdee.trusted && 'text-primary')}>{trust}</span>
          <InfoTip label={i.trusted.label} title={i.trusted.title}>
            {i.trusted.text}
          </InfoTip>
        </div>
        <p className="mt-0.5 flex items-baseline gap-1.5">
          <span className="text-display tabular-nums">{f.number(tdee.kcal)}</span>
          <span className="text-small text-muted-foreground">
            {h.half(f.number(tdee.half_width))}
          </span>
        </p>
      </div>
      <Sum tdee={tdee} />
      {!tdee.trusted && (
        <p className="max-w-prose text-small">{doubtsSaid(tdee, t, f).join(' ')}</p>
      )}
      {judged}
      {recovered !== undefined && <p className="max-w-prose text-small">{recovered}</p>}
    </section>
  )
}

// Sum is the sum the TDEE comes of, as calibration measures it: the
// intake of the window's logged days, and the energy the trend weight's
// change stored or released (its kg a week at 7700 kcal a kg), of the
// logged days from the first weigh-in to the last: "Intake
// 2,209 + trend −0.43 kg/wk (≈ 470) = 2,679", then what it is of. That
// energy is written as the TDEE less the intake, so the sum adds up as it
// is written, each rounded.
function Sum({ tdee }: { tdee: Tdee }) {
  const t = useMessages()
  const f = useFormat()
  const h = t.charts.hero
  const of = (
    <span className="text-muted-foreground">
      · {h.of(tdee.needs.logged_days.has, tdee.weigh_ins)}
    </span>
  )
  if (tdee.kcal === undefined || tdee.intake_kcal === undefined || tdee.kg_per_week === undefined) {
    return <p className="rounded-md bg-surface-2 px-3 py-2.5 text-small">{of}</p>
  }
  const kcal = Math.round(tdee.kcal)
  const intake = Math.round(tdee.intake_kcal)
  // A slope that rounds to 0 kg a week is no term of the sum: the few kcal
  // between the intake and the TDEE are rounding, said by ≈ on the sum.
  const flat = Math.round(tdee.kg_per_week * 100) === 0
  const stored = flat ? 0 : kcal - intake
  const number = (n: number) => <b className="font-semibold tabular-nums">{f.number(n)}</b>
  // The minus sign, not a hyphen, in either language.
  const kg = flat ? f.number(0) : f.signed(tdee.kg_per_week, 2).replace('-', '−')

  return (
    <p className="rounded-md bg-surface-2 px-3 py-2.5 text-small">
      {h.intake} {number(intake)} {stored < 0 ? '−' : '+'} {h.trend(kg)} (≈{' '}
      {number(Math.abs(stored))}) {flat && kcal !== intake ? '≈' : '='} {number(kcal)} {of}
    </p>
  )
}

// FirstVerdict is the verdict's place while the phase is new: the TDEE is
// there, its verdict on the phase not yet. It says so as the morning
// summary does (#312), with a bar of the phase's days toward the first,
// the day after the last the TDEE is of the days before it.
function FirstVerdict({
  tdee,
  phase,
  today,
}: {
  tdee: Tdee
  phase: Phase | undefined
  today: string
}) {
  const t = useMessages()
  const f = useFormat()
  const h = t.charts.hero
  const until = tdee.before_phase_until
  if (until === undefined) {
    return <p className="max-w-prose text-small">{h.begunSoon}</p>
  }
  const first = addDays(until, 1)
  const need = phase === undefined ? 0 : daysBetween(phase.start, first)
  const has = phase === undefined ? 0 : Math.max(0, Math.min(need, daysBetween(phase.start, today)))
  const count = h.daysOf(f.number(has), f.number(need))

  return (
    <div className="space-y-1.5">
      <p className="text-small">{h.begun(f.short(first))}</p>
      {need > 0 && (
        <>
          <p className="flex justify-between gap-2 text-caption text-muted-foreground">
            <span>
              <b className="font-semibold text-foreground">{h.verdict}</b> ·{' '}
              {h.first(f.short(first))}
            </span>
            <span className="tabular-nums">{count}</span>
          </p>
          <progress
            aria-label={h.verdict}
            aria-valuetext={count}
            value={has}
            max={need}
            className="block h-2 w-full appearance-none overflow-hidden rounded-full bg-surface-2 [&::-moz-progress-bar]:rounded-full [&::-moz-progress-bar]:bg-primary [&::-webkit-progress-bar]:bg-surface-2 [&::-webkit-progress-value]:rounded-full [&::-webkit-progress-value]:bg-primary"
          />
        </>
      )}
    </div>
  )
}

// rateSaid is the rate measured, in words, against the phase's range: of
// a loss or a gain, the range as the same (0.65–0.85); of a weight that
// goes the other way, signed (−0.85 to −0.65); of a phase that keeps it,
// around none (± 0.1).
function rateSaid(verdict: Verdict, within: boolean, v: Said, f: Format): string {
  const measured = verdict.measured_pct_week ?? 0
  const target = verdict.target_pct_week ?? 0
  const tolerance = verdict.tolerance_pct_week ?? 0
  const lo = target - tolerance
  const hi = target + tolerance
  let moving = v.steady
  if (measured < 0) {
    moving = v.losing(f.number(-measured, 2))
  } else if (measured > 0) {
    moving = v.gaining(f.number(measured, 2))
  }
  let range: string
  if (target === 0) {
    range = v.around(f.number(tolerance, 2))
  } else if ((lo >= 0 || hi <= 0) && measured * target >= 0) {
    range = v.range(
      f.number(Math.min(Math.abs(lo), Math.abs(hi)), 2),
      f.number(Math.max(Math.abs(lo), Math.abs(hi)), 2),
    )
  } else {
    range = v.span(f.signed(lo, 2), f.signed(hi, 2))
  }
  return within ? v.within(moving, range) : v.outside(moving, range)
}
