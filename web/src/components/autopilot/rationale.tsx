import { useLayoutEffect, useRef, useState } from 'react'

import { useReviewWords } from '@/components/autopilot/review-words'
import { useRationale } from '@/components/plan/change-words'
import type { Proposal } from '@/components/plan/proposal-diff'
import { useFormat } from '@/i18n/format'
import { useMessages } from '@/i18n/messages'
import { linkTarget } from '@/lib/styles'
import { cn } from '@/lib/utils'

// Rationale is why a change of the plan was made: a coach's words, the
// app's code in the language, or, for the autopilot's (the code
// `autopilot`), its weekly review said from its numbers: what it
// decided, then why; brief, the first reason alone, as the plan's history
// says it above the changes. One it proposed past its drift limit, while
// it waits, says so, and what an answer does. Clamped, a coach's words
// show three lines, and the rest a tap away (More).
export function Rationale({
  proposal,
  brief = false,
  clamp = false,
}: {
  proposal: Proposal
  brief?: boolean
  clamp?: boolean
}) {
  const t = useMessages()
  const f = useFormat()
  const rationale = useRationale()
  const say = useReviewWords()
  const c = proposal.check_in
  if (proposal.rationale !== 'autopilot' || c === undefined) {
    return clamp ? (
      <Clamped text={rationale(proposal.rationale)} />
    ) : (
      <p className="whitespace-pre-line">{rationale(proposal.rationale)}</p>
    )
  }
  const words = say(c)
  const a = t.autopilot

  return (
    <div className="space-y-1">
      <p className="font-medium">{words.headline}</p>
      {(brief ? words.why.slice(0, 1) : words.why).map((w) => (
        <p key={w} className="text-sm text-muted-foreground">
          {w}
        </p>
      ))}
      {c.past_drift && proposal.status === 'pending' && (
        <div className="mt-2 space-y-0.5 rounded-md border border-warning/40 bg-warning-soft p-3 text-sm">
          <p className="font-semibold text-warning">{a.pastDriftTitle}</p>
          <p>
            {c.drift_limit_kcal === undefined
              ? a.pastDriftNoLimit(f.signed(c.drift_kcal))
              : a.pastDrift(f.signed(c.drift_kcal), f.number(c.drift_limit_kcal))}
          </p>
        </div>
      )}
    </div>
  )
}

// Clamped is a text cut to three lines, with a button that shows the
// rest (More) when it is longer, and cuts it again (Less).
function Clamped({ text }: { text: string }) {
  const t = useMessages()
  const shown = useRef<HTMLParagraphElement>(null)
  const [open, setOpen] = useState(false)
  const [longer, setLonger] = useState(false)
  useLayoutEffect(() => {
    const p = shown.current
    if (p !== null && !open && text !== '') {
      setLonger(p.scrollHeight > p.clientHeight + 1)
    }
  }, [text, open])

  return (
    <div className="space-y-1">
      <p
        ref={shown}
        className={cn(
          'text-small whitespace-pre-line text-muted-foreground',
          !open && 'line-clamp-3',
        )}
      >
        {text}
      </p>
      {(longer || open) && (
        <button
          type="button"
          aria-expanded={open}
          onClick={() => {
            setOpen((was) => !was)
          }}
          className={cn(linkTarget, 'min-w-11 text-left text-small font-semibold text-primary')}
        >
          {open ? t.plan.pending.less : t.plan.pending.more}
        </button>
      )}
    </div>
  )
}
