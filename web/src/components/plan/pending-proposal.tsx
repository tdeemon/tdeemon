import { useQueryClient } from '@tanstack/react-query'
import { Gauge, Sparkles } from 'lucide-react'
import { useId, useState } from 'react'

import { $api } from '@/api/client'
import { statusOf } from '@/api/errors'
import type { components } from '@/api/schema'
import { Rationale } from '@/components/autopilot/rationale'
import { useGists, useMaker, useWho } from '@/components/plan/change-words'
import { Loosen } from '@/components/plan/loosen'
import { ProposalChecks } from '@/components/plan/proposal-checks'
import { ProposalData } from '@/components/plan/proposal-data'
import { ProposalDiff, type Proposal } from '@/components/plan/proposal-diff'
import {
  useDate,
  useLimitWords,
  useViolationText,
  violationsOf,
} from '@/components/plan/proposal-text'
import { Button } from '@/components/ui/button'
import { useFormat } from '@/i18n/format'
import { useMessages } from '@/i18n/messages'
import { useDesktop } from '@/lib/media'
import { useShortcuts } from '@/lib/shortcuts'
import { sayChange } from '@/lib/toast'

type Plan = components['schemas']['PlanBody']
type Preview = components['schemas']['PreviewBody']

// PendingProposal is the proposal of a change of the plan that waits for
// the user's answer, in the coach's colour, as a model made it in a chat
// that did not decide it: who made it (the maker's mark, the model) and
// when, how long it waits; why, in three lines a tap from the rest; what
// it changes against the plan, now → new with the difference; what the
// limits found, each rule once; what the data say beside it (but of the
// autopilot's, whose weekly review is the data's, said as its
// rationale); and at its foot the buttons to accept or reject it, on a
// desktop A and X too. An answer is said in a toast, and an accepted one
// changes the plan, so every answer the app holds is read again. One
// that changes the user's limits is accepted only here, and one that
// loosens a limit asks first, as the limits' form does.
export function PendingProposal({
  proposal,
  preview,
  plan,
}: {
  proposal: Proposal
  preview: Preview | undefined
  plan: Plan
}) {
  const t = useMessages()
  const f = useFormat()
  const id = useId()
  const queryClient = useQueryClient()
  const violation = useViolationText()
  const limit = useLimitWords()
  const date = useDate()
  const who = useWho()
  const maker = useMaker()
  const gists = useGists()
  const desktop = useDesktop()
  // When the card came, for how long ago it was made and how long it
  // waits: a proposal waits days, so a minute does not matter.
  const [now] = useState(Date.now)
  const daysLeft = Math.max(
    1,
    Math.ceil((new Date(proposal.expires_at).getTime() - now) / 86_400_000),
  )
  // Whether the user is asked to loosen what the proposal loosens.
  const [asking, setAsking] = useState(false)
  const loosened = proposal.changes.limits.filter((ch) => ch.loosens)
  const p = t.plan.pending
  // What it changes, said in the toast of the answer.
  const what = gists(proposal, plan.targets)
  const accept = $api.useMutation('post', '/api/v1/proposals/{id}/apply', {
    onSuccess: async () => {
      sayChange({ title: p.accepted, lines: what })
      await queryClient.invalidateQueries()
    },
    onError: () => {
      setAsking(false)
    },
  })
  const reject = $api.useMutation('post', '/api/v1/proposals/{id}/reject', {
    onSuccess: async () => {
      sayChange({ title: p.rejected })
      await queryClient.invalidateQueries()
    },
  })
  const path = { params: { path: { id: proposal.id } } }
  // Each action starts by forgetting the other's answer, so the one shown
  // is the latest.
  const error = accept.error ?? reject.error
  const failed = accept.isError || reject.isError
  // 409: it waits no more (applied or rejected elsewhere, superseded,
  // expired); 422: the limits refuse it now, or, without a violation, a
  // day of it went by and it can never apply, though it may be rejected.
  const gone = statusOf(error) === 409
  const refused = violationsOf(error)
  const stale = statusOf(error) === 422 && refused.length === 0
  const busy = accept.isPending || reject.isPending
  const canAccept = !(busy || gone || stale || asking)
  const canReject = !(busy || gone)
  const onAccept = () => {
    reject.reset()
    if (loosened.length > 0) {
      accept.reset()
      setAsking(true)
      return
    }
    accept.mutate(path)
  }
  const onReject = () => {
    accept.reset()
    setAsking(false)
    reject.mutate(path)
  }
  // A desktop's keyboard answers it too: A accepts, X rejects.
  useShortcuts(
    desktop
      ? {
          // A capital too, as Caps Lock types it.
          ...Object.fromEntries(
            ['a', 'A'].map((k) => [
              k,
              () => {
                if (canAccept) {
                  onAccept()
                }
              },
            ]),
          ),
          ...Object.fromEntries(
            ['x', 'X'].map((k) => [
              k,
              () => {
                if (canReject) {
                  onReject()
                }
              },
            ]),
          ),
        }
      : {},
  )
  const yours = proposal.by.client === 'web' && proposal.by.model === undefined
  const Mark = proposal.by.client === 'autopilot' ? Gauge : Sparkles

  return (
    <section
      aria-labelledby={id}
      className="space-y-3 rounded-lg border border-l-3 border-l-coach bg-[linear-gradient(var(--coach-soft),var(--card)_7.5rem)] p-4 shadow-card lg:p-5"
    >
      <header className="space-y-2">
        <div className="flex items-center gap-2">
          {!yours && (
            <span className="inline-flex h-6 shrink-0 items-center gap-1 rounded-sm bg-coach-soft px-2 text-caption font-semibold text-coach">
              <Mark aria-hidden className="size-3.5" />
              {maker(proposal.by)}
            </span>
          )}
          <p className="min-w-0 flex-1 truncate text-caption text-muted-foreground">
            {[yours ? p.byYou : p.by(who(proposal.by)), f.ago(proposal.created_at, now)].join(
              ' · ',
            )}
          </p>
          <span className="shrink-0 rounded-sm bg-warning-soft px-2 py-0.5 text-caption font-semibold text-warning">
            {p.expiresIn(daysLeft)}
          </span>
        </div>
        <h2 id={id} className="text-h2">
          {p.title}
        </h2>
      </header>
      <Rationale proposal={proposal} brief clamp />
      {proposal.window !== undefined && (
        <p className="text-caption text-muted-foreground">
          {p.window(date(proposal.window.from), date(proposal.window.to))}
        </p>
      )}
      <ProposalDiff changes={proposal.changes} targets={plan.targets} today={plan.today} />
      {proposal.changes.limits.length > 0 && (
        <p className="text-small text-muted-foreground">{p.limitsHere}</p>
      )}
      {preview !== undefined && <ProposalChecks checks={preview.checks} />}
      {/* The autopilot's own proposal is the data's already, said above. */}
      {proposal.by.client !== 'autopilot' && (
        <ProposalData proposal={proposal} plan={plan} preview={preview} />
      )}
      {failed && (
        <div role="alert" className="space-y-1 text-sm text-destructive">
          {gone ? (
            <p>{p.gone}</p>
          ) : refused.length > 0 ? (
            <>
              <p>{p.refusedNow}</p>
              <ul>
                {refused.map((v) => (
                  <li key={`${v.rule} ${v.day_type ?? ''}`}>{violation(v)}</li>
                ))}
              </ul>
            </>
          ) : stale ? (
            <p>{p.stale}</p>
          ) : (
            <p>{p.failed}</p>
          )}
        </div>
      )}
      {asking && (
        <Loosen
          names={loosened.map((ch) => limit.label(ch.name)).join(', ')}
          pending={busy}
          onYes={() => {
            accept.mutate({ ...path, body: { confirm: true } })
          }}
          onBack={() => {
            setAsking(false)
          }}
        />
      )}
      <div className="flex gap-2">
        <Button className="flex-1" disabled={!canAccept} onClick={onAccept}>
          {accept.isPending ? p.sending : p.accept}
        </Button>
        <Button variant="outline" className="flex-1" disabled={!canReject} onClick={onReject}>
          {p.reject}
        </Button>
      </div>
    </section>
  )
}
