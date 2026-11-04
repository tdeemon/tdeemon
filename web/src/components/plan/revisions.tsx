import { useQuery, useQueryClient } from '@tanstack/react-query'
import { Link } from '@tanstack/react-router'
import { ChevronRight } from 'lucide-react'
import { useId, useState } from 'react'

import { $api } from '@/api/client'
import { statusOf } from '@/api/errors'
import type { components } from '@/api/schema'
import { sessionQuery } from '@/api/session'
import { Rationale } from '@/components/autopilot/rationale'
import { useGists, useWho } from '@/components/plan/change-words'
import { ProposalDiff, type Proposal } from '@/components/plan/proposal-diff'
import { useLimitWords, useViolationText, violationsOf } from '@/components/plan/proposal-text'
import { Button } from '@/components/ui/button'
import { useFormat } from '@/i18n/format'
import { useMessages } from '@/i18n/messages'
import { sayChange } from '@/lib/toast'
import { cn } from '@/lib/utils'
import { dateAt } from '@/lib/zoned'

type Plan = components['schemas']['PlanBody']

// tightened are the limits a revision tightened, which taking it back
// loosens.
function tightened(rev: Proposal): string[] {
  return rev.changes.limits.filter((ch) => !ch.loosens).map((ch) => ch.name)
}

// takenBack is the code the app proposes taking a revision back with, as
// its rationale: each language says it its own way (rationales).
const takenBack = 'taken back in the app'

// useWhen says when a revision was applied, and by whom: "Oct 20, by
// Claude (claude-opus-5-5)".
function useWhen(): (rev: Proposal) => string {
  const t = useMessages()
  const f = useFormat()
  const session = useQuery(sessionQuery)
  const timeZone = session.data?.user.timezone ?? 'UTC'
  const who = useWho()
  return (rev) =>
    t.plan.revisions.applied(
      rev.decided_at === undefined ? '' : f.day(rev.decided_at, timeZone),
      who(rev.by),
    )
}

// RevisionList is the history of the plan as rows, newest first, each a
// link to its revision: when and by whom, what it changed in a line, and
// whether it was taken back. The one open beside the list (a
// desktop's) is marked.
export function RevisionList({
  revisions,
  targets,
  open,
}: {
  revisions: readonly Proposal[]
  targets: Plan['targets']
  open?: string | undefined
}) {
  const t = useMessages()
  const when = useWhen()
  const gists = useGists()
  const r = t.plan.revisions

  return (
    <ol className="divide-y overflow-hidden rounded-lg border bg-card shadow-card">
      {revisions.map((rev) => {
        const what = gists(rev, targets)
        return (
          <li key={rev.id}>
            <Link
              to="/plan/history"
              search={{ rev: rev.id }}
              aria-current={open === rev.id ? 'page' : undefined}
              className="group flex min-h-14 items-center gap-3 px-4 py-2.5 outline-none hover:bg-muted/60 focus-visible:ring-2 focus-visible:ring-ring focus-visible:ring-inset aria-[current=page]:bg-primary-soft lg:px-5"
            >
              <span className="min-w-0 flex-1">
                <span className="block text-small text-muted-foreground">
                  {when(rev)}
                  {rev.status === 'reverted' && ` · ${r.reverted}`}
                </span>
                <span className="block truncate font-medium group-aria-[current=page]:text-primary">
                  {what.length === 0 ? t.plan.rationales['changed in the app'] : what.join(' · ')}
                </span>
              </span>
              <ChevronRight aria-hidden className="size-4 shrink-0 text-muted-foreground" />
            </Link>
          </li>
        )
      })}
    </ol>
  )
}

