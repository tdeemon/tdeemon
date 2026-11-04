import { Link, useSearch } from '@tanstack/react-router'
import { ChevronLeft, History } from 'lucide-react'
import { useId, useState } from 'react'

import { $api } from '@/api/client'
import { EmptyState } from '@/components/empty-state'
import { LoadError } from '@/components/load-error'
import { RevisionDetail, RevisionList } from '@/components/plan/revisions'
import { Skeleton } from '@/components/skeleton'
import { SubPage } from '@/components/sub-page'
import { buttonVariants } from '@/components/ui/button'
import { useMessages } from '@/i18n/messages'
import { useDesktop } from '@/lib/media'
import { cn } from '@/lib/utils'

// PlanHistoryPage is the history of the plan, a page of its own: its
// revisions as rows, newest first, and the one open (rev) whole, with
// the way to take it back. On a desktop the rows stay beside the
// revision open, the latest at first; on a phone one or the other. A
// revision taken back that the plan's latest no longer hold (the oldest
// of them, before the revision that took it back) gives its place to
// that one.
export function PlanHistoryPage() {
  const t = useMessages()
  const id = useId()
  const desktop = useDesktop()
  const { rev } = useSearch({ from: '/app/plan/history' })
  const plan = $api.useQuery('get', '/api/v1/plan')
  const r = t.plan.revisions
  // The revision taken back here last, and the one that took it back.
  const [takenBack, setTakenBack] = useState<{ from: string; by: string } | undefined>(undefined)

  let body
  if (plan.data === undefined) {
    body = plan.isError ? (
      <LoadError title={t.loadFailed.plan} error={plan.error} onRetry={() => void plan.refetch()} />
    ) : (
      <Skeleton blocks={['h-14', 'h-14', 'h-14']} />
    )
  } else if (plan.data.revisions.length === 0) {
    body = <EmptyState icon={History}>{r.none}</EmptyState>
  } else {
    const { revisions } = plan.data
    // A revision asked for that the plan's latest do not hold is said
    // gone, not shown as another.
    const shown =
      rev !== undefined && takenBack?.from === rev && !revisions.some((x) => x.id === rev)
        ? takenBack.by
        : rev
    const asked = shown === undefined ? undefined : revisions.find((x) => x.id === shown)
    const missing = shown !== undefined && asked === undefined
    const open = asked ?? (desktop && rev === undefined ? revisions[0] : undefined)
    const detail = missing ? (
      <p className="text-sm text-muted-foreground">{r.missing}</p>
    ) : (
      open && (
        <RevisionDetail
          key={open.id}
          rev={open}
          plan={plan.data}
          onTakenBack={(by) => {
            setTakenBack({ from: open.id, by })
          }}
        />
      )
    )
    if (desktop) {
      body = (
        <div className="grid grid-cols-[minmax(0,22rem)_minmax(0,1fr)] items-start gap-6">
          <div className="sticky top-8">
            <RevisionList revisions={revisions} targets={plan.data.targets} open={open?.id} />
          </div>
          {detail}
        </div>
      )
    } else if (open === undefined) {
      body = (
        <>
          {detail}
          <RevisionList revisions={revisions} targets={plan.data.targets} />
        </>
      )
    } else {
      body = (
        <>
          <Link
            to="/plan/history"
            className={cn(buttonVariants({ variant: 'ghost', size: 'sm' }), '-ml-3')}
          >
            <ChevronLeft aria-hidden />
            {r.allShort}
          </Link>
          {detail}
        </>
      )
    }
  }

  return (
    <SubPage title={r.title} titleId={id} from="plan" wide={desktop}>
      {body}
    </SubPage>
  )
}
