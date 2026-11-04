import { Link } from '@tanstack/react-router'

import { $api } from '@/api/client'
import type { components } from '@/api/schema'
import { daysBetween, useFormat } from '@/i18n/format'
import { useMessages } from '@/i18n/messages'
import { cn } from '@/lib/utils'

type Verdict = components['schemas']['VerdictBody']

// The dot of a phase's line: the brand's green on target, the warning's
// amber off it; the words beside it say which.
const dots: Record<Verdict['rate'], string> = {
  none: 'bg-muted-foreground',
  on_target: 'bg-success',
  below: 'bg-warning',
  above: 'bg-warning',
}

// DayPhase is the phase of the plan a day is in, and its week of the
// phase's ("Cut · week 1 of 12"); of the phase now, the rate calibration
// measures and where it stands against the phase's, its dot green on
// target and amber off it, with the words: the hero's subtitle, a link to
// the progress, 44 px to a finger with an ::after reaching 4 px above it
// and 24 below, clear of the day's type's chip over it. Nothing without a
// phase.
export function DayPhase({ date }: { date: string }) {
  const t = useMessages()
  const f = useFormat()
  const plan = $api.useQuery('get', '/api/v1/plan')
  const summary = $api.useQuery('get', '/api/v1/summary')
  const verdict = summary.data?.verdict
  const today = plan.data?.today
  const phase = plan.data?.phases.find(
    (p) => p.start <= date && (p.end === undefined || date <= p.end),
  )
  if (phase === undefined || today === undefined) {
    return null
  }
  const week = Math.floor(daysBetween(phase.start, date) / 7) + 1
  const weeks =
    phase.end === undefined ? undefined : Math.ceil((daysBetween(phase.start, phase.end) + 1) / 7)
  // The verdict is of the phase today is in.
  const now = phase.start <= today && (phase.end === undefined || today <= phase.end)
  const rate = now && verdict?.measured_pct_week !== undefined ? verdict.rate : 'none'

  return (
    <Link
      to="/progress"
      className={cn(
        "relative after:absolute after:inset-x-0 after:-top-1 after:-bottom-6 after:content-['']",
        'flex items-center gap-1.5 text-caption text-muted-foreground outline-none hover:text-foreground focus-visible:ring-2 focus-visible:ring-ring',
      )}
    >
      {rate !== 'none' && (
        <span aria-hidden className={cn('size-2 shrink-0 rounded-full', dots[rate])} />
      )}
      <span>
        {t.plan.change.types[phase.type]} ·{' '}
        {weeks === undefined ? t.day.summary.week(week) : t.day.summary.weekOf(week, weeks)}
        {rate !== 'none' && verdict?.measured_pct_week !== undefined && (
          <>
            {' · '}
            {t.day.summary.rate(f.signed(verdict.measured_pct_week, 1))},{' '}
            {t.day.summary.rates[rate]}
          </>
        )}
      </span>
    </Link>
  )
}
