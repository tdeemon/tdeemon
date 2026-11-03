import { useQuery } from '@tanstack/react-query'
import { useNavigate } from '@tanstack/react-router'
import { LogOut, Share2, X } from 'lucide-react'
import { useId, useRef, useState, type SubmitEvent } from 'react'

import { statusOf } from '@/api/errors'
import { useShareRecipe, useStopSharing } from '@/api/recipe-shares'
import { sessionQuery } from '@/api/session'
import type { Food } from '@/api/types'
import { Sheet } from '@/components/sheet'
import { Button } from '@/components/ui/button'
import type { Messages } from '@/i18n/en'
import { useMessages } from '@/i18n/messages'
import { problem } from '@/i18n/problem'
import { fieldLabel, inputClass } from '@/lib/styles'
import { sayChange } from '@/lib/toast'
import { cn } from '@/lib/utils'

// why says why a share with the address typed was not made: one's own,
// which the server refuses as it does an address of no user, is said as
// it is.
function why(t: Messages, error: unknown, own: boolean): string {
  const status = statusOf(error)
  if (status === 409) {
    return t.sharing.taken
  }
  if (status === 422) {
    return own ? t.sharing.self : t.sharing.noUser
  }
  if (status === 429) {
    return t.sharing.tooMany
  }
  return t.sharing.failed(problem(t, error))
}

// RecipeSharing is a recipe's members, when it is shared (each removed in
// a tap, the user leaving it by their own), and Share, a sheet of the
// other user's address. Each change is said in a toast whose Undo takes
// it back; a recipe left is the user's no more, so the app goes back to
// the foods.
export function RecipeSharing({ food, className }: { food: Food; className?: string }) {
  const t = useMessages()
  const id = useId()
  const navigate = useNavigate()
  const stop = useStopSharing()
  const [open, setOpen] = useState(false)
  const opener = useRef<HTMLButtonElement>(null)
  const members = food.recipe?.shared_with ?? []
  // A member of the user's household no one takes out; the user leaves
  // with their household, and only while someone else stays.
  const withHousehold = members.some((m) => m.household === true)
  const canLeave = members.some((m) => m.you !== true && m.household !== true)
  const undo = (action: 'recipe.share' | 'recipe.unshare') => ({
    last: { action, entity: food.id },
    date: 'today',
  })

  // remove stops the share with the member at seat, the user leaving it
  // by their own. It waits for the answer itself, not by the call's
  // callbacks, which go with the page when the recipe left is asked again
  // first and is gone.
  async function remove(seat: number, name: string, you: boolean) {
    try {
      await stop.mutateAsync({ params: { path: { id: food.id, seat } } })
    } catch {
      // Said by the status below.
      return
    }
    sayChange(
      { title: you ? t.sharing.left(food.name) : t.sharing.removed(name) },
      undo('recipe.unshare'),
    )
    if (you) {
      await navigate({ to: '/foods' })
    }
  }

  return (
    <section aria-labelledby={`${id}-title`} className={cn('space-y-2', className)}>
      <div className="flex items-center justify-between gap-2">
        <h2 id={`${id}-title`} className="text-h3">
          {t.sharing.title}
        </h2>
        <Button
          ref={opener}
          variant="outline"
          size="sm"
          onClick={() => {
            setOpen(true)
          }}
        >
          <Share2 aria-hidden />
          {t.sharing.share}
        </Button>
      </div>
      {members.length === 0 ? (
        <p className="text-small text-muted-foreground">{t.sharing.none}</p>
      ) : (
        <ul aria-labelledby={`${id}-title`} className="divide-y">
          {members.map((m) => (
            <li key={m.seat} className="flex min-h-11 items-center gap-2 py-1.5">
              <span className="min-w-0 flex-1 break-words">
                {m.name}
                {m.you === true && (
                  <span className="text-muted-foreground"> · {t.sharing.you}</span>
                )}
                {m.household === true && (
                  <span className="text-muted-foreground"> · {t.sharing.household}</span>
                )}
              </span>
              {(m.you === true ? canLeave : m.household !== true) && (
                <Button
                  variant="ghost"
                  size={m.you === true ? 'sm' : 'icon'}
                  disabled={stop.isPending}
                  aria-label={m.you === true ? t.sharing.leave : t.sharing.remove(m.name)}
                  onClick={() => {
                    void remove(m.seat, m.name, m.you === true)
                  }}
                >
                  {m.you === true ? (
                    <>
                      <LogOut aria-hidden />
                      {t.sharing.leave}
                    </>
                  ) : (
                    <X aria-hidden />
                  )}
                </Button>
              )}
            </li>
          ))}
        </ul>
      )}
      {members.some((m) => m.you === true) && canLeave && (
        <p className="text-caption text-muted-foreground">
          {withHousehold ? t.sharing.leaveWithHousehold : t.sharing.leaveHint}
        </p>
      )}
      {stop.isError && (
        <p role="alert" className="text-sm text-destructive">
          {t.sharing.failed(problem(t, stop.error))}
        </p>
      )}
      <Sheet open={open} onOpenChange={setOpen} title={t.sharing.sheet} finalFocus={opener}>
        <ShareForm
          food={food}
          onShared={(email) => {
            sayChange({ title: t.sharing.shared(email) }, undo('recipe.share'))
            setOpen(false)
          }}
        />
      </Sheet>
    </section>
  )
}

// ShareForm shares food with the user of the address typed: an error
// says why, at its field.
function ShareForm({ food, onShared }: { food: Food; onShared: (email: string) => void }) {
  const t = useMessages()
  const id = useId()
  const share = useShareRecipe()
  const session = useQuery(sessionQuery)
  const [address, setAddress] = useState('')
  const [sent, setSent] = useState('')

  function submit(event: SubmitEvent<HTMLFormElement>) {
    event.preventDefault()
    const typed = address.trim()
    if (typed !== '') {
      setSent(typed)
      share.mutate(
        { params: { path: { id: food.id } }, body: { email: typed } },
        {
          onSuccess: () => {
            onShared(typed)
          },
        },
      )
    }
  }

  return (
    <form onSubmit={submit} className="space-y-4">
      <label className="block space-y-1">
        <span className={cn(fieldLabel, 'block')}>{t.sharing.email}</span>
        <input
          // Text, not email: a browser refuses an address of letters such as
          // č, which the server takes.
          type="text"
          inputMode="email"
          autoCapitalize="none"
          spellCheck={false}
          value={address}
          required
          autoComplete="off"
          aria-invalid={share.isError || undefined}
          aria-describedby={share.isError ? `${id}-error` : `${id}-hint`}
          className={inputClass}
          onChange={(e) => {
            setAddress(e.currentTarget.value)
          }}
        />
      </label>
      {share.isError ? (
        <p id={`${id}-error`} role="alert" className="text-sm text-destructive">
          {why(t, share.error, sent.toLowerCase() === session.data?.user.email.toLowerCase())}
        </p>
      ) : (
        <p id={`${id}-hint`} className="text-small text-muted-foreground">
          {t.sharing.hint}
        </p>
      )}
      <Button type="submit" className="w-full" disabled={share.isPending}>
        {share.isPending ? t.sharing.sharing : t.sharing.submit}
      </Button>
    </form>
  )
}
