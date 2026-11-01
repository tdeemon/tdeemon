import { useEffect, useId, useRef, useState, type RefObject, type SubmitEvent } from 'react'

import { $api } from '@/api/client'
import { statusOf } from '@/api/errors'
import { CopyField } from '@/components/copy-field'
import { Button } from '@/components/ui/button'
import type { Messages } from '@/i18n/en'
import { useMessages } from '@/i18n/messages'
import { problem } from '@/i18n/problem'
import { fieldLabel, inputClass } from '@/lib/styles'
import { cn } from '@/lib/utils'

type Role = 'user' | 'admin'

// why says why an invite was not made.
function why(t: Messages, error: unknown): string {
  const status = statusOf(error)
  if (status === 409) {
    return t.users.taken
  }
  return status === 422 ? t.users.badEmail : t.users.failed(problem(t, error))
}

// InviteForm invites someone, by the address they will sign in with and
// their role, in the sheet it is in (made anew each time it opens). The
// link the server answers is shown once, to copy, and takes the focus.
// email is the field the sheet focuses as it opens.
export function InviteForm({
  email,
  onCreated,
  onDone,
}: {
  email: RefObject<HTMLInputElement | null>
  onCreated: () => void
  onDone: () => void
}) {
  const t = useMessages()
  const id = useId()
  const copy = useRef<HTMLButtonElement>(null)
  const [address, setAddress] = useState('')
  const [role, setRole] = useState<Role>('user')
  const create = $api.useMutation('post', '/api/v1/invites', { onSuccess: onCreated })
  const made = create.data
  useEffect(() => {
    if (made) {
      copy.current?.focus()
    }
  }, [made])

  if (made) {
    return (
      <div className="space-y-4">
        <CopyField label={t.users.link} value={made.url} buttonRef={copy} />
        <p className="text-small text-muted-foreground">{t.users.linkHint}</p>
        <Button className="w-full" onClick={onDone}>
          {t.settings.done}
        </Button>
      </div>
    )
  }

  function submit(event: SubmitEvent<HTMLFormElement>) {
    event.preventDefault()
    const typed = address.trim()
    if (typed !== '') {
      create.mutate({ body: { email: typed, role } })
    }
  }

  return (
    <form onSubmit={submit} className="space-y-4">
      <label className="block space-y-1">
        <span className={cn(fieldLabel, 'block')}>{t.users.email}</span>
        <input
          ref={email}
          type="email"
          value={address}
          required
          autoComplete="off"
          aria-invalid={create.isError || undefined}
          aria-describedby={create.isError ? `${id}-error` : undefined}
          className={inputClass}
          onChange={(e) => {
            setAddress(e.currentTarget.value)
          }}
        />
      </label>
      <label className="block space-y-1">
        <span className={cn(fieldLabel, 'block')}>{t.users.role}</span>
        <select
          value={role}
          className={inputClass}
          onChange={(e) => {
            setRole(e.currentTarget.value === 'admin' ? 'admin' : 'user')
          }}
        >
          <option value="user">{t.users.roles.user}</option>
          <option value="admin">{t.users.roles.admin}</option>
        </select>
      </label>
      {create.isError && (
        <p id={`${id}-error`} role="alert" className="text-sm text-destructive">
          {why(t, create.error)}
        </p>
      )}
      <Button type="submit" className="w-full" disabled={create.isPending}>
        {create.isPending ? t.users.making : t.users.makeLink}
      </Button>
    </form>
  )
}
