import { Appearance } from '@/components/settings/appearance'
import { SingleKeyShortcuts } from '@/components/settings/single-key-shortcuts'
import { SubPage } from '@/components/sub-page'
import { useMessages } from '@/i18n/messages'

// AppearancePage picks this browser's theme, the system's, light or dark,
// and the user's keyboard: whether the shortcuts of one key work.
export function AppearancePage() {
  const t = useMessages()

  return (
    <SubPage title={t.settings.appearance} from="settings">
      <div className="rounded-lg border bg-card p-4 lg:p-5">
        <Appearance />
      </div>
      <div className="mt-4 rounded-lg border bg-card p-4 lg:p-5">
        <SingleKeyShortcuts />
      </div>
    </SubPage>
  )
}
