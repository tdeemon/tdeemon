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

// Asking is a change of a user asked about before it is made: each but
// an activation, which takes nothing away.
interface Asking {
  id: string
  what: 'deactivate' | 'admin' | 'user' | 'delete'
}

// slot is the button that asks a change: making an admin and a user
// share theirs.
function slot(a: Asking): string {
  return `${a.id}:${a.what === 'user' ? 'admin' : a.what}`
}

// why says why a user's account was not changed.
function why(t: Messages, error: unknown): string {
  return statusOf(error) === 409 ? t.users.onlyAdmin : t.users.failed(problem(t, error))
}

// Accounts are the instance's users, for its admin, a card each: the
// name, the address, the role, and whether the account is deactivated.
// Each but the admin's own is deactivated, made an admin or a user, or
// deleted with all its data, after it asks (an activation at once): the
// focus goes to Keep, and back to what asked, or to the heading once the
// user is gone. A
// change the server refuses, the only active admin's, says so on its
// card.
export function Accounts({ me }: { me: string }) {
  const t = useMessages()
  const queryClient = useQueryClient()
  const list = $api.useQuery('get', '/api/v1/users')
  const heading = useRef<HTMLHeadingElement>(null)
  // The buttons that ask, by the user and the change.
  const openers = useRef(new Map<string, HTMLButtonElement>())
  const [asking, setAsking] = useState<Asking | undefined>(undefined)
  const asked = () => queryClient.invalidateQueries({ queryKey: usersKey })
  // back closes the question, and gives the focus to what asked it.
  const back = (a: Asking) => {
    flushSync(() => {
      setAsking(undefined)
    })
    ;(openers.current.get(slot(a)) ?? heading.current)?.focus()
  }
  const change = $api.useMutation('patch', '/api/v1/users/{id}', {
    onSuccess: async (_, sent) => {
      const a = asking
      if (a?.id === sent.params.path.id) {
        back(a)
      }
      await asked()
    },
  })
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
  // opener is a button's ref that keeps it among the openers.
  const opener = (key: string) => (el: HTMLButtonElement | null) => {
    if (el) {
      openers.current.set(key, el)
    }
    return () => {
      openers.current.delete(key)
    }
  }
  const ask = (a: Asking) => {
    reset()
    setAsking(a)
  }
  // question is what an Ask says and answers of a change asked.
  const question = (a: Asking, name: string) =>
    ({
      deactivate: { q: t.users.confirmDeactivate(name), yes: t.users.deactivate },
      admin: { q: t.users.confirmMakeAdmin(name), yes: t.users.makeAdmin },
      user: { q: t.users.confirmMakeUser(name), yes: t.users.makeUser },
      delete: { q: t.users.confirmDelete(name), yes: t.users.yesDelete },
    })[a.what]
  const answer = (a: Asking) => {
    const path = { params: { path: { id: a.id } } }
    switch (a.what) {
      case 'deactivate':
        change.mutate({ ...path, body: { active: false } })
        break
      case 'admin':
      case 'user':
        change.mutate({ ...path, body: { role: a.what } })
        break
      case 'delete':
        remove.mutate(path)
        break
    }
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
              {!yours && asking?.id !== u.id && (
                <div className="flex flex-wrap gap-2">
                  <Button
                    ref={opener(slot({ id: u.id, what: 'deactivate' }))}
                    variant="outline"
                    size="sm"
                    aria-label={t.users.of(active ? t.users.deactivate : t.users.activate, u.name)}
                    disabled={change.isPending}
                    // Focused again as a change asked of it ends.
                    focusableWhenDisabled
                    onClick={() => {
                      if (active) {
                        ask({ id: u.id, what: 'deactivate' })
                      } else {
                        reset()
                        change.mutate({ params: { path: { id: u.id } }, body: { active: true } })
                      }
                    }}
                  >
                    {active ? t.users.deactivate : t.users.activate}
                  </Button>
                  <Button
                    ref={opener(slot({ id: u.id, what: 'admin' }))}
                    variant="outline"
                    size="sm"
                    aria-label={t.users.of(
                      u.role === 'admin' ? t.users.makeUser : t.users.makeAdmin,
                      u.name,
                    )}
                    disabled={change.isPending}
                    // Focused again as a change asked of it ends.
                    focusableWhenDisabled
                    onClick={() => {
                      ask({ id: u.id, what: u.role === 'admin' ? 'user' : 'admin' })
                    }}
                  >
                    {u.role === 'admin' ? t.users.makeUser : t.users.makeAdmin}
                  </Button>
                  <Button
                    ref={opener(slot({ id: u.id, what: 'delete' }))}
                    variant="destructive"
                    size="sm"
                    aria-label={t.users.of(t.users.delete, u.name)}
                    onClick={() => {
                      ask({ id: u.id, what: 'delete' })
                    }}
                  >
                    {t.users.delete}
                  </Button>
                </div>
              )}
              {asking?.id === u.id && (
                <Ask
                  key={asking.what}
                  question={question(asking, u.name).q}
                  yes={question(asking, u.name).yes}
                  pending={change.isPending || remove.isPending}
                  onYes={() => {
                    answer(asking)
                  }}
                  onKeep={() => {
                    back(asking)
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
