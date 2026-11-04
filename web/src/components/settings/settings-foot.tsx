import { $api } from '@/api/client'
import { useMessages } from '@/i18n/messages'

// SettingsFoot is under the settings' list: the server's name and
// version, that the data and the server are the user's, and a copy of
// the data, as a ZIP (the account's page has it too, with its delete).
export function SettingsFoot() {
  const t = useMessages()
  const info = $api.useQuery('get', '/api/v1/info')

  return (
    <p className="text-center text-caption text-muted-foreground">
      {/* Without the server's word, what is said of it stands alone. */}
      {info.isPending && <span className="block">{t.connecting}</span>}
      {info.isError && <span className="block">{t.unreachable}</span>}
      {info.data && (
        <>
          <span data-testid="server-name">{info.data.name}</span>{' '}
          <span data-testid="server-version">{info.data.version}</span>
          {' · '}
        </>
      )}
      {t.settings.yourData}
      {' · '}
      <a
        href="/api/v1/account/export"
        download
        className="inline-flex min-h-11 items-center rounded-sm px-1 text-primary underline-offset-4 outline-none hover:underline focus-visible:ring-2 focus-visible:ring-ring"
      >
        {t.settings.exportZip}
      </a>
    </p>
  )
}
