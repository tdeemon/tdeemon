import { useQuery } from '@tanstack/react-query'
import { Link } from '@tanstack/react-router'

import { sessionQuery } from '@/api/session'
import { SettingsAvatar } from '@/components/settings-avatar'
import { buttonVariants } from '@/components/ui/button'
import { useMessages } from '@/i18n/messages'
import { narrowPage } from '@/lib/styles'

// NotFoundPage is an address the app has no page of: the way home, and
// for a signed-in user, outside the sections' frame, the way to the
// settings, the user's mark, at every width.
export function NotFoundPage() {
  const t = useMessages()
  const session = useQuery(sessionQuery)

  return (
    <main className={narrowPage}>
      <header className="flex items-center justify-between gap-2">
        <h1 className="text-2xl font-semibold">{t.notFound.title}</h1>
        {session.data !== undefined && <SettingsAvatar className="tablet:flex" />}
      </header>
      <Link to="/" className={buttonVariants({ variant: 'outline' })}>
        {t.notFound.back}
      </Link>
    </main>
  )
}