// RevisionDetail is a revision of the plan whole: when, who, why and
// what it changed, before → after. One applied may be taken back in a
// step: the user, here, consents to the inverse the server makes, which
// applies at once, and a toast says it. Taking back a limit it tightened
// loosens it: the question says so, and the answer confirms it. The
// revision that takes it back is given to onTakenBack once applied.
export function RevisionDetail({
  rev,
  plan,
  onTakenBack,
}: {
  rev: Proposal
  plan: Plan
  onTakenBack?: ((revert: string) => void) | undefined
}) {
  const t = useMessages()
  const id = useId()
  const queryClient = useQueryClient()
  const violation = useViolationText()
  const limit = useLimitWords()
  const when = useWhen()
  const gists = useGists()
  // Whether the user is asked about taking it back, and why it failed.
  const [asking, setAsking] = useState(false)
  const [failed, setFailed] = useState<{ error: unknown } | undefined>(undefined)
  const revert = $api.useMutation('post', '/api/v1/proposals/{id}/revert')
  const apply = $api.useMutation('post', '/api/v1/proposals/{id}/apply')
  const busy = revert.isPending || apply.isPending
  const session = useQuery(sessionQuery)
  const timeZone = session.data?.user.timezone ?? 'UTC'
  // The user's day starts at this hour, so an hour before it is the day
  // before's.
  const startHour = session.data?.user.day_start_hour ?? 0
  const r = t.plan.revisions

  async function takeBack() {
    setFailed(undefined)
    const what = gists(rev, plan.targets)
    try {
      const made = await revert.mutateAsync({
        params: { path: { id: rev.id } },
        body: { rationale: takenBack },
      })
      // The user confirmed what the question said it loosens, and no more:
      // anything else is asked when the revert, waiting, is accepted.
      const loosens = made.proposal.changes.limits.filter((ch) => ch.loosens)
      const told = tightened(rev)
      const confirm = loosens.length > 0 && loosens.every((ch) => told.includes(ch.name))
      await apply.mutateAsync({
        params: { path: { id: made.proposal.id } },
        ...(confirm ? { body: { confirm: true } } : {}),
      })
      sayChange({ title: r.done, lines: what })
      onTakenBack?.(made.proposal.id)
    } catch (error) {
      setFailed({ error })
    } finally {
      setAsking(false)
      // A revert made but refused when applied waits, as any proposal.
      await queryClient.invalidateQueries()
    }
  }

  // why says why a change could not be taken back: the limits now, or
  // that it took effect or the plan changed since.
  const why = (error: unknown) => {
    const refused = violationsOf(error)
    if (refused.length > 0) {
      return [t.plan.pending.refusedNow, ...refused.map(violation)].join(' ')
    }
    return statusOf(error) === 422 ? r.cannot : t.plan.pending.failed
  }

  return (
    <section
      aria-labelledby={id}
      className="space-y-3 rounded-lg border bg-card p-4 shadow-card lg:p-5"
    >
      <header className="space-y-0.5">
        <h2 id={id} className="text-h3">
          {when(rev)}
        </h2>
        {rev.status === 'reverted' && (
          <p className="text-small text-muted-foreground">{r.reverted}</p>
        )}
      </header>
      <Rationale proposal={rev} />
      {/* What took effect at once did so on the day it was applied. */}
      <ProposalDiff
        changes={rev.changes}
        targets={plan.targets}
        today={
          rev.decided_at === undefined
            ? plan.today
            : dateAt(Date.parse(rev.decided_at) - startHour * 3_600_000, timeZone)
        }
        past
      />
      {failed !== undefined && (
        <p role="alert" className="text-sm text-destructive">
          {why(failed.error)}
        </p>
      )}
      {rev.status === 'applied' &&
        (asking ? (
          <div className="space-y-2">
            <p className="text-sm">{r.confirm}</p>
            {tightened(rev).length > 0 && (
              <p className="text-sm">{r.loosens(tightened(rev).map(limit.label).join(', '))}</p>
            )}
            <div className="flex gap-2">
              <Button disabled={busy} onClick={() => void takeBack()}>
                {busy ? t.plan.pending.sending : r.yes}
              </Button>
              <Button
                variant="outline"
                disabled={busy}
                onClick={() => {
                  setAsking(false)
                }}
              >
                {r.no}
              </Button>
            </div>
          </div>
        ) : (
          <Button
            variant="outline"
            onClick={() => {
              setAsking(true)
            }}
          >
            {r.revert}
          </Button>
        ))}
    </section>
  )
}

// HistoryCard is the plan's latest revisions beside it on a desktop, as
// rows to their page, and a link to them all.
export function HistoryCard({ plan }: { plan: Plan }) {
  const t = useMessages()
  const id = useId()
  const r = t.plan.revisions
  const latest = plan.revisions.slice(0, 3)

  return (
    <section aria-labelledby={id} className="space-y-2">
      <div className="flex items-baseline justify-between gap-2 px-1">
        <h2 id={id} className="text-h3">
          {r.title}
        </h2>
        <Link
          to="/plan/history"
          className={cn(
            "relative text-small font-semibold text-primary after:absolute after:-inset-3 after:content-['']",
          )}
        >
          {plan.revisions_more ? r.all(plan.revisions.length) : r.count(plan.revisions.length)}
        </Link>
      </div>
      <RevisionList revisions={latest} targets={plan.targets} />
    </section>
  )
}
