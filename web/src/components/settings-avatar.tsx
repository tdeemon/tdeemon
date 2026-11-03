import { useQuery } from '@tanstack/react-query'
import { Link } from '@tanstack/react-router'

import { sessionQuery } from '@/api/session'
import { useMessages } from '@/i18n/messages'
import { cn } from '@/lib/utils'

// initials are a name's first letters, of its first two words.
function initials(name: string): string {
  return name
    .trim()
    .split(/\s+/)
    .slice(0, 2)
    .map((word) => Array.from(word)[0] ?? '')
    .join('')
    .toLocaleUpperCase()
}

// UserMark is the signed-in user's mark: their initials in a circle, for
// the eye alone.
export function UserMark({ className }: { className?: string }) {
  const session = useQuery(sessionQuery)
  return (
    <span
      aria-hidden
      className={cn(
        'flex size-9 shrink-0 items-center justify-center rounded-full bg-surface-2 text-small font-semibold text-foreground',
        className,
      )}
    >
      {initials(session.data?.user.name ?? '')}
    </span>
  )
}

// SettingsAvatar is the way to the settings and the account from a
// page's header on a phone, whose bar at the bottom has no room for
// them: the user's mark at the header's end. A tablet's rail and a
// desktop's sidebar have the settings, and it hides there.
export function SettingsAvatar({ className }: { className?: string }) {
  const t = useMessages()
  return (
    <Link
      to="/settings"
      aria-label={t.nav.account}
      className={cn(
        'flex size-11 shrink-0 items-center justify-center rounded-full outline-none focus-visible:ring-2 focus-visible:ring-ring tablet:hidden',
        className,
      )}
    >
      <UserMark />
    </Link>
  )
}
