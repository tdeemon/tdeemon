import { useId, useState, type RefObject, type SubmitEvent } from 'react'

import { statusOf } from '@/api/errors'
import { codeOf, useInvite } from '@/api/household'
import { Button } from '@/components/ui/button'
import type { Messages } from '@/i18n/en'
import { useMessages } from '@/i18n/messages'
import { problem } from '@/i18n/problem'
import { fieldLabel, inputClass } from '@/lib/styles'
import { sayChange } from '@/lib/toast'
import { cn } from '@/lib/utils'

// why says why an invite was not made: by the refusal's code, too many
// today, or what failed.
function why(t: Messages, error: unknown): string {
  if (statusOf(error) === 429) {
    return t.household.tooMany
  }
  const code = codeOf(error, 'body.email')
  const refusals: Record<string, string> = t.household.refusals
  return (code !== undefined ? refusals[code] : undefined) ?? t.household.failed(problem(t, error))
}

// InviteForm invites an address into the household (whoever has an
// account of it answers), in the sheet it is in; done, a toast says so and the sheet
// closes. email is the field the sheet focuses as it opens.
export function InviteForm({
  email,
  onDone,
}: {
  email: RefObject<HTMLInputElement | null>
  onDone: () => void
}) {
  const t = useMessages()
  const id = useId()
  const [address, setAddress] = useState('')
  // Said by the hook, so a sheet closed before the answer says it too.
  const invite = useInvite((invited) => {
    sayChange({ title: t.household.invited(invited) })
  })

  function submit(event: SubmitEvent<HTMLFormElement>) {
    event.preventDefault()
    const typed = address.trim()
    if (typed === '') {
      return
    }
    invite.mutate({ body: { email: typed } }, { onSuccess: onDone })
  }

  return (
    <form onSubmit={submit} className="space-y-4">
      <label className="block space-y-1">
        <span className={cn(fieldLabel, 'block')}>{t.household.email}</span>
        <input
          ref={email}
          // Text, not email: a browser refuses an address of letters such as
          // č, which the server takes.
          type="text"
          inputMode="email"
          autoCapitalize="none"
          spellCheck={false}
          value={address}
          required
          autoComplete="off"
          aria-invalid={invite.isError || undefined}
          aria-describedby={invite.isError ? `${id}-error` : `${id}-hint`}
          className={inputClass}
          onChange={(e) => {
            setAddress(e.currentTarget.value)
          }}
        />
      </label>
      {invite.isError ? (
        <p id={`${id}-error`} role="alert" className="text-sm text-destructive">
          {why(t, invite.error)}
        </p>
      ) : (
        <p id={`${id}-hint`} className="text-small text-muted-foreground">
          {t.household.inviteHint}
        </p>
      )}
      <Button type="submit" className="w-full" disabled={invite.isPending}>
        {invite.isPending ? t.household.sending : t.household.send}
      </Button>
    </form>
  )
}
