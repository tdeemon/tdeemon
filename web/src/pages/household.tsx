import { useQuery } from '@tanstack/react-query'
import { House, UserPlus } from 'lucide-react'
import { useRef, useState } from 'react'

import { householdQuery } from '@/api/household'
import { sessionQuery } from '@/api/session'
import { EmptyState } from '@/components/empty-state'
import { InviteForm } from '@/components/household/invite-form'
import { Invites } from '@/components/household/invites'
import { Members } from '@/components/household/members'
import { LoadError } from '@/components/load-error'
import { Sheet } from '@/components/sheet'
import { Skeleton } from '@/components/skeleton'
import { SubPage } from '@/components/sub-page'
import { Button } from '@/components/ui/button'
import { useMessages } from '@/i18n/messages'

// HouseholdPage is the user's household (story 408): the invites for
// their answer first, then its members and the way out, or that they are
// in none; the invites it made, and someone invited in a sheet. A
// household's notification opens it.
export function HouseholdPage() {
  const t = useMessages()
  const session = useQuery(sessionQuery)
  const household = useQuery(householdQuery)
  const opener = useRef<HTMLButtonElement>(null)
  const email = useRef<HTMLInputElement>(null)
  const [inviting, setInviting] = useState(false)
  const timeZone = session.data?.user.timezone ?? 'UTC'

  let body
  if (household.isPending) {
    body = <Skeleton blocks={['h-24', 'h-10']} />
  } else if (household.data === undefined) {
    body = (
      <LoadError
        title={t.loadFailed.household}
        error={household.error}
        onRetry={() => void household.refetch()}
      />
    )
  } else {
    const { members, sent, received } = household.data
    body = (
      <>
        <Invites kind="received" invites={received} timeZone={timeZone} fallback={opener} />
        {members.length > 0 ? (
          <Members members={members} fallback={opener} />
        ) : (
          <EmptyState icon={House}>{t.household.none}</EmptyState>
        )}
        <Button
          ref={opener}
          onClick={() => {
            setInviting(true)
          }}
        >
          <UserPlus aria-hidden />
          {t.household.invite}
        </Button>
        <Invites kind="sent" invites={sent} timeZone={timeZone} fallback={opener} />
      </>
    )
  }

  return (
    <SubPage title={t.settings.household} from="settings" hint={t.household.hint}>
      {body}
      <Sheet
        open={inviting}
        onOpenChange={setInviting}
        title={t.household.invite}
        initialFocus={email}
        finalFocus={opener}
      >
        <InviteForm
          email={email}
          onDone={() => {
            setInviting(false)
          }}
        />
      </Sheet>
    </SubPage>
  )
}
