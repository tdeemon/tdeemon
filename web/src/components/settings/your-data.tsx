import { useQueryClient } from '@tanstack/react-query'
import { useNavigate } from '@tanstack/react-router'
import { Download, Trash2 } from 'lucide-react'
import { useId, useRef, useState, type SubmitEvent } from 'react'

import { $api } from '@/api/client'
import { statusOf } from '@/api/errors'
import { Sheet } from '@/components/sheet'
import { Button, buttonVariants } from '@/components/ui/button'
import type { Messages } from '@/i18n/en'
import { useMessages } from '@/i18n/messages'
import { problem } from '@/i18n/problem'
import { fieldLabel, inputClass } from '@/lib/styles'
import { cn } from '@/lib/utils'

// why says why the account was not deleted.
function why(t: Messages, error: unknown): string {
  const status = statusOf(error)
  if (status === 409) {
    return t.settings.data.onlyAdmin
  }
  return status === 422 ? t.settings.data.wrongEmail : t.settings.data.failed(problem(t, error))
}

// YourData is what the user can do with all their data: take a copy of
// it, a ZIP the server makes, or delete the account with it. A delete
// cannot be taken back, so it asks in a sheet, which says what goes, and
// waits for the account's address typed before its button works. Deleted,
// nothing of the user stays in the app, and the browser is at the
// sign-in page.
export function YourData({ email }: { email: string }) {
  const t = useMessages()
  const d = t.settings.data
  const id = useId()
  const queryClient = useQueryClient()
  const navigate = useNavigate()
  const opener = useRef<HTMLButtonElement>(null)
  const field = useRef<HTMLInputElement>(null)
  const [open, setOpen] = useState(false)
  const [typed, setTyped] = useState('')
  const remove = $api.useMutation('delete', '/api/v1/account', {
    onSuccess: async () => {
      queryClient.clear()
      await navigate({ to: '/login', replace: true })
    },
  })
  const same = typed.trim().toLowerCase() === email.toLowerCase()

  function submit(event: SubmitEvent<HTMLFormElement>) {
    event.preventDefault()
    if (same) {
      remove.mutate({ body: { email: typed.trim() } })
    }
  }

  return (
    <section aria-labelledby={`${id}-title`} className="space-y-3">
      <h2 id={`${id}-title`} className="text-h3">
        {d.title}
      </h2>
      <div className="space-y-4 rounded-lg border bg-card p-4 lg:p-5">
        <div className="space-y-2">
          <a
            href="/api/v1/account/export"
            download
            aria-describedby={`${id}-export`}
            className={buttonVariants({ variant: 'outline' })}
          >
            <Download aria-hidden />
            {d.export}
          </a>
          <p id={`${id}-export`} className="text-small text-muted-foreground">
            {d.exportHint}
          </p>
        </div>
        <div className="space-y-2 border-t pt-4">
          <Button
            ref={opener}
            variant="destructive"
            aria-describedby={`${id}-delete`}
            onClick={() => {
              setTyped('')
              remove.reset()
              setOpen(true)
            }}
          >
            <Trash2 aria-hidden />
            {d.delete}
          </Button>
          <p id={`${id}-delete`} className="text-small text-muted-foreground">
            {d.deleteHint}
          </p>
        </div>
      </div>
      <Sheet
        open={open}
        onOpenChange={setOpen}
        title={d.delete}
        initialFocus={field}
        finalFocus={opener}
      >
        <form onSubmit={submit} className="space-y-4">
          <p>{d.deleteWhat}</p>
          <p className="text-small text-muted-foreground">{d.deleteSnapshots}</p>
          <div className="space-y-1">
            <label className="block space-y-1">
              <span className={cn(fieldLabel, 'block')}>{d.email}</span>
              <input
                ref={field}
                type="email"
                value={typed}
                autoComplete="off"
                aria-describedby={`${id}-type`}
                className={inputClass}
                onChange={(e) => {
                  setTyped(e.currentTarget.value)
                }}
              />
            </label>
            <p id={`${id}-type`} className="text-caption text-muted-foreground">
              {d.typeEmail(email)}
            </p>
          </div>
          {remove.isError && (
            <p role="alert" className="text-sm text-destructive">
              {why(t, remove.error)}
            </p>
          )}
          <Button
            type="submit"
            variant="destructive"
            className="w-full"
            disabled={!same || remove.isPending}
          >
            {remove.isPending ? d.deleting : d.yesDelete}
          </Button>
        </form>
      </Sheet>
    </section>
  )
}
