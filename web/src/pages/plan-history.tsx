import { Link, useSearch } from '@tanstack/react-router'
import { ChevronLeft } from 'lucide-react'
import { useId } from 'react'

import { $api } from '@/api/client'
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
// revision open, the latest at first; on a phone one or the other.
export function PlanHistoryPage() {
  const t = useMessages()
  const id = useId()
  const desktop = useDesktop()
  const { rev } = useSearch({ from: '/app/plan/history' })
  const plan = $api.useQuery('get', '/api/v1/plan')
  const r = t.plan.revisions

  let body
  if (plan.data === undefined) {
    body = plan.isError ? (
      <LoadError title={t.loadFailed.plan} error={plan.error} onRetry={() => void plan.refetch()} />
    ) : (
      <Skeleton blocks={['h-14', 'h-14', 'h-14']} />
    )
  } else if (plan.data.revisions.length === 0) {
    body = <p className="text-sm text-muted-foreground">{r.none}</p>
  } else {
    const { revisions } = plan.data
    const open = revisions.find((x) => x.id === rev) ?? (desktop ? revisions[0] : undefined)
    const detail = open && <RevisionDetail key={open.id} rev={open} plan={plan.data} />
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
      body = <RevisionList revisions={revisions} targets={plan.data.targets} />
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
