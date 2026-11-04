import { useQueryClient } from '@tanstack/react-query'
import { MailOpen } from 'lucide-react'
import { useRef, useState } from 'react'
import { flushSync } from 'react-dom'

import { $api } from '@/api/client'
import { statusOf } from '@/api/errors'
import { Ask } from '@/components/ask'
import { EmptyState } from '@/components/empty-state'
import { LoadError } from '@/components/load-error'
import { Skeleton } from '@/components/skeleton'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { useFormat } from '@/i18n/format'
import { useMessages } from '@/i18n/messages'
import { problem } from '@/i18n/problem'

const invitesKey = ['get', '/api/v1/invites']

// expired is whether an invite's link works no more.
function expired(at: string): boolean {
  return new Date(at).getTime() <= Date.now()
}

// Invites are the admin's invites no account took yet, the newest first:
// the address, the role, and until when the link works, or that it
// expired. One is revoked after it asks: the focus goes to Keep, and
// back to what asked, or to the heading once the invite is gone.
export function Invites({ timeZone }: { timeZone: string }) {
  const t = useMessages()
  const f = useFormat()
  const queryClient = useQueryClient()
  const list = $api.useQuery('get', '/api/v1/invites')
  const heading = useRef<HTMLHeadingElement>(null)
  const openers = useRef(new Map<string, HTMLButtonElement>())
  const [asking, setAsking] = useState<string | undefined>(undefined)
  const gone = async () => {
    setAsking(undefined)
    heading.current?.focus()
    await queryClient.invalidateQueries({ queryKey: invitesKey })
  }
  const revoke = $api.useMutation('delete', '/api/v1/invites/{id}', {
    onSuccess: gone,
    // Revoked elsewhere, or taken: it is gone all the same.
    onError: async (error) => {
      if (statusOf(error) === 404) {
        await gone()
      }
    },
  })

  let body
  if (list.isPending) {
    body = <Skeleton blocks={['h-16']} />
  } else if (list.data === undefined) {
    body = (
      <LoadError
        title={t.loadFailed.invites}
        error={list.error}
        onRetry={() => void list.refetch()}
      />
    )
  } else if (list.data.invites.length === 0) {
    body = <EmptyState icon={MailOpen}>{t.users.noInvites}</EmptyState>
  } else {
    body = (
      <ul className="space-y-3">
        {list.data.invites.map((inv) => (
          <li key={inv.id} className="space-y-3 rounded-lg border bg-card p-4 shadow-card">
            <div className="flex items-start gap-3">
              <div className="min-w-0 flex-1 space-y-1">
                <h3 className="truncate font-semibold">{inv.email}</h3>
                <p className="flex flex-wrap items-center gap-1.5 text-small text-muted-foreground">
                  <Badge tone={inv.role === 'admin' ? 'primary' : 'neutral'}>
                    {t.users.roles[inv.role]}
                  </Badge>
                  {expired(inv.expires_at) ? (
                    <Badge tone="warning">{t.users.expired}</Badge>
                  ) : (
                    <span>{t.users.worksUntil(f.day(inv.expires_at, timeZone))}</span>
                  )}
                </p>
              </div>
              {asking === inv.id ? null : (
                <Button
                  ref={(el) => {
                    if (el) {
                      openers.current.set(inv.id, el)
                    }
                    return () => {
                      openers.current.delete(inv.id)
                    }
                  }}
                  variant="ghost"
                  size="sm"
                  aria-label={t.users.revoke(inv.email)}
                  onClick={() => {
                    revoke.reset()
                    setAsking(inv.id)
                  }}
                >
                  {t.users.yesRevoke}
                </Button>
              )}
            </div>
            {asking === inv.id && (
              <Ask
                question={t.users.confirmRevoke(inv.email)}
                yes={t.users.yesRevoke}
                pending={revoke.isPending}
                onYes={() => {
                  revoke.mutate({ params: { path: { id: inv.id } } })
                }}
                onKeep={() => {
                  flushSync(() => {
                    setAsking(undefined)
                  })
                  ;(openers.current.get(inv.id) ?? heading.current)?.focus()
                }}
              />
            )}
          </li>
        ))}
      </ul>
    )
  }

  return (
    <section aria-labelledby="users-invites" className="space-y-3">
      <h2 id="users-invites" ref={heading} tabIndex={-1} className="text-h3 outline-none">
        {t.users.invites}
      </h2>
      {body}
      {revoke.isError && statusOf(revoke.error) !== 404 && (
        <p role="alert" className="text-sm text-destructive">
          {t.users.failed(problem(t, revoke.error))}
        </p>
      )}
    </section>
  )
}
