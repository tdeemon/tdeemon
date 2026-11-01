import { useEffect, useId, useRef } from 'react'

import { Button } from '@/components/ui/button'
import { useMessages } from '@/i18n/messages'

// Ask is the question before a change that cannot be taken back (a
// revoke, a delete): the focus goes to Keep, the safe answer, and the
// question describes both. yes is the destructive answer's words.
export function Ask({
  question,
  yes,
  pending,
  onYes,
  onKeep,
}: {
  question: string
  yes: string
  pending: boolean
  onYes: () => void
  onKeep: () => void
}) {
  const t = useMessages()
  const id = useId()
  const keep = useRef<HTMLButtonElement>(null)
  useEffect(() => {
    keep.current?.focus()
  }, [])

  return (
    <div className="flex flex-wrap items-center gap-2 text-sm">
      <span id={id} className="w-full">
        {question}
      </span>
      <Button
        variant="destructive"
        size="sm"
        aria-describedby={id}
        disabled={pending}
        onClick={onYes}
      >
        {yes}
      </Button>
      <Button ref={keep} variant="ghost" size="sm" aria-describedby={id} onClick={onKeep}>
        {t.settings.keep}
      </Button>
    </div>
  )
}
