import { useQuery } from '@tanstack/react-query'

import { sessionQuery } from '@/api/session'
import { UserMark } from '@/components/settings-avatar'
import { Badge } from '@/components/ui/badge'
import { useMessages } from '@/i18n/messages'

// AccountCard is whose settings they are, over their list on a phone:
// the user's mark, name and address, and that the server is theirs (the
// data stay on it).
export function AccountCard() {
  const t = useMessages()
  const user = useQuery(sessionQuery).data?.user

  return (
    <div className="flex items-center gap-3 rounded-lg border bg-card p-4 shadow-card">
      <UserMark className="size-12 text-base" />
      <div className="min-w-0 flex-1">
        <p className="truncate text-h3">{user?.name}</p>
        <p className="truncate text-small text-muted-foreground">{user?.email}</p>
      </div>
      <Badge tone="primary">{t.settings.yourServer}</Badge>
    </div>
  )
}
