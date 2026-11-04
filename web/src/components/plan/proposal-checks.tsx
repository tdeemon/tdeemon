import { ChevronDown, ShieldAlert } from 'lucide-react'
import { useId, useState } from 'react'

import type { components } from '@/api/schema'
import { useWarningText } from '@/components/plan/change-words'
import { useDate, useViolationText } from '@/components/plan/proposal-text'
import { useFormat } from '@/i18n/format'
import { useMessages } from '@/i18n/messages'
import { cn } from '@/lib/utils'

type Check = components['schemas']['CheckBody']
type LimitWarning = components['schemas']['LimitWarningBody']

// A Found warning is one of a check, with the day the check is of.
interface Found {
  day: string
  warning: LimitWarning
}

// ProposalChecks says what the limits found of a proposal, each rule
// once, however many types of day and periods it is of: what the
// limits would refuse, in the danger's colour, and what they warn of,
// in a row that counts them and says what they are of, the values from
// the least to the most; opened, each where it is found.
export function ProposalChecks({ checks }: { checks: readonly Check[] }) {
  const t = useMessages()
  const f = useFormat()
  const date = useDate()
  const id = useId()
  const violation = useViolationText()
  const warning = useWarningText()
  const [open, setOpen] = useState(false)
  const w = t.plan.checks

  const refused = [...new Set(checks.flatMap((ck) => ck.violations.map(violation)))]
  const byRule = new Map<LimitWarning['rule'], Found[]>()
  for (const ck of checks) {
    for (const found of ck.warnings) {
      byRule.set(found.rule, [...(byRule.get(found.rule) ?? []), { day: ck.day, warning: found }])
    }
  }
  const rules = [...byRule.entries()]
  // range is the values from the least to the most, one when they are
  // the same as shown.
  const range = (values: number[]) => {
    const low = f.number(Math.min(...values))
    const high = f.number(Math.max(...values))
    return low === high ? low : `${low}–${high}`
  }
  // scope says what a rule's warnings are of: which types of day, and
  // how many periods.
  const scope = (found: Found[]) => {
    const types = new Set(found.map((x) => x.warning.day_type ?? 'every'))
    const periods = new Set(found.map((x) => x.day)).size
    const of = types.size > 1 ? w.bothTypes : w.types[[...types][0] ?? 'every']
    return periods > 1 ? `${of}, ${w.periods(periods)}` : of
  }

  if (refused.length === 0 && rules.length === 0) {
    return null
  }
  return (
    <div className="space-y-2">
      {refused.length > 0 && (
        <div className="rounded-md bg-destructive/10 px-3 py-2.5 text-small text-destructive">
          <p className="font-semibold">{w.refused}</p>
          <ul>
            {refused.map((line) => (
              <li key={line}>{line}</li>
            ))}
          </ul>
        </div>
      )}
      {rules.length > 0 && (
        <div className="rounded-md bg-warning-soft text-small">
          <button
            type="button"
            aria-expanded={open}
            aria-controls={`${id}-each`}
            onClick={() => {
              setOpen((was) => !was)
            }}
            className="flex min-h-11 w-full items-start gap-2.5 rounded-md px-3 py-2.5 text-left outline-none focus-visible:ring-2 focus-visible:ring-ring"
          >
            <ShieldAlert aria-hidden className="mt-0.5 size-4 shrink-0 text-warning" />
            <span className="min-w-0 flex-1 space-y-0.5">
              <span className="block">
                <b className="font-semibold text-warning">{w.count(rules.length)}</b>
                {rules.length === 1 && rules[0] !== undefined && (
                  <span className="text-muted-foreground"> · {scope(rules[0][1])}</span>
                )}
              </span>
              {rules.map(([rule, found]) => (
                <span
                  key={rule}
                  className={cn('block font-normal text-muted-foreground', !open && 'line-clamp-2')}
                >
                  {t.plan.warnings[rule](
                    range(found.map((x) => x.warning.value)),
                    range(found.map((x) => x.warning.threshold)),
                  )}
                  {rules.length > 1 && ` (${scope(found)})`}
                </span>
              ))}
            </span>
            <ChevronDown
              aria-hidden
              className={cn(
                'mt-0.5 size-4 shrink-0 text-muted-foreground transition-transform motion-reduce:transition-none',
                open && 'rotate-180',
              )}
            />
          </button>
          {open && (
            <ul id={`${id}-each`} className="space-y-1 border-t border-warning/20 px-3 py-2.5">
              {rules.flatMap(([rule, found]) =>
                found.map((x) => (
                  <li key={`${rule} ${x.day} ${x.warning.day_type ?? ''}`}>
                    <span className="text-muted-foreground">{w.from(date(x.day))} </span>
                    {warning(x.warning)}
                  </li>
                )),
              )}
            </ul>
          )}
        </div>
      )}
    </div>
  )
}
