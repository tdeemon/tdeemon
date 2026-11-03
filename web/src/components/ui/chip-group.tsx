import { Check } from 'lucide-react'
import { useId, type ReactNode } from 'react'

import { cn } from '@/lib/utils'

// ChipGroup picks one of a few options, as chips in a row: radios in a
// fieldset whose legend, for a screen reader only, names it, so it says
// them as one group and the arrow keys move between them, as in
// SegmentedControl. A chip has a control's edge; the one chosen is the
// brand's soft green with a check, as a chosen segment is. A chip is 32
// px high to the eye and 44 px to a finger (an ::after over it, 6 px
// above and below).
export function ChipGroup<T extends string>({
  legend,
  options,
  value,
  onChange,
  className,
}: {
  legend: ReactNode
  options: readonly { value: T; label: ReactNode }[]
  value: T
  onChange: (value: T) => void
  className?: string
}) {
  const id = useId()

  return (
    <fieldset className={cn('flex min-w-0 flex-wrap gap-1', className)}>
      <legend className="sr-only">{legend}</legend>
      {options.map((option) => (
        <label
          key={option.value}
          className="relative flex h-8 min-w-11 cursor-pointer items-center justify-center gap-1 rounded-sm border border-control-border bg-card px-2.5 text-small whitespace-nowrap transition-colors after:absolute after:inset-x-0 after:-inset-y-1.5 after:content-[''] has-checked:border-transparent has-checked:bg-primary-soft has-checked:font-semibold has-checked:text-primary has-focus-visible:ring-2 has-focus-visible:ring-ring has-focus-visible:ring-offset-2 has-focus-visible:ring-offset-background motion-reduce:transition-none"
        >
          <input
            type="radio"
            name={`${id}-choice`}
            className="sr-only"
            checked={value === option.value}
            onChange={() => {
              onChange(option.value)
            }}
          />
          {value === option.value && <Check aria-hidden className="size-4 shrink-0" />}
          {option.label}
        </label>
      ))}
    </fieldset>
  )
}
