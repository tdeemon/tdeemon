import { Link } from '@tanstack/react-router'
import { ChevronRight, Gauge, Sparkles } from 'lucide-react'
import { useState } from 'react'

import { $api } from '@/api/client'
import { useGists, useMaker } from '@/components/plan/change-words'
import { useMessages } from '@/i18n/messages'

// ProposalNudge is the day's banner that a proposal waits for the user's
// answer, in the coach's colour: who proposes how many changes ("Claude
// proposes 2 plan changes"), and under it what they are and how long it
// waits; a link to the plan, where it is answered; nothing while none
// waits. The autopilot's own has its mark, not an AI's sparkles.
export function ProposalNudge() {
  const t = useMessages()
  const gists = useGists()
  const maker = useMaker()
  const plan = $api.useQuery('get', '/api/v1/plan')
  // When the banner came: a proposal waits days, so a minute does not
  // matter.
  const [now] = useState(Date.now)
  const pending = plan.data?.pending
  if (plan.data === undefined || pending === undefined) {
    return null
  }
  const n = t.plan.nudge
  const what = gists(pending, plan.data.targets)
  const count = Math.max(what.length, 1)
  const yours = pending.by.client === 'web' && pending.by.model === undefined
  const Icon = pending.by.client === 'autopilot' ? Gauge : Sparkles
  const daysLeft = Math.max(1, Math.ceil((Date.parse(pending.expires_at) - now) / 86_400_000))

  return (
    <Link
      to="/plan"
      className="flex min-h-11 items-center gap-3 rounded-lg border border-l-3 border-l-coach bg-coach-soft py-3 pr-2 pl-3.5 outline-none hover:bg-coach-soft/80 focus-visible:ring-2 focus-visible:ring-ring focus-visible:ring-offset-2 focus-visible:ring-offset-background"
    >
      <Icon aria-hidden className="size-5 shrink-0 text-coach" />
      <span className="min-w-0 flex-1">
        <span className="block text-h3">
          {yours ? n.yours(count) : n.proposes(maker(pending.by), count)}
        </span>
        {/* What it changes and how long it waits. */}{' '}
        <span className="line-clamp-2 block text-small text-muted-foreground">
          {[...what, t.plan.pending.expiresIn(daysLeft)].join(' · ')}
        </span>
      </span>
      <ChevronRight aria-hidden className="size-5 shrink-0 text-coach" />
    </Link>
  )
}
