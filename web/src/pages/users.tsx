import { useQuery, useQueryClient } from '@tanstack/react-query'
import { UserPlus } from 'lucide-react'
import { useRef, useState } from 'react'

import { sessionQuery } from '@/api/session'
import { Sheet } from '@/components/sheet'
import { SubPage } from '@/components/sub-page'
import { Button } from '@/components/ui/button'
import { Accounts } from '@/components/users/accounts'
import { InviteForm } from '@/components/users/invite-form'
import { Invites } from '@/components/users/invites'
import { useMessages } from '@/i18n/messages'

// UsersPage is the admin's: someone invited, in a sheet that shows the
// link once; the invites no account took yet; and the accounts, each
// deactivated or activated, made an admin or a user, or deleted. Anyone
// else is told only the admin manages users.
export function UsersPage() {
  const t = useMessages()
  const queryClient = useQueryClient()
  const session = useQuery(sessionQuery)
  const user = session.data?.user
  const opener = useRef<HTMLButtonElement>(null)
  const email = useRef<HTMLInputElement>(null)
  const [inviting, setInviting] = useState(false)

  return (
    <SubPage title={t.settings.users} from="settings" hint={t.users.hint}>
      {/* The sections have the session before they show. */}
      {user?.role === 'admin' ? (
        <>
          <Button
            ref={opener}
            onClick={() => {
              setInviting(true)
            }}
          >
            <UserPlus aria-hidden />
            {t.users.invite}
          </Button>
          <Invites timeZone={user.timezone} />
          <Accounts me={user.id} />
          <Sheet
            open={inviting}
            onOpenChange={setInviting}
            title={t.users.invite}
            initialFocus={email}
            finalFocus={opener}
          >
            <InviteForm
              email={email}
              onCreated={() => {
                void queryClient.invalidateQueries({ queryKey: ['get', '/api/v1/invites'] })
              }}
              onDone={() => {
                setInviting(false)
              }}
            />
          </Sheet>
        </>
      ) : (
        user && <p className="rounded-lg border bg-card p-4">{t.users.notAdmin}</p>
      )}
    </SubPage>
  )
}
