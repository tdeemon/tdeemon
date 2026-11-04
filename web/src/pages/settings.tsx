import { Navigate, Outlet, useChildMatches } from '@tanstack/react-router'
import { useCallback, useId, useRef } from 'react'

import { AccountCard } from '@/components/settings/account-card'
import { SettingsFoot } from '@/components/settings/settings-foot'
import { SettingsList } from '@/components/settings/settings-list'
import { useMessages } from '@/i18n/messages'
import { FocusTitle } from '@/lib/focus-title'
import { useDesktop } from '@/lib/media'
import { cn } from '@/lib/utils'

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
  // A row of the list beside the page opens another: its title takes the
  // focus (SubPage), so the keyboard and a screen reader are where the
  // page is; the desktop's first page, opened of itself, leaves it be.
  const rowClicked = useRef(false)
  const focusTitle = useCallback(() => {
    const clicked = rowClicked.current
    rowClicked.current = false
    return clicked
  }, [])

  // One tree at any width, the page open in the same place in it, so a
  // window that crosses a desktop's width keeps the page as it was (an
  // access key shown once, a draft typed): the list and the name come
  // beside it, or go.
  return (
    <FocusTitle value={focusTitle}>
      <section aria-labelledby={desktop ? id : undefined} className={cn(desktop && 'space-y-6')}>
        {/* The page open beside the list has the heading of the first
          level, so its sections are of the second, as on a phone. */}
        {desktop && (
          <p id={id} className="text-title">
            {t.settings.title}
          </p>
        )}
        <div
          className={cn(
            desktop && 'grid grid-cols-[minmax(0,22rem)_minmax(0,1fr)] items-start gap-8',
          )}
        >
          {/* The list stays in sight as the page beside it scrolls. */}
          {desktop && (
            <div
              className="sticky top-8 space-y-4"
              onClickCapture={() => {
                rowClicked.current = true
              }}
            >
              <SettingsList beside />
              <SettingsFoot />
            </div>
          )}
          <div className={cn(desktop && 'max-w-2xl min-w-0')}>
            {/* A desktop has a page open beside the list: the first's. */}
            {desktop && !open ? <Navigate to="/settings/appearance" replace /> : <Outlet />}
          </div>
        </div>
      </section>
    </FocusTitle>
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
