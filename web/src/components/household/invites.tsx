import { useRef, useState, type RefObject } from 'react'
import { flushSync } from 'react-dom'

import { statusOf } from '@/api/errors'
import { focusLater, useDeleteInvite, type HouseholdInvite } from '@/api/household'
import { Ask } from '@/components/ask'
import { Sheet } from '@/components/sheet'
import { Button } from '@/components/ui/button'
import { useFormat } from '@/i18n/format'
import { useMessages } from '@/i18n/messages'
import { problem } from '@/i18n/problem'
import { sayChange } from '@/lib/toast'

import { JoinPreview } from './join-preview'

// Invites are the household's invites waiting (sent), each revoked after
// it asks, or those waiting for the user's answer (received), each seen
// in a sheet of what joining does and joined there, or declined after it
// asks. The focus goes to Keep, and back to what asked, or to the heading
// once the invite is gone; to fallback when the list went with it, or the
// household was joined.
export function Invites({
  kind,
  invites,
  timeZone,
  fallback,
}: {
  kind: 'sent' | 'received'
  invites: readonly HouseholdInvite[]
  timeZone: string
  fallback: RefObject<HTMLElement | null>
}) {
  const t = useMessages()
  const f = useFormat()
  const heading = useRef<HTMLHeadingElement>(null)
  const openers = useRef(new Map<string, HTMLButtonElement>())
  // seer is the button that opened the sheet of what joining does.
  const seer = useRef<HTMLButtonElement | null>(null)
  const [asking, setAsking] = useState<string | undefined>(undefined)
  const [seeing, setSeeing] = useState<HouseholdInvite | undefined>(undefined)
  const remove = useDeleteInvite()
  const sent = kind === 'sent'
  const h = t.household

  if (invites.length === 0) {
    return null
  }

  // yes revokes or declines inv. It waits for the answer itself, not by
  // the call's callbacks, which go with the list when it was the last.
  async function yes(inv: HouseholdInvite) {
    try {
      await remove.mutateAsync({ params: { path: { id: inv.id } } })
    } catch (error) {
      // Gone elsewhere, it is gone all the same; any other is said below.
      if (statusOf(error) !== 404) {
        return
      }
    }
    if (!sent) {
      sayChange({ title: h.declined(inv.by.name) })
    }
    setAsking(undefined)
    focusLater(heading, fallback)
  }

  return (
    <section aria-labelledby={`household-${kind}`} className="space-y-3">
      <h2 id={`household-${kind}`} ref={heading} tabIndex={-1} className="text-h3 outline-none">
        {sent ? h.sent : h.received}
      </h2>
      <ul className="space-y-3">
        {invites.map((inv) => {
          const who = sent ? inv.invited : inv.by
          return (
            <li key={inv.id} className="space-y-3 rounded-lg border bg-card p-4 shadow-card">
              <div className="flex flex-wrap items-start gap-3">
                <div className="min-w-0 flex-1 space-y-1">
                  <h3 className="font-semibold break-words">
                    {sent ? who.name : h.invitesYou(who.name)}
                  </h3>
                  <p className="text-small break-words text-muted-foreground">{who.email}</p>
                  <p className="text-small text-muted-foreground">
                    {h.waitsUntil(f.day(inv.expires_at, timeZone))}
                  </p>
                </div>
                {asking === inv.id ? null : (
                  <div className="flex flex-wrap gap-2">
                    {!sent && (
                      <Button
                        size="sm"
                        onClick={(e) => {
                          seer.current = e.currentTarget
                          setSeeing(inv)
                        }}
                      >
                        {h.see}
                      </Button>
                    )}
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
                      aria-label={sent ? h.revoke(who.name) : undefined}
                      onClick={() => {
                        remove.reset()
                        setAsking(inv.id)
                      }}
                    >
                      {sent ? h.yesRevoke : h.decline}
                    </Button>
                  </div>
                )}
              </div>
              {asking === inv.id && (
                <Ask
                  question={sent ? h.confirmRevoke(who.name) : h.confirmDecline(who.name)}
                  yes={sent ? h.yesRevoke : h.decline}
                  pending={remove.isPending}
                  onYes={() => {
                    void yes(inv)
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
          )
        })}
      </ul>
      {remove.isError && statusOf(remove.error) !== 404 && (
        <p role="alert" className="text-sm text-destructive">
          {h.failed(problem(t, remove.error))}
        </p>
      )}
      {!sent && (
        <Sheet
          open={seeing !== undefined}
          onOpenChange={(open) => {
            if (!open) {
              setSeeing(undefined)
            }
          }}
          title={h.preview}
          finalFocus={seer}
        >
          {seeing && (
            <JoinPreview
              invite={seeing}
              onDone={() => {
                setSeeing(undefined)
                focusLater(fallback)
              }}
            />
          )}
        </Sheet>
      )}
    </section>
  )
}
