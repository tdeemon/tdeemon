import { LogOut } from 'lucide-react'
import { useRef, useState, type RefObject } from 'react'
import { flushSync } from 'react-dom'

import { focusLater, useLeave, type Household } from '@/api/household'
import { Ask } from '@/components/ask'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { useMessages } from '@/i18n/messages'
import { problem } from '@/i18n/problem'
import { sayChange } from '@/lib/toast'

// Members are the household's members, by name and address, the user
// marked, and the way out, after it asks what everyone keeps. No member
// removes another. Left, the focus goes to fallback.
export function Members({
  members,
  fallback,
}: {
  members: Household['members']
  fallback: RefObject<HTMLElement | null>
}) {
  const t = useMessages()
  const h = t.household
  const opener = useRef<HTMLButtonElement>(null)
  const [asking, setAsking] = useState(false)
  const leave = useLeave()

  // out leaves the household. It waits for the answer itself, not by the
  // call's callbacks, which go with the members once it is left.
  async function out() {
    try {
      await leave.mutateAsync({})
    } catch {
      // Said below.
      return
    }
    setAsking(false)
    sayChange({ title: h.left })
    focusLater(fallback)
  }

  return (
    <section aria-labelledby="household-members" className="space-y-3">
      <h2 id="household-members" className="text-h3">
        {h.members}
      </h2>
      <ul className="divide-y rounded-lg border bg-card shadow-card">
        {members.map((m) => (
          <li key={m.email} className="flex flex-wrap items-center gap-x-3 gap-y-1 px-4 py-3">
            <div className="min-w-0 flex-1">
              <p className="font-semibold break-words">{m.name}</p>
              <p className="text-small break-words text-muted-foreground">{m.email}</p>
            </div>
            {m.you === true && <Badge tone="primary">{h.you}</Badge>}
          </li>
        ))}
      </ul>
      {asking ? (
        <Ask
          question={h.confirmLeave}
          yes={h.yesLeave}
          pending={leave.isPending}
          onYes={() => void out()}
          onKeep={() => {
            flushSync(() => {
              setAsking(false)
            })
            opener.current?.focus()
          }}
        />
      ) : (
        <Button
          ref={opener}
          variant="outline"
          onClick={() => {
            leave.reset()
            setAsking(true)
          }}
        >
          <LogOut aria-hidden />
          {h.leave}
        </Button>
      )}
      {leave.isError && (
        <p role="alert" className="text-sm text-destructive">
          {h.failed(problem(t, leave.error))}
        </p>
      )}
    </section>
  )
}
