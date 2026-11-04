import { Link } from '@tanstack/react-router'
import { ChevronRight, Gauge, Sparkles } from 'lucide-react'

import type { components } from '@/api/schema'
import { useGists, useMaker } from '@/components/plan/change-words'
import { useDate } from '@/components/plan/proposal-text'
import { useMessages } from '@/i18n/messages'

type Plan = components['schemas']['PlanBody']
type Proposal = components['schemas']['ProposalBody']

// CoachLine is under the chart a proposal that waits cites, in the
// coach's colour, as the day's banner says it: who proposes how many
// changes ("Claude proposes 1 plan change"), what they are, the days it
// was drawn from, which the chart shades, and the way to the plan, where
// it is answered. The autopilot's own has its mark, not an AI's sparkles.
export function CoachLine({ proposal, plan }: { proposal: Proposal; plan: Plan }) {
  const t = useMessages()
  const gists = useGists()
  const maker = useMaker()
  const date = useDate()
  const n = t.plan.nudge
  const what = gists(proposal, plan.targets)
  const count = Math.max(what.length, 1)
  const yours = proposal.by.client === 'web' && proposal.by.model === undefined
  const Icon = proposal.by.client === 'autopilot' ? Gauge : Sparkles

  return (
    <Link
      to="/plan"
      className="flex min-h-11 items-center gap-3 rounded-md bg-coach-soft py-2.5 pr-2 pl-3 outline-none hover:bg-coach-soft/80 focus-visible:ring-2 focus-visible:ring-ring"
    >
      <Icon aria-hidden className="size-4 shrink-0 text-coach" />
      <span className="min-w-0 flex-1 text-small">
        <span className="block font-semibold text-coach">
          {yours ? n.yours(count) : n.proposes(maker(proposal.by), count)}
        </span>{' '}
        <span className="block text-muted-foreground">
          {[
            ...what,
            ...(proposal.window === undefined
              ? []
              : [t.plan.pending.window(date(proposal.window.from), date(proposal.window.to))]),
          ].join(' · ')}
        </span>
      </span>
      <ChevronRight aria-hidden className="size-5 shrink-0 text-coach" />
    </Link>
  )
}
