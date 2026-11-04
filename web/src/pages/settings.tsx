import { Navigate, Outlet, useChildMatches } from '@tanstack/react-router'
import { useId } from 'react'

import { AccountCard } from '@/components/settings/account-card'
import { SettingsFoot } from '@/components/settings/settings-foot'
import { SettingsList } from '@/components/settings/settings-list'
import { useMessages } from '@/i18n/messages'
import { useDesktop } from '@/lib/media'

// SettingsPage is the user's settings (SettingsList), each a row to its
// page, and the page open of them: on a desktop the list at the left and
// the page beside it, under the title; on a phone one or the other. A
// desktop's sidebar and a tablet's rail lead here, and on a phone the
// user's mark in a page's header (SettingsAvatar), so a setting is two
// taps from any page.
export function SettingsPage() {
  const t = useMessages()
  const id = useId()
  const desktop = useDesktop()
  // A setting's page is open; else the list alone.
  const open = useChildMatches({
    select: (matches) => matches.some((m) => m.routeId !== '/app/settings/'),
  })

  if (!desktop) {
    return <Outlet />
  }
  return (
    <section aria-labelledby={id} className="space-y-6">
      <h1 id={id} className="text-title">
        {t.settings.title}
      </h1>
      <div className="grid grid-cols-[minmax(0,22rem)_minmax(0,1fr)] items-start gap-8">
        {/* The list stays in sight as the page beside it scrolls. */}
        <div className="sticky top-8 space-y-4">
          <SettingsList beside />
          <SettingsFoot />
        </div>
        <div className="max-w-2xl min-w-0">
          {/* A desktop has a page open beside the list: the first's. */}
          {open ? <Outlet /> : <Navigate to="/settings/appearance" replace />}
        </div>
      </div>
    </section>
  )
}

// SettingsIndexPage is the settings' list on a phone: whose they are
// (AccountCard), the rows, and the server's name and version, with a
// copy of the data.
export function SettingsIndexPage() {
  const t = useMessages()

  return (
    <section className="max-w-2xl space-y-4">
      <h1 className="text-title">{t.settings.title}</h1>
      <AccountCard />
      <SettingsList />
      <SettingsFoot />
    </section>
  )
}
