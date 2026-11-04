import { RecentChangesList } from '@/components/recent-changes'
import { SubPage } from '@/components/sub-page'
import { useMessages } from '@/i18n/messages'

// ChangesPage is the user's latest changes, by whom, with Undo on the
// one undo takes back next: the settings' page of them (the toast of an
// undo opens them in a sheet).
export function ChangesPage() {
  const t = useMessages()

  return (
    <SubPage title={t.changes.title} from="settings" hint={t.changes.summary}>
      <div className="rounded-lg border bg-card p-4 lg:px-5">
        <RecentChangesList />
      </div>
    </SubPage>
  )
}
