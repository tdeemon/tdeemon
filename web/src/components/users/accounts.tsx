import { useQueryClient } from '@tanstack/react-query'
import { useRef, useState } from 'react'
import { flushSync } from 'react-dom'

import { $api } from '@/api/client'
import { statusOf } from '@/api/errors'
import { Ask } from '@/components/ask'
import { LoadError } from '@/components/load-error'
import { Skeleton } from '@/components/skeleton'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import type { Messages } from '@/i18n/en'
import { useMessages } from '@/i18n/messages'
import { problem } from '@/i18n/problem'

const usersKey = ['get', '/api/v1/users']

// why says why a user's account was not changed.
function why(t: Messages, error: unknown): string {
  return statusOf(error) === 409 ? t.users.onlyAdmin : t.users.failed(problem(t, error))
}

// Accounts are the instance's users, for its admin, a card each: the
// name, the address, the role, and whether the account is deactivated.
// Each but the admin's own is deactivated or activated, made an admin or
// a user, or deleted with all its data, after it asks: the focus goes to
// Keep, and back to Delete, or to the heading once the user is gone. A
// change the server refuses, the only active admin's, says so on its
// card.
export function Accounts({ me }: { me: string }) {
  const t = useMessages()
  const queryClient = useQueryClient()
  const list = $api.useQuery('get', '/api/v1/users')
  const heading = useRef<HTMLHeadingElement>(null)
  const openers = useRef(new Map<string, HTMLButtonElement>())
  const [asking, setAsking] = useState<string | undefined>(undefined)
  const asked = () => queryClient.invalidateQueries({ queryKey: usersKey })
  const change = $api.useMutation('patch', '/api/v1/users/{id}', { onSuccess: asked })
  const remove = $api.useMutation('delete', '/api/v1/users/{id}', {
    onSuccess: async () => {
      setAsking(undefined)
      heading.current?.focus()
      await asked()
    },
  })
  // The user a failed change was of, and why it failed.
  const failed = change.isError
    ? { id: change.variables.params.path.id, error: change.error }
    : remove.isError
      ? { id: remove.variables.params.path.id, error: remove.error }
      : undefined
  const reset = () => {
    change.reset()
    remove.reset()
  }

  let body
  if (list.isPending) {
    body = <Skeleton blocks={['h-24', 'h-24']} />
  } else if (list.data === undefined) {
    body = (
      <LoadError
        title={t.loadFailed.users}
        error={list.error}
        onRetry={() => void list.refetch()}
      />
    )
  } else {
    body = (
      <ul className="space-y-3">
        {list.data.users.map((u) => {
          const yours = u.id === me
          const active = u.disabled_at === undefined
          return (
            <li key={u.id} className="space-y-3 rounded-lg border bg-card p-4 shadow-card">
              <div className="min-w-0 space-y-1">
                <h3 className="truncate font-semibold">{u.name}</h3>
                <p className="truncate text-small text-muted-foreground">{u.email}</p>
                <p className="flex flex-wrap gap-1.5">
                  <Badge tone={u.role === 'admin' ? 'primary' : 'neutral'}>
                    {t.users.roles[u.role]}
                  </Badge>
                  {!active && <Badge tone="warning">{t.users.deactivated}</Badge>}
                  {yours && <Badge>{t.users.you}</Badge>}
                </p>
              </div>
              {!yours && asking !== u.id && (
                <div className="flex flex-wrap gap-2">
                  <Button
                    variant="outline"
                    size="sm"
                    aria-label={t.users.of(active ? t.users.deactivate : t.users.activate, u.name)}
                    disabled={change.isPending}
                    onClick={() => {
                      reset()
                      change.mutate({ params: { path: { id: u.id } }, body: { active: !active } })
                    }}
                  >
                    {active ? t.users.deactivate : t.users.activate}
                  </Button>
                  <Button
                    variant="outline"
                    size="sm"
                    aria-label={t.users.of(
                      u.role === 'admin' ? t.users.makeUser : t.users.makeAdmin,
                      u.name,
                    )}
                    disabled={change.isPending}
                    onClick={() => {
                      reset()
                      change.mutate({
                        params: { path: { id: u.id } },
                        body: { role: u.role === 'admin' ? 'user' : 'admin' },
                      })
                    }}
                  >
                    {u.role === 'admin' ? t.users.makeUser : t.users.makeAdmin}
                  </Button>
                  <Button
                    ref={(el) => {
                      if (el) {
                        openers.current.set(u.id, el)
                      }
                      return () => {
                        openers.current.delete(u.id)
                      }
                    }}
                    variant="destructive"
                    size="sm"
                    aria-label={t.users.of(t.users.delete, u.name)}
                    onClick={() => {
                      reset()
                      setAsking(u.id)
                    }}
                  >
                    {t.users.delete}
                  </Button>
                </div>
              )}
              {asking === u.id && (
                <Ask
                  question={t.users.confirmDelete(u.name)}
                  yes={t.users.yesDelete}
                  pending={remove.isPending}
                  onYes={() => {
                    remove.mutate({ params: { path: { id: u.id } } })
                  }}
                  onKeep={() => {
                    flushSync(() => {
                      setAsking(undefined)
                    })
                    ;(openers.current.get(u.id) ?? heading.current)?.focus()
                  }}
                />
              )}
              {failed?.id === u.id && (
                <p role="alert" className="text-sm text-destructive">
                  {why(t, failed.error)}
                </p>
              )}
            </li>
          )
        })}
      </ul>
    )
  }

  return (
    <section aria-labelledby="users-accounts" className="space-y-3">
      <h2 id="users-accounts" ref={heading} tabIndex={-1} className="text-h3 outline-none">
        {t.users.accounts}
      </h2>
      {body}
    </section>
  )
}
