import { useId, type ReactNode } from 'react'

import { Field } from '@/components/day/field'
import type { Scale } from '@/i18n/en'
import { useMessages } from '@/i18n/messages'
import { cn } from '@/lib/utils'

// A chosen step's colours: the brand's green for the mood (up is good),
// the warning's amber for hunger (a signal, not a fault: in a deficit
// hunger is expected).
const tones = {
  good: 'aria-pressed:bg-primary-soft aria-pressed:text-primary',
  signal: 'aria-pressed:bg-warning-soft aria-pressed:text-warning',
}

// Stepper picks a value of a check-in's scale, 1 to 5, in one tap: a
// segmented row of the steps' numbers, each named with its word ("4
// good"), and under it the step chosen and its word, or until one is,
// what the scale's ends mean. Tapping the chosen step again clears it
// (0).
export function Stepper({
  label,
  words,
  value,
  tone,
  status,
  onPick,
}: {
  label: string
  words: Scale
  value: number | undefined
  tone: keyof typeof tones
  // status is where the last pick stands, next to the label.
  status: ReactNode
  onPick: (value: number) => void
}) {
  const t = useMessages()
  const id = useId()
  const chosen = value === undefined || value === 0 ? undefined : words[value - 1]

  return (
    <Field label={label} labelId={id} status={status}>
      <fieldset
        aria-labelledby={id}
        className="grid min-w-0 grid-cols-5 gap-1 rounded-md bg-surface-2 p-1"
      >
        {words.map((word, i) => {
          const step = i + 1
          return (
            <button
              key={step}
              type="button"
              aria-pressed={value === step}
              onClick={() => {
                onPick(value === step ? 0 : step)
              }}
              className={cn(
                'min-h-11 min-w-0 rounded-sm text-small font-medium text-muted-foreground transition-colors outline-none hover:text-foreground focus-visible:ring-2 focus-visible:ring-ring focus-visible:ring-offset-1 focus-visible:ring-offset-surface-2 aria-pressed:font-semibold aria-pressed:shadow-card motion-reduce:transition-none',
                tones[tone],
              )}
            >
              {/* Read out as "4 good", the space outside. */}
              {step} <span className="sr-only">{word}</span>
            </button>
          )
        })}
      </fieldset>
      <p aria-hidden className="text-caption text-muted-foreground">
        {value !== undefined && chosen !== undefined
          ? t.day.scaleChosen(value, chosen)
          : t.day.scaleEnds(words[0], words[4])}
      </p>
    </Field>
  )
}
