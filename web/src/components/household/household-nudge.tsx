import { useQuery } from '@tanstack/react-query'
import { Link } from '@tanstack/react-router'
import { House } from 'lucide-react'

import { householdQuery } from '@/api/household'
import { useMessages } from '@/i18n/messages'

// HouseholdNudge is the day's line that an invite into a household waits
// for the user's answer, the newest's, with a link to the household's
// page, where it is answered; nothing while none waits.
export function HouseholdNudge() {
  const t = useMessages()
  const household = useQuery(householdQuery)
  const invite = household.data?.received[0]
  if (invite === undefined) {
    return null
  }
  return (
    <Link
      to="/settings/household"
      className="flex min-h-11 items-center gap-2 rounded-md border border-primary/30 bg-primary-soft px-3 py-2 text-small text-primary outline-none focus-visible:ring-2 focus-visible:ring-ring"
    >
      <House aria-hidden className="size-4 shrink-0" />
      <span className="min-w-0 flex-1 truncate">{t.household.nudge(invite.by.name)}</span>
      <span className="shrink-0 font-semibold">{t.household.nudgeSee}</span>
    </Link>
  )
}
