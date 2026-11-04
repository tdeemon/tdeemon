import { Switch } from '@base-ui/react/switch'
import { useQuery, useQueryClient } from '@tanstack/react-query'
import { useId } from 'react'

import { $api } from '@/api/client'
import { sessionQuery } from '@/api/session'
import type { Session } from '@/api/sign-in'
import { SaveStatus } from '@/components/day/save-status'
import { messagesOf, useMessages } from '@/i18n/messages'
import { sayChange } from '@/lib/toast'

// SingleKeyShortcuts turns the keyboard's shortcuts of one key on or off,
// as a switch: N, W, G and a letter, ? and /, and typing on the day,
// which speech input can set off (WCAG 2.1.4). It is the user's, sent at
// once and kept on every device; the session holds it as changed.
export function SingleKeyShortcuts() {
  const t = useMessages()
  const id = useId()
  const queryClient = useQueryClient()
  const session = useQuery(sessionQuery)
  const save = $api.useMutation('patch', '/api/v1/me', {
    onSuccess: (answer) => {
      queryClient.setQueryData(sessionQuery.queryKey, (s: Session | undefined) =>
        s ? { ...s, user: answer.user } : s,
      )
      sayChange({ title: messagesOf(answer.user.locale).toast.saved })
    },
  })
  const on = session.data?.user.single_key_shortcuts !== false

  return (
    <div className="space-y-2">
      <div className="flex items-start justify-between gap-4">
        <div className="space-y-1">
          <p id={`${id}-label`} className="font-medium">
            {t.settings.singleKeys}
          </p>
          <p id={`${id}-hint`} className="text-sm text-muted-foreground">
            {t.settings.singleKeysHint}
          </p>
        </div>
        <Switch.Root
          aria-labelledby={`${id}-label`}
          aria-describedby={`${id}-hint`}
          checked={on}
          disabled={session.data === undefined || save.isPending}
          onCheckedChange={(checked) => {
            save.mutate({ body: { single_key_shortcuts: checked } })
          }}
          className="relative mt-0.5 inline-flex h-7 w-12 shrink-0 items-center rounded-full border border-control-border bg-surface-2 p-px outline-none after:absolute after:-inset-2 after:content-[''] focus-visible:ring-2 focus-visible:ring-ring focus-visible:ring-offset-2 focus-visible:ring-offset-card data-checked:border-primary data-checked:bg-primary data-disabled:opacity-45"
        >
          <Switch.Thumb className="size-6 rounded-full bg-card shadow-card transition-transform duration-(--duration-base) data-checked:translate-x-5" />
        </Switch.Root>
      </div>
      <SaveStatus change={save} />
    </div>
  )
}
