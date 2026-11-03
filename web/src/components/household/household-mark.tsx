import { useQuery } from '@tanstack/react-query'
import { House } from 'lucide-react'

import { householdQuery } from '@/api/household'
import { useMessages } from '@/i18n/messages'

// HouseholdMark is a food's line that the user's household shares it, so
// a change reaches every member; nothing out of a household.
export function HouseholdMark() {
  const t = useMessages()
  const household = useQuery(householdQuery)
  if (household.data === undefined || household.data.members.length === 0) {
    return null
  }
  return (
    <p className="flex items-start gap-2 text-small text-muted-foreground">
      <House aria-hidden className="mt-0.5 size-4 shrink-0" />
      <span>{t.household.mark}</span>
    </p>
  )
}
