import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { useNavigate } from '@tanstack/react-router'
import { Bell, Globe, History, House, KeyRound, LogOut, Palette, Users } from 'lucide-react'

import { $api, fetchClient } from '@/api/client'
import { HttpError } from '@/api/errors'
import { householdQuery, type Household } from '@/api/household'
import { pushDeviceQuery } from '@/api/push'
import { sessionQuery } from '@/api/session'
import { ListGroup } from '@/components/list-group'
import { ListRow } from '@/components/list-row'
import type { Messages } from '@/i18n/en'
import { locales } from '@/i18n/locale'
import { useMessages } from '@/i18n/messages'
import { leave } from '@/lib/push'
import { useTheme } from '@/lib/theme'

// householdSummary says the household in short: its other members, an
// invite waiting for the user's answer, or what one is for.
function householdSummary(s: Messages['settings'], household: Household): string {
  const others = household.members.filter((m) => m.you !== true).map((m) => m.name)
  if (others.length > 0) {
    return s.householdWith(others.join(', '))
  }
  return household.received.length > 0 ? s.householdInvited : s.householdNone
}

// SettingsList is the user's settings, each a row to its page with its
// value in short: this browser's theme, the account's language, time zone
// and day's start, the devices and AI clients with a token, the
// notifications, the household; for the admin, the users; the recent
// changes; and last signing out, as whom, a button with no chevron.
// Beside the page open of them (a desktop's, beside), a row has no
// chevron, and the open one is marked. The settings' page is its title.
export function SettingsList({ beside = false }: { beside?: boolean }) {
  const t = useMessages()
  const theme = useTheme()
  const session = useQuery(sessionQuery)
  const user = session.data?.user
  const tokens = $api.useQuery('get', '/api/v1/tokens')
  const notified = $api.useQuery('get', '/api/v1/push/subscriptions')
  const household = useQuery(householdQuery)
  const queryClient = useQueryClient()
  const navigate = useNavigate()
  const signOut = useMutation({
    // This device's notifications end first, in the session that ends.
    mutationFn: async () => {
      await leave(queryClient.getQueryData(pushDeviceQuery.queryKey))
      const { response } = await fetchClient.POST('/api/v1/auth/logout')
      if (!response.ok) {
        throw new HttpError(response.status)
      }
    },
    // Nothing of this user stays in the app for whoever signs in next.
    onSuccess: async () => {
      queryClient.clear()
      await navigate({ to: '/login', replace: true })
    },
  })
  const s = t.settings
  const chevron = !beside

  return (
    <ListGroup
      foot={
        signOut.isError && (
          <p role="alert" className="px-1 text-small text-destructive">
            {s.signOutFailed}
          </p>
        )
      }
    >
      <ListRow
        icon={Palette}
        title={s.appearance}
        to="/settings/appearance"
        chevron={chevron}
        summary={s.themes[theme]}
      />
      <ListRow
        icon={Globe}
        title={s.account}
        to="/settings/account"
        chevron={chevron}
        summary={
          user &&
          s.accountSummary(
            locales.find((l) => l.locale === user.locale)?.name ?? user.locale,
            user.timezone,
            `${String(user.day_start_hour).padStart(2, '0')}:00`,
          )
        }
      />
      <ListRow
        icon={KeyRound}
        title={s.devices}
        to="/settings/devices"
        chevron={chevron}
        summary={tokens.data && s.devicesSummary(tokens.data.tokens.length)}
      />
      <ListRow
        icon={Bell}
        title={s.notifications.title}
        to="/settings/notifications"
        chevron={chevron}
        summary={notified.data && s.notifications.summary(notified.data.subscriptions.length)}
      />
      <ListRow
        icon={House}
        title={s.household}
        to="/settings/household"
        chevron={chevron}
        summary={household.data && householdSummary(s, household.data)}
      />
      {user?.role === 'admin' && (
        <ListRow
          icon={Users}
          title={s.users}
          to="/settings/users"
          chevron={chevron}
          summary={s.usersSummary}
        />
      )}
      <ListRow
        icon={History}
        title={t.changes.title}
        to="/settings/changes"
        chevron={chevron}
        summary={t.changes.summary}
      />
      <ListRow
        icon={LogOut}
        title={s.signOut}
        summary={user?.email}
        disabled={signOut.isPending}
        onClick={() => {
          signOut.mutate()
        }}
      />
    </ListGroup>
  )
}
