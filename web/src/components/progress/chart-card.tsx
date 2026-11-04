import { useId, useState, type ReactNode } from 'react'

import { Chart } from '@/components/charts/chart'
import { Legend, type Mark } from '@/components/charts/legend'
import type { Option } from '@/components/charts/options'
import { DayTable, Toggle, type DayRows } from '@/components/progress/day-table'
import { card, entry } from '@/components/progress/styles'
import { useMessages } from '@/i18n/messages'
import { cn } from '@/lib/utils'

// ChartCard is one of the progress's main charts: its title (with an ⓘ,
// info) and a line of what it shows (note), the sentence it comes to
// (lead), the legend of its marks, and the chart, which label sums up for
// a screen reader; or, its Table pressed, its values by day. more is its
// switch of more layers, when it has them; footer what comes under it (a
// coach's proposal that cites it). Its tooltip stays within it.
export function ChartCard({
  title,
  info,
  note,
  lead,
  marks,
  option,
  label,
  table,
  more,
  footer,
}: {
  title: string
  info?: ReactNode
  note?: string | undefined
  lead?: string | undefined
  marks: Mark[]
  option: Option
  label: string
  table: DayRows
  more?: { shown: boolean; onChange: (shown: boolean) => void } | undefined
  footer?: ReactNode
}) {
  const t = useMessages()
  const id = useId()
  const [tabled, setTabled] = useState(false)

  return (
    <section aria-labelledby={id} className={cn(card, 'min-w-0 space-y-3 overflow-hidden', entry)}>
      <div className="flex flex-wrap items-start justify-between gap-x-3 gap-y-1">
        <div className="min-w-0 space-y-0.5">
          <h2 className="flex items-center gap-1.5 text-h3">
            <span id={id}>{title}</span>
            {info}
          </h2>
          {note !== undefined && <p className="text-small text-muted-foreground">{note}</p>}
        </div>
        <div className="flex shrink-0 items-center gap-4">
          {more !== undefined && !tabled && (
            <Toggle pressed={more.shown} onChange={more.onChange}>
              {t.charts.moreLayers}
            </Toggle>
          )}
          <Toggle pressed={tabled} onChange={setTabled}>
            {t.charts.table}
          </Toggle>
        </div>
      </div>
      {lead !== undefined && <p className="font-medium">{lead}</p>}
      {tabled ? (
        <DayTable title={title} table={table} />
      ) : (
        <>
          <Legend marks={marks} />
          <Chart option={option} label={label} />
        </>
      )}
      {footer}
    </section>
  )
}
