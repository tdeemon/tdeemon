import { Hourglass } from 'lucide-react'
import { useId } from 'react'

import type { components } from '@/api/schema'
import { CalibrationCounts } from '@/components/progress/calibration-counts'
import { PlanRow } from '@/components/progress/plan-row'
import { recoverySaid } from '@/components/progress/said'
import { entry } from '@/components/progress/styles'
import { InfoTip } from '@/components/ui/info-tip'
import { useFormat } from '@/i18n/format'
import { useMessages } from '@/i18n/messages'
import { cn } from '@/lib/utils'

type Summary = components['schemas']['SummaryBody']

// CalibrationCard is the progress's hero while there is no TDEE yet:
// calibration on its way to one, what it counts against what it needs
// and the day a first may come, each the summary's (DESIGN.md §7).
// Beside it the trend weight, how the body recovered, and without a
// phase or a target the way to the plan (missing).
export function CalibrationCard({
  summary,
  missing,
}: {
  summary: Summary
  missing?: 'no_phase' | 'no_target' | undefined
}) {
  const t = useMessages()
  const f = useFormat()
  const id = useId()
  const c = t.calibration
  const i = t.info
  const { tdee, trend, recovery } = summary
  const recovered = recoverySaid(recovery, t, f)

  return (
    <section
      aria-labelledby={id}
      className={cn('space-y-4 rounded-lg border bg-card p-4 lg:p-6', entry)}
    >
      <div className="flex items-start gap-3">
        <Hourglass aria-hidden className="mt-1 size-6 shrink-0 text-muted-foreground lg:size-7" />
        <div className="min-w-0 flex-1 space-y-2">
          <div className="flex items-center gap-2">
            <h2 id={id} className="text-h2 lg:text-title">
              {c.title}
            </h2>
            <InfoTip label={i.tdee.label} title={i.tdee.title}>
              {i.tdee.text}
            </InfoTip>
          </div>
          <p className="max-w-prose text-muted-foreground">{c.lead}</p>
          <CalibrationCounts needs={tdee.needs} />
          <p className="max-w-prose font-medium">
            {tdee.expected === undefined ? c.start : c.expected(f.short(tdee.expected))}
          </p>
          {recovered !== undefined && <p className="max-w-prose">{recovered}</p>}
        </div>
      </div>
      {trend !== undefined && (
        <p className="flex items-center gap-2 pl-9 text-small lg:pl-10">
          {t.charts.verdict.trend(f.number(trend.kg, 1))}
          <InfoTip label={i.trend.label} title={i.trend.title}>
            {i.trend.text}
          </InfoTip>
        </p>
      )}
      {missing !== undefined && <PlanRow missing={missing} />}
    </section>
  )
}
