import { Check } from 'lucide-react'
import { useId, type ReactNode } from 'react'

import { chipClass, chosenChip } from '@/lib/styles'
import { cn } from '@/lib/utils'

// ChipGroup picks one of a few options, as chips in a row: radios in a
// fieldset whose legend, for a screen reader only, names it, so it says
// them as one group and the arrow keys move between them, as in
// SegmentedControl. A chip is chipClass's (lib/styles.ts).
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
        <label key={option.value} className={cn(chipClass, chosenChip)}>
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
