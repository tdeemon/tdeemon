import type { RefObject } from 'react'

import type { components } from '@/api/schema'
import { TargetsForm } from '@/components/plan/targets-form'
import { Sheet } from '@/components/sheet'
import { useMessages } from '@/i18n/messages'

type Plan = components['schemas']['PlanBody']

// TargetsSheet edits the targets by type of day, from today on, or,
// before the plan's first target, that one from its day, and the
// training days, in a sheet; the focus goes back to the button that
// opened it (finalFocus).
export function TargetsSheet({
  plan,
  open,
  onOpenChange,
  finalFocus,
}: {
  plan: Plan
  open: boolean
  onOpenChange: (open: boolean) => void
  finalFocus: RefObject<HTMLElement | null>
}) {
  const t = useMessages()

  return (
    <Sheet
      open={open}
      onOpenChange={onOpenChange}
      title={t.plan.edit.targetsTitle}
      finalFocus={finalFocus}
    >
      <TargetsForm
        plan={plan}
        onDone={() => {
          onOpenChange(false)
        }}
      />
    </Sheet>
  )
}
