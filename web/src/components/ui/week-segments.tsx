import { useId, type ReactNode } from 'react'

import { week, type Weekday } from '@/components/autopilot/weekday'
import { useLocale } from '@/i18n/locale'
import { cn } from '@/lib/utils'

// The control's and a day's classes: a segmented control of seven
// columns, Monday first, one row at any width, no gap between them so
// each is 44 px wide on a phone; a day chosen is the
// brand's soft green, as a chosen segment and a chip are.
const rowClass = 'grid grid-cols-7 rounded-md border border-control-border bg-surface-2 p-1'
const dayClass =
  'flex h-11 min-w-0 cursor-pointer items-center justify-center rounded-sm text-small text-muted-foreground transition-colors has-checked:bg-primary-soft has-checked:font-semibold has-checked:text-primary has-focus-visible:ring-2 has-focus-visible:ring-ring motion-reduce:transition-none'

// useDayNames are the days of the week, short, in the app's language,
// Monday first.
function useDayNames(): string[] {
  const locale = useLocale()
  const format = new Intl.DateTimeFormat(locale, { weekday: 'short', timeZone: 'UTC' })
  // 2026-10-26 is a Monday.
  return week.map((_, i) => format.format(Date.UTC(2026, 9, 26 + i)))
}

// WeekSegments picks the days of the week, as a segmented control of
// seven columns: one (radios, value a day) or any of them (checkboxes,
// value the days). A fieldset whose legend names it holds them, so a
// screen reader says them as one group; children follow the days (a
// hint, which describedBy names).
export function WeekSegments(
  props: {
    legend: ReactNode
    hideLegend?: boolean
    describedBy?: string
    className?: string
    children?: ReactNode
  } & (
    | { multiple?: false; value: Weekday; onChange: (day: Weekday) => void }
    | { multiple: true; value: readonly Weekday[]; onChange: (days: Weekday[]) => void }
  ),
) {
  const id = useId()
  const names = useDayNames()
  const chosen = (d: Weekday) =>
    props.multiple === true ? props.value.includes(d) : props.value === d

  return (
    <fieldset
      className={cn('min-w-0 space-y-2', props.className)}
      aria-describedby={props.describedBy}
    >
      <legend className={props.hideLegend === true ? 'sr-only' : 'font-medium'}>
        {props.legend}
      </legend>
      <div className={rowClass}>
        {week.map((d, i) => (
          <label key={d} className={dayClass}>
            <input
              type={props.multiple === true ? 'checkbox' : 'radio'}
              name={`${id}-day`}
              className="sr-only"
              checked={chosen(d)}
              onChange={() => {
                if (props.multiple === true) {
                  props.onChange(
                    week.filter((w) =>
                      w === d ? !props.value.includes(w) : props.value.includes(w),
                    ),
                  )
                } else {
                  props.onChange(d)
                }
              }}
            />
            <span className="truncate">{names[i]}</span>
          </label>
        ))}
      </div>
      {props.children}
    </fieldset>
  )
}
