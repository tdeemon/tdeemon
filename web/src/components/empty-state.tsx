import type { LucideIcon } from 'lucide-react'
import type { ReactNode } from 'react'

import { cn } from '@/lib/utils'

// EmptyState is what a list or a page shows while there is nothing in it
// yet: an icon in a soft tile, a sentence of what will be here (under a
// title, when the place has none of its own), and the one action that
// fills it, when there is one there. bare leaves out the card, for a
// place already in one.
export function EmptyState({
  icon: Icon,
  title,
  children,
  action,
  bare = false,
  className,
}: {
  icon: LucideIcon
  title?: string
  children: ReactNode
  action?: ReactNode
  bare?: boolean
  className?: string
}) {
  return (
    <div
      className={cn(
        'flex flex-wrap items-center gap-x-3 gap-y-3',
        !bare && 'rounded-lg border bg-card p-4 shadow-card',
        className,
      )}
    >
      <span
        aria-hidden
        className="grid size-11 shrink-0 place-items-center rounded-md bg-surface-2 text-muted-foreground"
      >
        <Icon className="size-5" strokeWidth={1.75} />
      </span>
      <div className="min-w-0 flex-1 basis-40 space-y-0.5">
        {title !== undefined && <p className="text-h3">{title}</p>}
        <p className={cn('text-muted-foreground', title !== undefined && 'text-small')}>
          {children}
        </p>
      </div>
      {action}
    </div>
  )
}
