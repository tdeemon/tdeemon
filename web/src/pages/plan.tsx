import { $api } from '@/api/client'
import { AutopilotCard } from '@/components/autopilot/autopilot-card'
import { LoadError } from '@/components/load-error'
import { NoPlan } from '@/components/plan/no-plan'
import { PendingProposal } from '@/components/plan/pending-proposal'
import { PhaseCard } from '@/components/plan/phase-card'
import { HistoryCard } from '@/components/plan/revisions'
import { TargetsCard } from '@/components/plan/targets-card'
import { YouList } from '@/components/plan/you-list'
import { SettingsAvatar } from '@/components/settings-avatar'
import { Skeleton } from '@/components/skeleton'
import { useMessages } from '@/i18n/messages'
import { useDesktop } from '@/lib/media'

const kbdClass =
  'rounded-[5px] border border-control-border px-1.5 font-mono text-caption text-muted-foreground'

// PlanPage is the user's plan: its phase, the hero, with the way to its
// goal and the phases on a timeline; a change a coach proposed, which
// waits for the user's answer; the targets by type of day against the
// TDEE; the autopilot, its weekly review and the next. Before a plan,
// how one comes. Then what a plan rests on of the user's (the
// autopilot's settings, the profile, the limits, the notes for a coach),
// each a row to its page, and the plan's history, a row to its page on a
// phone. A desktop has two columns: the phase, the targets and the
// latest revisions at the left; the proposal, the autopilot and the
// rows at the right, its header saying the keys that answer a
// proposal. The settings are not the plan's: a phone's header leads to
// them.
export function PlanPage() {
  const t = useMessages()
  const desktop = useDesktop()
  const plan = $api.useQuery('get', '/api/v1/plan')
  const me = $api.useQuery('get', '/api/v1/me')
  // The autopilot's card comes with the plan, not after it, so nothing
  // under it moves when it does; once it failed, the plan says it without
  // waiting for it again.
  const autopilot = $api.useQuery('get', '/api/v1/autopilot')
  const autopilotWaits = autopilot.isLoading && autopilot.errorUpdateCount === 0

  // A plan that came stays while asking for it again fails: the bar over
  // the page says since when.
  let left
  let right
  if (plan.data === undefined || autopilotWaits) {
    left =
      plan.isError && plan.data === undefined ? (
        <LoadError
          title={t.loadFailed.plan}
          error={plan.error}
          onRetry={() => void plan.refetch()}
        />
      ) : (
        <Skeleton blocks={['h-80', 'h-52']} />
      )
  } else {
    const { phases, targets, training_weekdays: days, pending, revisions } = plan.data
    const none = phases.length === 0 && targets.length === 0
    const any = phases.length > 0 || targets.length > 0
    const hero = (
      <>
        {/* A proposal is the first step to a plan, when one waits. */}
        {none && pending === undefined && (
          <NoPlan needsProfile={(me.data?.profile.missing.length ?? 0) > 0} />
        )}
        {phases.length > 0 && <PhaseCard plan={plan.data} />}
      </>
    )
    const proposal = pending !== undefined && (
      // A proposal in place of another starts afresh, without the
      // other's answer.
      <PendingProposal
        key={pending.id}
        proposal={pending}
        preview={plan.data.pending_preview}
        plan={plan.data}
      />
    )
    const targetsCard = (any || days.length > 0) && <TargetsCard plan={plan.data} />
    // The autopilot, once there is a plan to review.
    const autopilotCard = any && <AutopilotCard plan={plan.data} autopilot={autopilot.data} />
    if (desktop) {
      left = (
        <>
          {hero}
          {targetsCard}
          {revisions.length > 0 && <HistoryCard plan={plan.data} />}
        </>
      )
      right = (
        <>
          {proposal}
          {autopilotCard}
        </>
      )
    } else {
      left = (
        <>
          {hero}
          {proposal}
          {targetsCard}
          {autopilotCard}
        </>
      )
    }
  }
  const waits = plan.data?.pending !== undefined

  return (
    <div className="space-y-4">
      <header className="flex items-center justify-between gap-2">
        <h1 className="text-title">{t.plan.title}</h1>
        {/* A desktop's keyboard answers a proposal that waits. */}
        {desktop && waits && (
          <p className="flex items-center gap-1.5 text-small text-muted-foreground">
            {t.plan.keys.waits} · <kbd className={kbdClass}>A</kbd> {t.plan.keys.accept} ·{' '}
            <kbd className={kbdClass}>X</kbd> {t.plan.keys.reject}
          </p>
        )}
        <SettingsAvatar className="-my-1.5" />
      </header>
      <div className="grid grid-cols-1 gap-3 lg:grid-cols-[minmax(0,1fr)_minmax(0,24rem)] lg:items-start lg:gap-5 xl:grid-cols-[minmax(0,1fr)_minmax(0,32.5rem)]">
        <div className="space-y-3 lg:space-y-4">{left}</div>
        <div className="space-y-3 lg:space-y-4">
          {right}
          <YouList history={!desktop} />
        </div>
      </div>
    </div>
  )
}
