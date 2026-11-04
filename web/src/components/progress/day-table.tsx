import { useState, type ReactNode } from 'react'

import { Button } from '@/components/ui/button'
import { useMessages } from '@/i18n/messages'
import { linkTarget } from '@/lib/styles'
import { cn } from '@/lib/utils'

// DayRows are a chart's values as a table: a column for each of its
// series, and a row for each day with a value, its date written already.
export interface DayRows {
  columns: string[]
  rows: { key: string; date: string; cells: string[] }[]
}

// The days a table shows before the rest is asked for.
const first = 14

// DayTable is a chart's values by day, the latest first: two weeks of
// them, the rest a tap away.
export function DayTable({ title, table }: { title: string; table: DayRows }) {
  const t = useMessages()
  const [all, setAll] = useState(false)
  const rows = all ? table.rows : table.rows.slice(0, first)

  return (
    <div>
      <table className="w-full text-small">
        <caption className="sr-only">{t.charts.tableOf(title)}</caption>
        <thead className="text-caption text-muted-foreground">
          <tr>
            <th scope="col" className="pb-2 text-left font-medium">
              {t.charts.date}
            </th>
            {table.columns.map((c) => (
              <th key={c} scope="col" className="pb-2 pl-3 text-right font-medium">
                {c}
              </th>
            ))}
          </tr>
        </thead>
        <tbody>
          {rows.map((r) => (
            <tr key={r.key} className="border-t">
              <th scope="row" className="py-2 text-left font-normal whitespace-nowrap">
                {r.date}
              </th>
              {r.cells.map((cell, i) => (
                <td
                  key={table.columns[i]}
                  className="py-2 pl-3 text-right whitespace-nowrap tabular-nums"
                >
                  {cell}
                </td>
              ))}
            </tr>
          ))}
        </tbody>
      </table>
      {table.rows.length > first && (
        <Button
          variant="ghost"
          size="sm"
          className="mt-2 w-full"
          aria-expanded={all}
          onClick={() => {
            setAll((a) => !a)
          }}
        >
          {all ? t.charts.weeks.fewer : t.charts.allDays(table.rows.length)}
        </Button>
      )}
    </div>
  )
}

// Toggle is a chart's switch, a word in the brand's colour, pressed or
// not: its layers, or its values as a table. 44 px to a finger.
export function Toggle({
  pressed,
  onChange,
  label,
  children,
}: {
  pressed: boolean
  onChange: (pressed: boolean) => void
  // Its name, when its words alone would be one of several alike: the
  // chart's ("Weight: table"), its words in it.
  label?: string | undefined
  children: ReactNode
}) {
  return (
    <button
      type="button"
      aria-pressed={pressed}
      aria-label={label}
      onClick={() => {
        onChange(!pressed)
      }}
      className={cn(
        linkTarget,
        'min-w-11 rounded-xs text-small font-semibold text-primary underline-offset-4 outline-none hover:underline focus-visible:ring-2 focus-visible:ring-ring aria-pressed:underline',
      )}
    >
      {children}
    </button>
  )
}
