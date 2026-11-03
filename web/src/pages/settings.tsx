import { SettingsList } from '@/components/settings/settings-list'
import { useMessages } from '@/i18n/messages'

// SettingsPage is the user's settings, each a row to its page: this
// browser's theme, the account's language, time zone and day's start, the
// devices and AI clients, and signing out. A desktop's sidebar and a
// tablet's rail lead here, and on a phone the user's mark in a page's
// header (SettingsAvatar); the bar's sections lead away.
export function SettingsPage() {
  const t = useMessages()

  return (
    <section className="max-w-2xl space-y-4">
      <h1 className="text-title">{t.settings.title}</h1>
      <SettingsList />
    </section>
  )
}
